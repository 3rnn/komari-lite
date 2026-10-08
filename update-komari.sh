#!/usr/bin/env bash
# Safely update an existing Komari Lite systemd installation from a pinned GitHub Release.
# This script deliberately preserves /opt/komari/data, the systemd unit, reverse proxy,
# firewall, administrator accounts, nodes, and Agent data.
set -Eeuo pipefail
IFS=$'\n\t'
umask 077

readonly REPOSITORY='3rnn/komari-lite'
readonly DEFAULT_ROOT='/opt/komari'
readonly DEFAULT_SERVICE='komari.service'
readonly API_BASE="https://api.github.com/repos/${REPOSITORY}/releases"
readonly DOWNLOAD_BASE="https://github.com/${REPOSITORY}/releases/download"

root="$DEFAULT_ROOT"
service="$DEFAULT_SERVICE"
requested_tag='latest'
dry_run=0
stage=''
backup_dir=''
previous_version='unknown'
target_version='unknown'
previous_schema=''
target_schema=''
migration_risk=0
replacement_started=0

usage() {
  cat <<'EOF'
Usage: bash update-komari.sh [--dry-run] [vX.Y.Z]

Update an existing Komari Lite installation from an official stable GitHub Release.

Options:
  --dry-run  Inspect the installation, requested release, architecture, assets, and
             planned safety checks without stopping the service or changing files.
  --help     Show this help text.

Without a version, the script resolves GitHub's latest stable release once and pins the
whole operation to that concrete tag. Use an explicit vX.Y.Z tag for reviewed upgrades.

The script never deletes /opt/komari, resets databases, changes systemd arguments,
modifies the firewall/proxy, or installs/updates Agents. It refuses database downgrades.
EOF
}

log() { printf '[komari-update] %s\n' "$*"; }
die() { printf '[komari-update] ERROR: %s\n' "$*" >&2; exit 1; }
require_command() { command -v "$1" >/dev/null 2>&1 || die "Required command is missing: $1"; }

cleanup() {
  local status=$?
  [[ -n "$stage" && -d "$stage" ]] && rm -rf -- "$stage"
  if (( status != 0 && replacement_started == 1 )); then
    log "Update did not complete. See the messages above before making further changes."
  fi
}
trap cleanup EXIT

json_field() {
  local file=$1 expression=$2
  python3 - "$file" "$expression" <<'PY'
import json, sys
obj = json.load(open(sys.argv[1], encoding='utf-8'))
value = obj
for key in sys.argv[2].split('.'):
    value = value[key]
if not isinstance(value, (str, int, float)):
    raise SystemExit(1)
print(value)
PY
}

version_from_binary() {
  local binary=$1 output
  output="$("$binary" version --json 2>/dev/null)" || return 1
  python3 -c '
import json, re, sys
value = json.load(sys.stdin).get("version")
if not isinstance(value, str) or not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:[.-][A-Za-z0-9.-]+)?", value):
    raise SystemExit(1)
print(value)
' <<<"$output"
}

schema_from_binary() {
  local binary=$1 output
  output="$("$binary" schema-version --json 2>/dev/null)" || return 1
  python3 -c '
import json, sys
value = json.load(sys.stdin).get("schema_version")
if not isinstance(value, int) or isinstance(value, bool) or value < 0:
    raise SystemExit(1)
print(value)
' <<<"$output"
}

schema_from_database() {
  local database=$1
  python3 - "$database" <<'PY'
import sqlite3, sys
path = sys.argv[1]
with sqlite3.connect('file:' + path + '?mode=ro', uri=True) as db:
    result = db.execute('PRAGMA quick_check').fetchone()
    if not result or result[0] != 'ok':
        raise SystemExit('SQLite quick_check failed')
    print(db.execute('PRAGMA user_version').fetchone()[0])
PY
}

sqlite_backup() {
  local source=$1 destination=$2
  python3 - "$source" "$destination" <<'PY'
import sqlite3, sys
source, destination = sys.argv[1:]
source_db = sqlite3.connect('file:' + source + '?mode=ro', uri=True)
target_db = sqlite3.connect(destination)
try:
    source_db.backup(target_db)
    result = target_db.execute('PRAGMA quick_check').fetchone()
    if not result or result[0] != 'ok':
        raise RuntimeError('backup quick_check failed')
finally:
    target_db.close()
    source_db.close()
PY
}

release_metadata() {
  local url=$1 destination=$2
  curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
    --connect-timeout 15 --max-time 60 --retry 2 --output "$destination" "$url"
}

resolve_release() {
  local metadata="$stage/release.json" url
  if [[ "$requested_tag" == 'latest' ]]; then
    url="$API_BASE/latest"
  else
    url="$API_BASE/tags/$requested_tag"
  fi
  release_metadata "$url" "$metadata"
  python3 - "$metadata" "$requested_tag" <<'PY'
import json, re, sys
release = json.load(open(sys.argv[1], encoding='utf-8'))
tag = release.get('tag_name')
if (not isinstance(tag, str) or not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+(?:[.-][A-Za-z0-9.-]+)?', tag)
        or release.get('draft') or release.get('prerelease')
        or (sys.argv[2] != 'latest' and tag != sys.argv[2])):
    raise SystemExit('Release is missing, unstable, draft/prerelease, or tag-mismatched')
assets = release.get('assets')
if not isinstance(assets, list) or len(assets) > 128:
    raise SystemExit('Release has an invalid asset inventory')
names = [item.get('name') for item in assets if isinstance(item, dict)]
if len(names) != len(assets) or len(names) != len(set(names)):
    raise SystemExit('Release has invalid or duplicate asset names')
print(tag)
PY
}

release_has_asset() {
  local name=$1
  python3 - "$stage/release.json" "$name" <<'PY'
import json, sys
items = json.load(open(sys.argv[1], encoding='utf-8')).get('assets', [])
for item in items:
    if item.get('name') == sys.argv[2]:
        raise SystemExit(0)
raise SystemExit(1)
PY
}

download_asset() {
  local tag=$1 name=$2 destination=$3
  curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
    --connect-timeout 15 --max-time 300 --retry 2 --output "$destination" \
    "$DOWNLOAD_BASE/$tag/$name"
  [[ -s "$destination" ]] || die "Downloaded asset is empty: $name"
}

checksum_for() {
  local checksums=$1 name=$2
  awk -v expected_name="$name" '$2 == expected_name && $1 ~ /^[0-9a-f]{64}$/ { print $1 }' "$checksums" | sed -n '1p'
}

verify_health() {
  local binary=$1 expected_version=$2
  local output
  for _ in $(seq 1 20); do
    if systemctl is-active --quiet "$service"; then
      output="$(cd "$root" && "$binary" health --json 2>/dev/null)" || true
      if python3 -c '
import json, sys
info = json.load(sys.stdin)
if info.get("ok") is True and info.get("version") == sys.argv[1] and info.get("schema_version") == info.get("expected_schema_version"):
    raise SystemExit(0)
raise SystemExit(1)
' "$expected_version" <<<"$output"
      then
        return 0
      fi
    fi
    sleep 1
  done
  return 1
}

rollback_after_failure() {
  local current_schema
  current_schema="$(schema_from_database "$root/data/komari.db" 2>/dev/null || printf unknown)"
  if (( migration_risk == 1 )) && [[ "$current_schema" != "$previous_schema" ]]; then
    log "Migration risk detected: database schema changed from $previous_schema to $current_schema."
    log "The previous executable is restored at $root/komari, but it is NOT started because it may be incompatible with the migrated database."
    log "Use the SQLite backups in $backup_dir only with an operator-reviewed recovery plan."
    install -m 0755 "$backup_dir/komari.previous" "$root/komari.new"
    mv -f "$root/komari.new" "$root/komari"
    systemctl stop "$service" || true
    return 1
  fi

  log "Restoring the previous executable because post-update health verification failed."
  install -m 0755 "$backup_dir/komari.previous" "$root/komari.new"
  mv -f "$root/komari.new" "$root/komari"
  systemctl start "$service" || true
  if verify_health "$root/komari" "$previous_version"; then
    log "Rollback completed and the previous version is healthy."
  else
    log "Rollback executable was restored, but automatic health recovery failed. Investigate $service and $backup_dir."
  fi
  return 1
}

while (( $# > 0 )); do
  case "$1" in
    --help|-h) usage; exit 0 ;;
    --dry-run) dry_run=1 ;;
    v[0-9]*.[0-9]*.[0-9]*)
      [[ "$requested_tag" == 'latest' ]] || die 'Only one release version may be specified.'
      requested_tag=$1 ;;
    *) die "Unknown argument: $1 (use --help)" ;;
  esac
  shift
done

(( EUID == 0 )) || die 'Run this script as root.'
for command in bash curl python3 sha256sum systemctl install mv cp awk sed date; do require_command "$command"; done
[[ -d "$root" && ! -L "$root" ]] || die "Installation directory is missing or unsafe: $root"
[[ -f "$root/komari" && ! -L "$root/komari" ]] || die "Existing executable is missing or unsafe: $root/komari"
[[ -f "$root/data/komari.db" ]] || die "Existing SQLite database is missing: $root/data/komari.db"
systemctl show "$service" -p LoadState --value | grep -qx 'loaded' || die "Systemd service is not installed: $service"
systemctl show "$service" -p ExecStart --value | grep -Fq "path=$root/komari" || die "Service does not execute $root/komari; refusing to alter another installation."
systemctl show "$service" -p WorkingDirectory --value | grep -Fxq "$root" || die "Service WorkingDirectory is not $root; refusing unsafe update."

previous_version="$(version_from_binary "$root/komari")" || die "Existing executable does not support \`version --json\`."
previous_schema="$(schema_from_database "$root/data/komari.db")" || die 'Existing SQLite database failed integrity/schema inspection.'

stage="$(mktemp -d /root/.cache/komari-update.XXXXXX)"
resolved_tag="$(resolve_release)" || die 'Unable to resolve a valid official stable release.'
target_version="${resolved_tag#v}"
case "$(uname -m)" in
  x86_64|amd64) architecture='amd64' ;;
  aarch64|arm64) architecture='arm64' ;;
  *) die "Unsupported CPU architecture: $(uname -m)" ;;
esac
asset="komari-linux-$architecture"
release_has_asset "$asset" || die "Release $resolved_tag does not provide $asset."
release_has_asset 'SHA256SUMS.txt' || die "Release $resolved_tag does not provide SHA256SUMS.txt."

log "Installation: $root ($service)"
log "Current version: $previous_version; database schema: $previous_schema"
log "Target release: $resolved_tag; selected asset: $asset"

if [[ "$previous_version" == "$target_version" ]]; then
  log "Requested version is already installed; no service interruption is required."
  exit 0
fi

download_asset "$resolved_tag" 'SHA256SUMS.txt' "$stage/SHA256SUMS.txt"
expected_sha="$(checksum_for "$stage/SHA256SUMS.txt" "$asset")"
[[ "$expected_sha" =~ ^[0-9a-f]{64}$ ]] || die "Published SHA256SUMS.txt lacks a valid checksum for $asset."
download_asset "$resolved_tag" "$asset" "$stage/$asset"
printf '%s  %s\n' "$expected_sha" "$stage/$asset" | sha256sum -c - >/dev/null || die "Checksum verification failed for $asset."
chmod 0755 "$stage/$asset"

target_binary_version="$(version_from_binary "$stage/$asset")" || die "Downloaded release binary does not support \`version --json\` on this architecture."
[[ "$target_binary_version" == "$target_version" ]] || die "Release tag $resolved_tag does not match binary version $target_binary_version."
target_schema="$(schema_from_binary "$stage/$asset")" || die "Downloaded release binary does not support \`schema-version --json\`."
if (( target_schema < previous_schema )); then
  die "Refusing database downgrade: current schema is $previous_schema, target expects $target_schema."
fi
if (( target_schema > previous_schema )); then migration_risk=1; fi

log "Target binary version: $target_binary_version; expected schema: $target_schema"
if (( migration_risk == 1 )); then
  log 'Migration risk: this release may migrate the SQLite schema during startup. A consistent SQLite backup will be created while the service is stopped.'
fi

if (( dry_run == 1 )); then
  log 'Dry run complete. No service or installation files were changed.'
  exit 0
fi

backup_parent="$root/backups"
install -d -m 0700 "$backup_parent"
backup_dir="$(mktemp -d "$backup_parent/update-${resolved_tag}-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")"
cp -a -- "$root/komari" "$backup_dir/komari.previous"
printf '%s\n' "$previous_version" >"$backup_dir/previous-version.txt"
printf '%s\n' "$resolved_tag" >"$backup_dir/target-release.txt"
printf '%s\n' "$expected_sha" >"$backup_dir/$asset.sha256"

log 'Download and checksum verification succeeded. Stopping the service.'
systemctl stop "$service"
if (( migration_risk == 1 )); then
  install -d -m 0700 "$backup_dir/sqlite"
  sqlite_backup "$root/data/komari.db" "$backup_dir/sqlite/komari.db"
  if [[ -f "$root/data/metrics.db" ]]; then sqlite_backup "$root/data/metrics.db" "$backup_dir/sqlite/metrics.db"; fi
  log "Consistent SQLite backup created: $backup_dir/sqlite"
fi

install -m 0755 "$stage/$asset" "$root/komari.new"
mv -f "$root/komari.new" "$root/komari"
replacement_started=1

log "Starting $service with its existing systemd configuration."
if ! systemctl start "$service" || ! verify_health "$root/komari" "$target_version"; then
  rollback_after_failure
  die "Update failed. Previous executable backup: $backup_dir"
fi

log "Update successful. Previous version: $previous_version; target version: $target_version"
log "Backup location: $backup_dir"

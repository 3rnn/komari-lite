#!/usr/bin/env bash
# Safely update an existing Komari Lite installation from an official GitHub Release.
# This script preserves persistent data, the existing systemd unit, reverse proxy,
# firewall, administrator accounts, nodes, and Agent data.
set -Eeuo pipefail
IFS=$'\n\t'
umask 077

readonly REPOSITORY='3rnn/komari-lite'
readonly DEFAULT_ROOT='/opt/komari'
readonly DEFAULT_SERVICE='komari.service'
readonly DEFAULT_STATE_DIR='/var/lib/komari-upgrade'
readonly DEFAULT_UNIT_DIR='/etc/systemd/system'
readonly DEFAULT_SAFE_UPGRADE_SHA256='6db6232b7f01cb641c8a0bda842c7467d7374b59ccdaa163d45456b6acdebb92,1fe3c0525b3358fff7367ef5ef0b1174bcfa3e23f6f03a9d8fb9362301659cd1'
readonly API_BASE="https://api.github.com/repos/${REPOSITORY}/releases"
readonly DOWNLOAD_BASE="https://github.com/${REPOSITORY}/releases/download"

root="${KOMARI_ROOT:-$DEFAULT_ROOT}"
service="${KOMARI_SERVICE:-$DEFAULT_SERVICE}"
state_dir="${KOMARI_UPGRADE_STATE_DIR:-$DEFAULT_STATE_DIR}"
unit_dir="${KOMARI_UPGRADE_UNIT_DIR:-$DEFAULT_UNIT_DIR}"
requested_tag='latest'
dry_run=0
recover_flat=0
stage=''
layout=''
current_binary=''
current_release=''
backup_dir=''
previous_version='unknown'
target_version='unknown'
previous_schema=''
target_schema=''
expected_sha=''
controller=''
controller_sha=''
asset=''

usage() {
  cat <<'EOF'
Usage: bash update-komari.sh [--dry-run] [vX.Y.Z]

Update an existing Komari Lite installation from an official stable GitHub Release.

Options:
  --dry-run  Inspect the installation, requested release, download verification,
             database safety, and planned transaction without changing persistent files.
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
  if (( status != 0 )); then
    log 'No automatic filesystem recovery was attempted by this wrapper. For a versioned installation, inspect the existing safe-upgrade journal before retrying.'
  fi
}
trap cleanup EXIT

sha256_file() { sha256sum -- "$1" | awk '{print $1}'; }

safe_mode() {
  local path=$1 metadata uid mode kind
  metadata="$(stat -Lc '%u %a %F' -- "$path")" || return 1
  IFS=' ' read -r uid mode kind <<<"$metadata"
  [[ "$uid" == '0' && "$kind" == 'directory' ]] || return 1
  (( (8#$mode & 8#022) == 0 ))
}

safe_directory() {
  local path=$1
  [[ -d "$path" && ! -L "$path" ]] || return 1
  safe_mode "$path"
}

safe_regular_file() {
  local path=$1 metadata uid mode kind
  [[ -f "$path" && ! -L "$path" ]] || return 1
  metadata="$(stat -Lc '%u %a %F' -- "$path")" || return 1
  IFS=' ' read -r uid mode kind <<<"$metadata"
  [[ "$uid" == '0' && "$kind" == 'regular file' ]] || return 1
  (( (8#$mode & 8#022) == 0 ))
}

safe_invoked_updater() {
  local path=$1 metadata uid mode kind
  [[ -f "$path" && ! -L "$path" ]] || return 1
  metadata="$(stat -Lc '%u %a %F' -- "$path")" || return 1
  IFS=' ' read -r uid mode kind <<<"$metadata"
  [[ "$kind" == 'regular file' && ( "$uid" == '0' || "${SUDO_UID:-}" == "$uid" ) ]] || return 1
  (( (8#$mode & 8#022) == 0 ))
}

require_within_root() {
  local path=$1 resolved
  resolved="$(realpath -e -- "$path")" || die "Missing path: $path"
  [[ "$resolved" == "$root" || "$resolved" == "$root"/* ]] || die "Unsafe path escapes installation root: $path -> $resolved"
  printf '%s\n' "$resolved"
}

require_protected_tree_path() {
  local path=$1
  while [[ "$path" != "$root" ]]; do
    safe_directory "$path" || die "Unsafe directory ownership or permissions: $path"
    path="$(dirname -- "$path")"
  done
  safe_directory "$root" || die "Unsafe installation root ownership or permissions: $root"
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
  if [[ "$requested_tag" == 'latest' ]]; then url="$API_BASE/latest"; else url="$API_BASE/tags/$requested_tag"; fi
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
if not isinstance(assets, list) or not assets or len(assets) > 128:
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
for item in json.load(open(sys.argv[1], encoding='utf-8')).get('assets', []):
    if item.get('name') == sys.argv[2]: raise SystemExit(0)
raise SystemExit(1)
PY
}

download_asset() {
  local tag=$1 name=$2 destination=$3
  curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
    --connect-timeout 15 --max-time 300 --retry 2 --output "$destination" "$DOWNLOAD_BASE/$tag/$name"
  [[ -s "$destination" ]] || die "Downloaded asset is empty: $name"
}

checksum_for() {
  local checksums=$1 name=$2
  awk -v expected_name="$name" '$2 == expected_name && $1 ~ /^[0-9a-f]{64}$/ { print $1 }' "$checksums" | sed -n '1p'
}

verify_download() {
  local name=$1 sha
  sha="$(checksum_for "$stage/SHA256SUMS.txt" "$name")"
  [[ "$sha" =~ ^[0-9a-f]{64}$ ]] || die "Published SHA256SUMS.txt lacks a valid checksum for $name."
  download_asset "$resolved_tag" "$name" "$stage/$name"
  printf '%s  %s\n' "$sha" "$stage/$name" | sha256sum -c - >/dev/null || die "Checksum verification failed for $name."
  printf '%s\n' "$sha"
}

verify_flat_health() {
  local expected_version=$1 output
  for _ in $(seq 1 20); do
    if systemctl is-active --quiet "$service"; then
      output="$(cd "$root" && "$root/komari" health --json 2>/dev/null)" || true
      if python3 -c '
import json, sys
info = json.load(sys.stdin)
raise SystemExit(0 if info.get("ok") is True and info.get("version") == sys.argv[1] and info.get("schema_version") == info.get("expected_schema_version") else 1)
' "$expected_version" <<<"$output"; then return 0; fi
    fi
    sleep 1
  done
  return 1
}

allowed_controller_sha() {
  local sha=$1 value=${KOMARI_SAFE_UPGRADE_SHA256_ALLOWLIST:-$DEFAULT_SAFE_UPGRADE_SHA256}
  case ",$value," in *,"$sha",*) return 0 ;; *) return 1 ;; esac
}

detect_layout() {
  safe_directory "$root" || die "Installation directory is missing or unsafe: $root"
  [[ -f "$root/data/komari.db" && ! -L "$root/data/komari.db" ]] || die "Existing SQLite database is missing or unsafe: $root/data/komari.db"
  safe_directory "$root/data" || die "Unsafe persistent data directory: $root/data"

  if [[ -f "$root/komari" && ! -L "$root/komari" ]]; then
    layout='flat'; current_binary="$root/komari"
    safe_regular_file "$current_binary" || die "Existing executable has unsafe ownership or permissions: $current_binary"
    return
  fi

  [[ -L "$root/komari" ]] || die "Existing executable is missing or has an unsupported type: $root/komari"
  [[ "$(readlink -- "$root/komari")" == 'current/komari' ]] || die "Unsafe versioned executable symlink: $root/komari"
  [[ -L "$root/current" ]] || die "Versioned installation is missing current symlink: $root/current"
  current_release="$(readlink -- "$root/current")"
  [[ "$current_release" =~ ^releases/[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]] || die "Unsafe versioned current symlink target: $current_release"
  current_binary="$(require_within_root "$root/komari")"
  [[ "$current_binary" == "$root/$current_release/komari" ]] || die "Unexpected versioned executable resolution: $current_binary"
  safe_regular_file "$current_binary" || die "Existing versioned executable has unsafe ownership or permissions: $current_binary"
  require_protected_tree_path "$(dirname -- "$current_binary")"
  [[ -d "$root/releases" && ! -L "$root/releases" ]] || die "Versioned release storage is missing or unsafe: $root/releases"
  safe_directory "$root/releases" || die "Unsafe versioned release storage: $root/releases"
  layout='versioned'
}

verify_service_target() {
  systemctl show "$service" -p LoadState --value | grep -qx 'loaded' || die "Systemd service is not installed: $service"
  local unit
  unit="$(systemctl show "$service" -p ExecStart -p WorkingDirectory 2>/dev/null || true)"
  printf '%s\n' "$unit" | grep -Fq "path=$root/komari" || die "Service does not execute $root/komari; refusing to alter another installation."
  printf '%s\n' "$unit" | grep -Fxq "WorkingDirectory=$root" || die "Service WorkingDirectory is not $root; refusing unsafe update."
}

prepare_versioned_controller() {
  safe_directory "$state_dir" || die "Versioned installation has unsafe safe-upgrade state directory: $state_dir"
  controller="$state_dir/safe_upgrade.py"
  safe_regular_file "$controller" || die "Versioned installation requires a protected existing safe-upgrade controller: $controller"
  controller_sha="$(sha256_file "$controller")"
  allowed_controller_sha "$controller_sha" || die "Unsupported safe-upgrade controller SHA-256: $controller_sha. Refusing to bypass or replace the existing gate."
  "$controller" gate --root "$root" --state-dir "$state_dir" || die 'Existing safe-upgrade gate rejected this installation or reports an unfinished transaction.'
}

validate_versioned_transaction_preflight() {
  local output
  if ! output="$(PYTHONDONTWRITEBYTECODE=1 python3 - "$controller" "$root" "$state_dir" "$service" "$stage/$asset" "$expected_sha" "$stage/Glass.zip" "$theme_sha" "$target_version" 2>&1 <<'PY'
import argparse, importlib.util, pathlib, sys
controller, root, state_dir, service, binary, sha256, bundle, bundle_sha, version = sys.argv[1:]
spec = importlib.util.spec_from_file_location('komari_safe_upgrade_preflight', controller)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
args = argparse.Namespace(
    root=root,
    state_dir=state_dir,
    unit_dir='/etc/systemd/system',
    service=service,
    systemctl='systemctl',
    http_base='http://127.0.0.1:25774',
    binary=binary,
    sha256=sha256,
    theme_bundle=bundle,
    theme_sha256=bundle_sha,
    hostname=None,
    machine_id=None,
    database=None,
    expected_version=version,
)
module.preflight(args)
print('accepted')
PY
)"; then
    printf '[komari-update] Safe-upgrade transaction preflight failed:\n%s\n' "$output" >&2
    die 'The installed safe-upgrade controller rejected this release before service stop. Fix the reported controller, recovery, theme, database, or disk-space issue; do not bypass it.'
  fi
  [[ "$output" == 'accepted' ]] || die 'Safe-upgrade transaction preflight returned an unexpected result.'
}

prepare_flat_state_dir() {
  if [[ -e "$state_dir" ]]; then
    safe_directory "$state_dir" || die "Flat-layout upgrade state directory is unsafe: $state_dir"
  else
    install -d -m 0700 "$state_dir"
    safe_directory "$state_dir" || die "Unable to create protected flat-layout upgrade state directory: $state_dir"
  fi
}

flat_recovery_unit_name() {
  printf 'komari-flat-recovery-%s.service\n' "$(printf '%s\0%s\0%s' "$root" "$service" "$state_dir" | sha256sum | awk '{print substr($1, 1, 16)}')"
}

flat_recovery_script() { printf '%s/flat-recovery/update-komari.sh\n' "$state_dir"; }

durable_file() {
  local path=$1
  python3 - "$path" <<'PY'
import os, sys
path = sys.argv[1]
fd = os.open(path, os.O_RDONLY)
try:
    os.fsync(fd)
finally:
    os.close(fd)
directory = os.open(os.path.dirname(path), os.O_DIRECTORY)
try:
    os.fsync(directory)
finally:
    os.close(directory)
PY
}

flat_journal_write() {
  local phase=$1 journal="$state_dir/flat-journal.json"
  python3 - "$journal" "$phase" "$root" "$service" "$backup_dir" "$previous_version" "$target_version" "$previous_schema" "$target_schema" <<'PY'
import json, os, pathlib, sys, tempfile
journal, phase, root, service, backup, previous, target, old_schema, new_schema = sys.argv[1:]
path = pathlib.Path(journal)
payload = {'phase': phase, 'root': root, 'service': service, 'backup_dir': backup, 'previous_version': previous, 'target_version': target, 'previous_schema': int(old_schema), 'target_schema': int(new_schema)}
fd, temporary = tempfile.mkstemp(prefix='.flat-journal.', dir=path.parent)
try:
    with os.fdopen(fd, 'w', encoding='utf-8') as handle:
        json.dump(payload, handle, sort_keys=True); handle.write('\n'); handle.flush(); os.fsync(handle.fileno())
    os.replace(temporary, path)
    directory = os.open(path.parent, os.O_DIRECTORY)
    try: os.fsync(directory)
    finally: os.close(directory)
finally:
    try: os.unlink(temporary)
    except FileNotFoundError: pass
PY
}

install_flat_recovery() {
  local recovery_dir recovery_script recovery_unit unit_path temporary dropin_dir dropin_path updater_source
  prepare_flat_state_dir
  updater_source="$(realpath -e -- "$0")" || die 'Flat-layout recovery source updater is missing.'
  safe_invoked_updater "$updater_source" || die "Flat-layout recovery source updater is unsafe: $updater_source"
  recovery_dir="$state_dir/flat-recovery"
  recovery_script="$(flat_recovery_script)"
  recovery_unit="$(flat_recovery_unit_name)"
  [[ "$root$state_dir$unit_dir" != *$'\n'* && "$root$state_dir$unit_dir" != *' '* && "$service" =~ ^[A-Za-z0-9_.@:-]+$ ]] || die 'Flat-layout automatic recovery requires safe systemd names and paths without spaces or newlines.'
  if [[ -e "$recovery_dir" ]]; then safe_directory "$recovery_dir" || die "Flat-layout recovery directory is unsafe: $recovery_dir"; else install -d -m 0700 "$recovery_dir"; fi
  install -o root -g root -m 0700 "$updater_source" "$recovery_script"
  safe_regular_file "$recovery_script" || die "Flat-layout recovery helper is unsafe: $recovery_script"
  if [[ -e "$unit_dir" ]]; then safe_directory "$unit_dir" || die "Flat-layout recovery unit directory is unsafe: $unit_dir"; else die "Flat-layout recovery unit directory is missing: $unit_dir"; fi
  unit_path="$unit_dir/$recovery_unit"; temporary="$unit_path.new"
  cat >"$temporary" <<EOF
[Unit]
Description=Komari flat-layout interrupted-update recovery
ConditionPathExists=$state_dir/flat-journal.json
After=local-fs.target
Before=$service

[Service]
Type=oneshot
User=root
UMask=0077
Environment=KOMARI_ROOT=$root
Environment=KOMARI_SERVICE=$service
Environment=KOMARI_UPGRADE_STATE_DIR=$state_dir
Environment=KOMARI_UPGRADE_UNIT_DIR=$unit_dir
ExecStart=/bin/bash $recovery_script --recover-flat

[Install]
WantedBy=multi-user.target
EOF
  install -o root -g root -m 0600 "$temporary" "$unit_path"; rm -f -- "$temporary"
  safe_regular_file "$unit_path" || die "Flat-layout recovery unit is unsafe: $unit_path"
  dropin_dir="$unit_dir/$service.d"; dropin_path="$dropin_dir/10-komari-flat-recovery.conf"
  if [[ -e "$dropin_dir" ]]; then safe_directory "$dropin_dir" || die "Flat-layout service recovery drop-in directory is unsafe: $dropin_dir"; else install -d -o root -g root -m 0755 "$dropin_dir"; fi
  printf '[Unit]\nRequires=%s\nAfter=%s\n' "$recovery_unit" "$recovery_unit" >"$dropin_path.new"
  install -o root -g root -m 0600 "$dropin_path.new" "$dropin_path"; rm -f -- "$dropin_path.new"
  safe_regular_file "$dropin_path" || die "Flat-layout service recovery drop-in is unsafe: $dropin_path"
  systemctl daemon-reload || die 'Unable to reload systemd after installing flat-layout recovery.'
  systemctl enable "$recovery_unit" || die 'Unable to enable flat-layout automatic recovery before service stop.'
}

recover_flat_interrupted_transaction() {
  local journal="$state_dir/flat-journal.json" details phase saved_backup saved_previous saved_schema
  [[ -e "$journal" ]] || return 0
  safe_directory "$state_dir" || die "Flat-layout recovery state directory is unsafe: $state_dir"
  safe_regular_file "$journal" || die "Flat-layout recovery journal is unsafe: $journal"
  safe_regular_file "$(flat_recovery_script)" || die 'Flat-layout recovery helper is unsafe.'
  details="$(python3 - "$journal" "$root" "$service" <<'PY'
import json, pathlib, sys
journal, root, service = sys.argv[1:]
data = json.load(open(journal, encoding='utf-8'))
if data.get('root') != root or data.get('service') != service: raise SystemExit('journal belongs to another installation')
if data.get('phase') == 'rolled_back': raise SystemExit(0)
if data.get('phase') not in {'prepared', 'stopping', 'stopped', 'snapshotted', 'replaced', 'starting', 'recovering', 'recovered_files'}: raise SystemExit('unknown journal phase')
backup = pathlib.Path(data.get('backup_dir', ''))
if backup.parent not in (pathlib.Path(root) / 'backup', pathlib.Path(root) / 'backups') or backup.is_symlink(): raise SystemExit('unsafe backup directory')
previous = data.get('previous_version')
schema = data.get('previous_schema')
if not isinstance(previous, str) or not previous or not isinstance(schema, int) or schema < 0: raise SystemExit('invalid recovery metadata')
print(data['phase']); print(backup); print(previous); print(schema)
PY
)" || die 'Flat-layout recovery journal is malformed or belongs to another installation.'
  mapfile -t _flat_recovery < <(printf '%s\n' "$details")
  phase="${_flat_recovery[0]:-}"; saved_backup="${_flat_recovery[1]:-}"; saved_previous="${_flat_recovery[2]:-}"; saved_schema="${_flat_recovery[3]:-}"
  [[ -n "$phase" ]] || return 0
  safe_directory "$saved_backup" || die "Flat-layout recovery backup directory is unsafe: $saved_backup"
  safe_regular_file "$saved_backup/komari.previous" || die "Flat-layout recovery backup is missing or unsafe: $saved_backup/komari.previous"
  backup_dir="$saved_backup"; previous_version="$saved_previous"; target_version='recovery'; previous_schema="$saved_schema"; target_schema="$saved_schema"
  flat_journal_write recovering
  log "Automatically recovering unfinished flat-layout transaction ($phase) from $saved_backup."
  systemctl stop "$service" || true
  install -m 0755 "$saved_backup/komari.previous" "$root/komari.recovery"; mv -f "$root/komari.recovery" "$root/komari"
  if [[ "$phase" == 'snapshotted' || "$phase" == 'replaced' || "$phase" == 'starting' || "$phase" == 'recovering' || "$phase" == 'recovered_files' ]]; then
    safe_directory "$saved_backup/sqlite" || die 'Flat-layout recovery SQLite backup directory is missing or unsafe.'
    safe_regular_file "$saved_backup/sqlite/komari.db" || die 'Flat-layout recovery SQLite backup is missing or unsafe.'
    schema_from_database "$saved_backup/sqlite/komari.db" >/dev/null || die 'Flat-layout recovery SQLite backup failed integrity verification.'
    if [[ -e "$saved_backup/sqlite/metrics.db" ]]; then
      safe_regular_file "$saved_backup/sqlite/metrics.db" || die 'Flat-layout recovery metrics backup is unsafe.'
      schema_from_database "$saved_backup/sqlite/metrics.db" >/dev/null || die 'Flat-layout recovery metrics backup failed integrity verification.'
    fi
    install -m 0600 "$saved_backup/sqlite/komari.db" "$root/data/komari.db.recovery"; mv -f "$root/data/komari.db.recovery" "$root/data/komari.db"
    if [[ -e "$saved_backup/sqlite/metrics.db" ]]; then install -m 0600 "$saved_backup/sqlite/metrics.db" "$root/data/metrics.db.recovery"; mv -f "$root/data/metrics.db.recovery" "$root/data/metrics.db"; fi
  fi
  schema_from_database "$root/data/komari.db" >/dev/null || die 'Recovered SQLite database failed integrity verification.'
  flat_journal_write recovered_files
  systemctl start "$service" || die "Flat-layout recovery restored the previous executable but could not start $service."
  verify_flat_health "$saved_previous" || die "Flat-layout recovery restored the previous executable but it did not become healthy."
  flat_journal_write rolled_back
  log 'Flat-layout interrupted transaction was rolled back and the previous version is healthy.'
}

flat_backup_parent() {
  if [[ -e "$root/backup" ]]; then
    [[ -d "$root/backup" && ! -L "$root/backup" ]] || die "Unsafe flat-layout backup path: $root/backup"
    printf '%s\n' "$root/backup"
  else
    printf '%s\n' "$root/backups"
  fi
}

prepare_flat_backup_parent() {
  local parent=$1
  if [[ -e "$parent" ]]; then
    safe_directory "$parent" || die "Unsafe flat-layout backup directory: $parent"
  else
    install -d -m 0700 "$parent"
    safe_directory "$parent" || die "Unable to create a protected flat-layout backup directory: $parent"
  fi
  require_protected_tree_path "$parent"
}

restore_flat_after_failure() {
  local current_schema
  current_schema="$(schema_from_database "$root/data/komari.db" 2>/dev/null || printf unknown)"
  install -m 0755 "$backup_dir/komari.previous" "$root/komari.new"
  mv -f "$root/komari.new" "$root/komari"
  if (( target_schema > previous_schema )) && [[ "$current_schema" != "$previous_schema" ]]; then
    log "Migration risk detected: database schema changed from $previous_schema to $current_schema."
    log 'The previous executable was restored but is not started because it may be incompatible with the migrated database.'
    log "Use the consistent SQLite backups in $backup_dir/sqlite only with an operator-reviewed recovery plan."
    systemctl stop "$service" || true
    return 2
  fi
  systemctl start "$service" || return 1
  if verify_flat_health "$previous_version"; then
    flat_journal_write rolled_back
    log 'Flat-layout rollback completed and the previous version is healthy.'
    return 0
  fi
  log "Rollback executable was restored, but the previous service did not become healthy. Investigate $service and $backup_dir."
  return 1
}

update_flat() {
  local parent rollback_status
  parent="$(flat_backup_parent)"
  prepare_flat_backup_parent "$parent"
  backup_dir="$(mktemp -d "$parent/update-${resolved_tag}-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")"
  cp -a -- "$root/komari" "$backup_dir/komari.previous"
  durable_file "$backup_dir/komari.previous"
  printf '%s\n' "$previous_version" >"$backup_dir/previous-version.txt"
  printf '%s\n' "$resolved_tag" >"$backup_dir/target-release.txt"
  printf '%s\n' "$expected_sha" >"$backup_dir/$asset.sha256"
  durable_file "$backup_dir/previous-version.txt"
  durable_file "$backup_dir/target-release.txt"
  durable_file "$backup_dir/$asset.sha256"
  install_flat_recovery
  flat_journal_write prepared

  log 'Download and checksum verification succeeded. Stopping the service.'
  flat_journal_write stopping
  systemctl stop "$service"
  flat_journal_write stopped
  install -d -m 0700 "$backup_dir/sqlite"
  sqlite_backup "$root/data/komari.db" "$backup_dir/sqlite/komari.db"
  durable_file "$backup_dir/sqlite/komari.db"
  if [[ -f "$root/data/metrics.db" ]]; then
    sqlite_backup "$root/data/metrics.db" "$backup_dir/sqlite/metrics.db"
    durable_file "$backup_dir/sqlite/metrics.db"
  fi
  flat_journal_write snapshotted

  install -m 0755 "$stage/$asset" "$root/komari.new"
  mv -f "$root/komari.new" "$root/komari"
  flat_journal_write replaced
  flat_journal_write starting
  if ! systemctl start "$service" || ! verify_flat_health "$target_version"; then
    if restore_flat_after_failure; then rollback_status=0; else rollback_status=$?; fi
    if (( rollback_status == 2 )); then
      die "Flat-layout update failed after a database migration. Previous executable restored but service intentionally left stopped; consistent SQLite backup: $backup_dir/sqlite"
    fi
    die "Flat-layout update failed. Previous executable backup: $backup_dir"
  fi
  log "Flat-layout update successful. Backup location: $backup_dir"
  rm -f -- "$state_dir/flat-journal.json"
}

while (( $# > 0 )); do
  case "$1" in
    --help|-h) usage; exit 0 ;;
    --dry-run) dry_run=1 ;;
    --recover-flat) recover_flat=1 ;;
    v[0-9]*.[0-9]*.[0-9]*) [[ "$requested_tag" == 'latest' ]] || die 'Only one release version may be specified.'; requested_tag=$1 ;;
    *) die "Unknown argument: $1 (use --help)" ;;
  esac
  shift
done

(( EUID == 0 )) || die 'Run this script as root.'
for command in bash curl python3 sha256sum systemctl install mv cp awk sed date stat realpath readlink; do require_command "$command"; done
root="$(realpath -e -- "$root" 2>/dev/null || printf '%s' "$root")"
if (( recover_flat == 1 )); then
  recover_flat_interrupted_transaction
  exit 0
fi
detect_layout
verify_service_target
previous_version="$(version_from_binary "$current_binary")" || die "Existing executable does not support \`version --json\`."
previous_schema="$(schema_from_database "$root/data/komari.db")" || die 'Existing SQLite database failed integrity/schema inspection.'

if [[ "$layout" == 'versioned' ]]; then
  [[ "${current_release#releases/}" == "$previous_version" ]] || die "Versioned current symlink release does not match its executable version."
  prepare_versioned_controller
fi

safe_directory /root || die 'Unsafe root-owned staging parent: /root'
stage="$(mktemp -d /root/.komari-update.XXXXXX)"
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
if [[ "$layout" == 'versioned' ]]; then release_has_asset 'Glass.zip' || die "Release $resolved_tag lacks Glass.zip required by the installed safe-upgrade controller."; fi

download_asset "$resolved_tag" 'SHA256SUMS.txt' "$stage/SHA256SUMS.txt"
expected_sha="$(verify_download "$asset")"
chmod 0755 "$stage/$asset"
target_binary_version="$(version_from_binary "$stage/$asset")" || die "Downloaded release binary does not support \`version --json\` on this architecture."
[[ "$target_binary_version" == "$target_version" ]] || die "Release tag $resolved_tag does not match binary version $target_binary_version."
target_schema="$(schema_from_binary "$stage/$asset")" || die "Downloaded release binary does not support \`schema-version --json\`."
(( target_schema >= previous_schema )) || die "Refusing database downgrade: current schema is $previous_schema, target expects $target_schema."
if [[ "$layout" == 'versioned' ]]; then theme_sha="$(verify_download 'Glass.zip')"; fi

log "Installation layout: $layout"
log "Installation: $root ($service)"
log "Current executable: $current_binary"
if [[ "$layout" == 'versioned' ]]; then
  log "Current symlink targets: komari -> $(readlink -- "$root/komari"); current -> $current_release"
  log "Target release directory: $root/releases/$target_version"
  log "Safe-upgrade controller: $controller (SHA-256: $controller_sha)"
  log 'Safe-upgrade gate compatibility: accepted'
fi
log "Current version: $previous_version; database schema: $previous_schema"
log "Target release: $resolved_tag; selected asset: $asset"
log "SHA-256 verification: passed ($expected_sha)"
log "Target binary version: $target_binary_version; expected schema: $target_schema"

if [[ "$previous_version" == "$target_version" ]]; then
  log 'Requested version is already installed; no service interruption is required.'
  exit 0
fi

if [[ "$layout" == 'versioned' ]]; then
  validate_versioned_transaction_preflight
  log 'Safe-upgrade transaction preflight: accepted'
fi

if [[ "$layout" == 'flat' ]]; then
  log "Planned backup location: $(flat_backup_parent)/update-${resolved_tag}-<timestamp>-<random>"
  log 'Planned service actions: stop after verified preparation, create consistent SQLite backups, replace the regular executable, start, and health-check.'
else
  log "Planned backup location: $state_dir/backups/<transaction-id>"
  log "Planned symlink change: current -> releases/$target_version (atomic rename); komari -> current/komari remains unchanged"
  log 'Planned service actions: delegate the complete transaction, snapshot, switch, health check, rollback, and recovery handling to the existing safe-upgrade controller.'
fi

if (( dry_run == 1 )); then
  log 'Dry run complete. No service, symlink, database, upgrade journal, or backup files were changed.'
  exit 0
fi

if [[ "$layout" == 'versioned' ]]; then
  log 'Delegating the versioned transaction to the installed safe-upgrade controller.'
  "$controller" upgrade --root "$root" --state-dir "$state_dir" --service "$service" \
    --binary "$stage/$asset" --sha256 "$expected_sha" --expected-version "$target_version" \
    --theme-bundle "$stage/Glass.zip" --theme-sha256 "$theme_sha" || die 'Safe-upgrade controller failed. It retains the authoritative transaction journal and recovery state; run its documented recovery only after review.'
  [[ -L "$root/current" && "$(readlink -- "$root/current")" == "releases/$target_version" ]] || die 'Safe-upgrade controller returned success but current symlink is not the requested release.'
  [[ -L "$root/komari" && "$(readlink -- "$root/komari")" == 'current/komari' ]] || die 'Safe-upgrade controller returned success but komari symlink changed unexpectedly.'
  [[ "$(version_from_binary "$(require_within_root "$root/komari")")" == "$target_version" ]] || die 'Safe-upgrade controller returned success but the active executable version is incorrect.'
  "$controller" gate --root "$root" --state-dir "$state_dir" || die 'Safe-upgrade transaction completed but the pre-start gate does not accept its journal state.'
  log "Versioned update successful. Previous release preserved: $current_release; active release: releases/$target_version"
else
  update_flat
fi

log "Update successful. Previous version: $previous_version; target version: $target_version"

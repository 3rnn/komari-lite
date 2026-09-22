#!/usr/bin/env bash
# Native systemd update helper with guarded rollback.
set -euo pipefail

ROOT="${KOMARI_ROOT:-/opt/komari}"
SERVICE="${KOMARI_SERVICE:-komari.service}"
BIN="${1:-}"
: "${BIN:?usage: $0 /path/to/komari.new}"
[[ -x "$BIN" ]] || { echo "new binary is not executable: $BIN" >&2; exit 2; }
[[ -x "$ROOT/komari" ]] || { echo "current binary is not executable: $ROOT/komari" >&2; exit 2; }

if [[ -n "${EXPECTED_HOSTNAME:-}" && "$(hostname)" != "$EXPECTED_HOSTNAME" ]]; then
  echo "hostname does not match EXPECTED_HOSTNAME" >&2
  exit 2
fi
if [[ -n "${EXPECTED_MACHINE_ID:-}" && "$(cat /etc/machine-id)" != "$EXPECTED_MACHINE_ID" ]]; then
  echo "machine ID does not match EXPECTED_MACHINE_ID" >&2
  exit 2
fi

stamp="$(date +%Y%m%d-%H%M%S)"
backup="$ROOT/komari.prev.$stamp"
staged="$ROOT/komari.new.$stamp"
updated=0

rollback() {
  local status=$?
  if (( updated )); then
    echo "update failed; restoring $backup" >&2
    [[ -f "$backup" ]] && cp -f "$backup" "$ROOT/komari"
    systemctl restart "$SERVICE" || true
  fi
  exit "$status"
}
trap rollback ERR

cp -f "$BIN" "$staged"
chmod 0755 "$staged"
cp -f "$ROOT/komari" "$backup"

systemctl stop "$SERVICE"
mv -f "$staged" "$ROOT/komari"
updated=1
systemctl start "$SERVICE"
systemctl is-active --quiet "$SERVICE"
updated=0
trap - ERR

rm -f "$BIN"
echo "updated: $(sha256sum "$ROOT/komari" | awk '{print $1}')"
echo "rollback binary: $backup"

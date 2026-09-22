#!/usr/bin/env bash
# Native systemd update helper. Configure optional identity guards through env.
set -euo pipefail

ROOT="${KOMARI_ROOT:-/opt/komari}"
SERVICE="${KOMARI_SERVICE:-komari.service}"
BIN="${1:-}"
: "${BIN:?usage: $0 /path/to/komari.new}"
[[ -x "$BIN" ]] || { echo "new binary is not executable: $BIN" >&2; exit 2; }

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
cp -f "$BIN" "$staged"
chmod 0755 "$staged"

rollback() {
  echo "verification failed; restoring $backup" >&2
  [[ -f "$backup" ]] && mv -f "$backup" "$ROOT/komari"
  systemctl restart "$SERVICE" || true
}

systemctl stop "$SERVICE"
cp -f "$ROOT/komari" "$backup"
mv -f "$staged" "$ROOT/komari"
systemctl start "$SERVICE"

echo "updated: $(sha256sum "$ROOT/komari" | awk '{print $1}')"
echo "rollback binary: $backup"

#!/usr/bin/env bash
# Legacy binary-only updater is deliberately disabled: schema migrations require data rollback.
set -euo pipefail
printf '%s\n' 'Binary-only updates are unsafe. Use deploy/safe_upgrade.py install-gate and upgrade with pinned binary/theme SHA-256 values; see deploy/README.md.' >&2
exit 2

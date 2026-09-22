#!/usr/bin/env bash
set -euo pipefail

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$root"

fail=0
reject_path() {
  local pattern="$1"
  local found
  found="$(git ls-files -co --exclude-standard | grep -E "$pattern" || true)"
  if [[ -n "$found" ]]; then
    printf 'Blocked sensitive/generated path(s):\n%s\n' "$found" >&2
    fail=1
  fi
}

reject_path '(^|/)(data|secrets?|credentials?|backup|backups|rollback|agent-release)(/|$)'
reject_path '(^|/)(\.env|.*\.(db|sqlite|sqlite3|pem|key|p12|pfx|crt|csr|zip|tar|tgz|gz)|komari|komari-[^/]+)$'

# Scan tracked and not-yet-tracked text files without echoing matching secrets.
if git grep -nI -E \
  '(-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,})' \
  -- . ':!README.md' ':!**/*_test.go' ':!**/*.test.ts' >/dev/null 2>&1; then
  echo 'Blocked possible credential material in repository text; inspect git grep locally.' >&2
  fail=1
fi

if (( fail )); then
  exit 1
fi

echo 'GitHub preflight passed: no blocked runtime files or recognizable credential material.'

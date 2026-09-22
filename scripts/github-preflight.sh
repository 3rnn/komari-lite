#!/usr/bin/env bash
# Release gate: refuse to stage or commit runtime state, credentials, keys, or build output.
set -euo pipefail

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$root"

fail=0
reject_path() {
  local label="$1"
  local pattern="$2"
  local found
  found="$(git ls-files -co --exclude-standard | grep -E "$pattern" || true)"
  if [[ -n "$found" ]]; then
    printf 'Blocked %s:\n%s\n' "$label" "$found" >&2
    fail=1
  fi
}

# Panel runtime state and release output live at the repository root.
reject_path 'runtime directory' '^/(data|secrets|credentials|backup|backups|rollback|release|agent-release|logs)/'
reject_path 'runtime data file' '(^|/)data/'
reject_path 'database or key material' '\.(db|db-wal|db-shm|sqlite|sqlite3|pem|key|p12|pfx|crt|csr)$'
reject_path 'environment file' '(^|/)\.env(\.local|\.(production|development|test|staging)(\.local)?)?$'
reject_path 'generated binary' '^/komari($|\.)|(^|/)komari-agent-[^/]+$'
reject_path 'archive' '\.(zip|tar|tgz|tar\.gz|gz)$'

# Scan tracked and not-yet-tracked text files without echoing matching secrets.
if git grep -nI -E \
  '(-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,})' \
  -- . ':!README.md' ':!README.zh-CN.md' ':!**/*_test.go' ':!**/*.test.ts' >/dev/null 2>&1; then
  echo 'Blocked possible credential material in repository text; inspect git grep locally.' >&2
  fail=1
fi

# Guard against a real deployment identity leaking into templates or docs.
if git grep -nI -E '(128\.241\.255\.1|ai\.8586898\.xyz|8586898)' -- . ':!scripts/github-preflight.sh' >/dev/null 2>&1; then
  echo 'Blocked host-specific deployment identifier in repository text.' >&2
  fail=1
fi

if (( fail )); then
  exit 1
fi

echo 'GitHub preflight passed: no runtime state, key material, or host-specific identifiers found.'

#!/usr/bin/env bash
# Release gate: refuse to stage or commit runtime state, credentials, keys, or build output.
set -euo pipefail

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$root"

fail=0
# Scan only the proposed change set. Historical compatibility bundles and prior
# task records are tracked baseline material; release hygiene must reject newly
# introduced runtime state or deployment identities without making the existing
# repository permanently unpublishable.
candidate_paths() {
  { git diff --name-only HEAD; git ls-files --others --exclude-standard; } | LC_ALL=C sort -u
}

reject_path() {
  local label="$1"
  local pattern="$2"
  local found
  found="$(candidate_paths | grep -E "$pattern" || true)"
  if [[ -n "$found" ]]; then
    printf 'Blocked %s:\n%s\n' "$label" "$found" >&2
    fail=1
  fi
}

# Panel runtime state and release output live at the repository root.
reject_path 'runtime directory' '^(data|secrets|credentials|backup|backups|rollback|release|agent-release|logs)/'
reject_path 'runtime data file' '(^|/)data/'
reject_path 'database or key material' '\.(db|db-wal|db-shm|sqlite|sqlite3|pem|key|p12|pfx|crt|csr)$'
reject_path 'environment file' '(^|/)\.env(\.local|\.(production|development|test|staging)(\.local)?)?$'
reject_path 'generated binary' '^komari($|\.)|(^|/)komari-agent-[^/]+$'
reject_path 'archive' '\.(zip|tar|tgz|tar\.gz|gz)$'

# Scan added text only so legacy tracked documentation cannot mask a clean
# release candidate or force an unrelated historical rewrite.
added_text="$(git diff --no-ext-diff --unified=0 HEAD -- . ':(exclude)scripts/github-preflight.sh' | grep -E '^\+[^+]' || true)"
if grep -E \
  '(-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,})' \
  <<<"$added_text" >/dev/null 2>&1; then
  echo 'Blocked possible credential material in added repository text; inspect the candidate diff locally.' >&2
  fail=1
fi

# Guard against a real deployment identity leaking into proposed templates or docs.
if grep -E '(128\.241\.255\.1|ai\.8586898\.xyz|8586898)' <<<"$added_text" >/dev/null 2>&1; then
  echo 'Blocked host-specific deployment identifier in added repository text.' >&2
  fail=1
fi

if (( fail )); then
  exit 1
fi

echo 'GitHub preflight passed: no runtime state, key material, or host-specific identifiers found.'

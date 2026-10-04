#!/usr/bin/env bash
# GitHub one-line bootstrap for the pinned v1.0.16 + 33-file Glass upgrade.
# The stream is never used as the transaction controller: download a reviewed,
# immutable Python helper to a root-private path, verify its size and SHA-256,
# then let it perform all stock/DB/service checks and the confirmed update.
set -euo pipefail

mode="${1:-update}"
if [[ "$#" -gt 1 || ( "$mode" != check && "$mode" != update && "$mode" != status ) ]]; then
    printf 'Usage: upgrade.sh [check|update|status]\n' >&2
    exit 2
fi
if [[ "$EUID" -ne 0 ]]; then
    printf 'Run as root through sudo.\n' >&2
    exit 1
fi
if [[ "$mode" == update && ! -t 2 ]]; then
    printf 'An interactive terminal is required to confirm the update.\n' >&2
    exit 1
fi
if [[ ! -x /usr/bin/python3 || ! -x /usr/bin/sha256sum || ! -x /usr/bin/stat ||
      ! -x /usr/bin/mktemp || ! -x /usr/bin/rm || ! -x /usr/bin/rmdir ]]; then
    printf 'Missing required system tools.\n' >&2
    exit 1
fi

helper_url='https://raw.githubusercontent.com/3rnn/komari-lite/29019b4afd7aed19862b24b5d94f48ce48737315/deploy/komari-compat-v1019.py'
helper_sha='26947c8ffd7f0f535ca244365286e41e2182cf3b0c61a79a2542c8f9ae8604a2'
helper_size=20164
umask 077
stage=$(/usr/bin/mktemp -d /var/lib/komari-compat.XXXXXXXXXX)
cleanup() {
    /usr/bin/rm -f -- "$stage/k19a.py"
    /usr/bin/rmdir -- "$stage" || true
}
trap cleanup EXIT

if [[ -x /usr/bin/curl ]]; then
    /usr/bin/curl --fail --location --silent --show-error \
        --proto '=https' --proto-redir '=https' --connect-timeout 15 \
        --max-time 180 --max-filesize "$helper_size" \
        --output "$stage/k19a.py" "$helper_url"
elif [[ -x /usr/bin/wget ]]; then
    /usr/bin/wget --quiet --https-only --max-redirect=5 --timeout=25 --tries=2 \
        --quota=1m --output-document="$stage/k19a.py" "$helper_url"
else
    printf 'curl or wget is required.\n' >&2
    exit 1
fi
if [[ "$(/usr/bin/stat -c %s "$stage/k19a.py")" != "$helper_size" ]]; then
    printf 'Downloaded helper size mismatch; refusing execution.\n' >&2
    exit 1
fi
printf '%s  %s\n' "$helper_sha" "$stage/k19a.py" | /usr/bin/sha256sum -c - >/dev/null
printf 'Verified fixed-commit Komari upgrade helper.\n' >&2

if [[ "$mode" == update ]]; then
    /usr/bin/python3 "$stage/k19a.py" update </dev/tty
else
    /usr/bin/python3 "$stage/k19a.py" "$mode"
fi

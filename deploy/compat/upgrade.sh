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

helper_url='https://raw.githubusercontent.com/3rnn/komari-lite/99adb189ebc83f14dcbef05b8c02f4ea03826dee/deploy/komari-compat-v1019.py'
helper_sha='c2d9a50385b2584134f5391f57ba36539a2cf5ed3fb6f78d60f67d6d33229ba5'
helper_size=23675
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

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

helper_url='https://raw.githubusercontent.com/3rnn/komari-lite/57034cd6c4104ca9c9c3db3898e68d1fa99588d9/deploy/komari-compat-v1019.py'
helper_sha='5610bfffd020393eeb351d415ae60dd87c5b94447b2eb44eebe20b3b7d09efcf'
helper_size=29951
umask 077
stage=$(/usr/bin/mktemp -d /var/lib/komari-compat.XXXXXXXXXX)
cleanup() {
    /usr/bin/rm -f -- "$stage/k19a.py"
    /usr/bin/rmdir -- "$stage" || true
}
trap cleanup EXIT

# Cap a streamed helper before its checksum is checked; wget --quota alone
# does not constrain an individual response. Python is already a prerequisite.
bounded_download() {
    /usr/bin/python3 -c 'import os, re, resource, subprocess, sys, tempfile, time
from urllib.parse import urljoin, urlsplit
limit = int(sys.argv[1])
resource.setrlimit(resource.RLIMIT_FSIZE, (limit, limit))
command = sys.argv[2:]
if os.path.basename(command[0]) != "wget":
    subprocess.run(command, check=True, timeout=180)
else:
    # GNU wget --https-only does not restrict redirect schemes. Disable its
    # automatic redirects and validate each Location before a new connection.
    deadline = time.monotonic() + 180
    url = command[-1]
    for hop in range(6):
        parsed = urlsplit(url)
        if (parsed.scheme.lower() != "https" or not parsed.hostname or
                parsed.username is not None or parsed.password is not None or
                any(ord(char) <= 32 or ord(char) == 127 for char in url)):
            raise ValueError("Helper redirect must use a valid HTTPS URL")
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError("Helper download deadline exceeded")
        # Disk-backed diagnostics are capped by the same inherited limit as
        # the helper, including when a peer streams excessive response headers.
        with tempfile.TemporaryFile() as headers:
            result = subprocess.run(command[:-1] + [url], stderr=headers,
                                    timeout=remaining)
            headers.seek(0)
            diagnostic = headers.read(limit + 1)
        if len(diagnostic) > limit:
            raise ValueError("Helper response headers exceed limit")
        status, locations = None, []
        for line in diagnostic.decode("iso-8859-1").splitlines():
            match = re.match(r"^\s+HTTP/\S+\s+(\d{3})(?:\s|$)", line)
            if match:
                status, locations = int(match[1]), []
            elif re.match(r"^\s+Location:", line, re.IGNORECASE):
                locations.append(line.split(":", 1)[1].strip())
        if status in (301, 302, 303, 307, 308):
            if hop == 5 or len(locations) != 1:
                raise ValueError("Helper redirect limit or invalid Location")
            url = urljoin(url, locations[0])
            continue
        if result.returncode != 0 or status is None or not 200 <= status < 300:
            raise ValueError("Helper HTTPS download failed")
        break
' "$helper_size" "$@"
}

if [[ -x /usr/bin/curl ]]; then
    bounded_download /usr/bin/curl --disable --fail --location --max-redirs 5 --silent --show-error \
        --proto '=https' --proto-redir '=https' --connect-timeout 15 \
        --max-time 180 --max-filesize "$helper_size" \
        --output "$stage/k19a.py" "$helper_url"
elif [[ -x /usr/bin/wget ]]; then
    bounded_download /usr/bin/wget --no-config --quiet --server-response --https-only --max-redirect=0 --timeout=25 --tries=2 \
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

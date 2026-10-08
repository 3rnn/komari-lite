#!/usr/bin/env bash
# Build the complete GitHub Release inventory from a clean, tagged source tree.
set -Eeuo pipefail
IFS=$'\n\t'

root="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
version="${1:?usage: $0 <vX.Y.Z> [output-directory] [previous-tag]}"
out="${2:-$root/release/$version}"
previous_tag="${3:-v1.0.20}"
revision="$(git -C "$root" rev-parse HEAD)"

[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Version must be vX.Y.Z' >&2; exit 1; }
[[ "$previous_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Previous tag must be vX.Y.Z' >&2; exit 1; }
for command in go node npm python3 sha256sum git aarch64-linux-gnu-gcc; do
  command -v "$command" >/dev/null || { echo "Missing required command: $command" >&2; exit 1; }
done

git -C "$root" diff --check
git -C "$root" rev-parse --verify "$previous_tag^{commit}" >/dev/null
mkdir -p "$out"
rm -f "$out"/komari "$out"/komari-linux-amd64 "$out"/komari-linux-arm64 \
  "$out"/komari-agent-* "$out"/install.sh "$out"/install.ps1 "$out"/manifest.json \
  "$out"/SHA256SUMS.txt "$out"/Glass.zip "$out"/komari-manager.py "$out"/safe_upgrade.py \
  "$out"/komari-oneclick.py "$out"/update-komari.sh "$out"/release-manifest.json

# Build the embedded administrator UI before compiling either panel binary.
(
  cd "$root/frontend"
  npm ci
  npm test
  npm run lint
  npm run build
)

# CGO is required by the SQLite driver. Linux arm64 uses the Debian cross compiler.
(
  cd "$root/backend"
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=true -ldflags="-s -w -X github.com/komari-monitor/komari/utils.VersionHash=$revision" -o "$out/komari-linux-amd64" .
  CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=true -ldflags="-s -w -X github.com/komari-monitor/komari/utils.VersionHash=$revision" -o "$out/komari-linux-arm64" .
  go test ./...
  go vet ./...
)
# Preserve the established generic amd64 asset for the existing safe controller.
cp "$out/komari-linux-amd64" "$out/komari"

(
  cd "$root/agent"
  go test ./...
  go vet ./...
)
"$root/scripts/build-agent-release.sh" "${version#v}" "$out"
# The Agent builder writes its own agent-only checksum file. Replace it below
# with the complete release inventory so it never attempts to checksum itself.
rm -f "$out/SHA256SUMS.txt"

# The existing safe controller requires a stock old->new Glass transition bundle.
old_tree="$(mktemp -d "${TMPDIR:-/tmp}/komari-release-old.XXXXXX")"
cleanup() { rm -rf -- "$old_tree"; }
trap cleanup EXIT

git -C "$root" archive "$previous_tag" backend/web/public/bundledThemes/Glass | tar -x -C "$old_tree"
PYTHONDONTWRITEBYTECODE=1 python3 "$root/deploy/build-theme-bundle.py" \
  --old "$old_tree/backend/web/public/bundledThemes/Glass" \
  --new "$root/backend/web/public/bundledThemes/Glass" \
  --output "$out/Glass.zip"

install -m 0755 "$root/update-komari.sh" "$out/update-komari.sh"
install -m 0755 "$root/deploy/komari-oneclick.py" "$out/komari-oneclick.py"
install -m 0644 "$root/deploy/komari-manager.py" "$out/komari-manager.py"
install -m 0644 "$root/deploy/safe_upgrade.py" "$out/safe_upgrade.py"

# One manifest covers the panel, updater, controllers, theme transition, and Agents.
(
  cd "$out"
  find . -maxdepth 1 -type f ! -name '.SHA256SUMS.txt' -printf '%f\n' | LC_ALL=C sort | while IFS= read -r file; do
    sha256sum "$file"
  done > .SHA256SUMS.txt
  mv -f .SHA256SUMS.txt SHA256SUMS.txt
  sha256sum -c SHA256SUMS.txt
)

python3 - "$out" "$version" <<'PY'
import json, pathlib, re, sys
out = pathlib.Path(sys.argv[1])
version = sys.argv[2]
entries = {}
for line in (out/'SHA256SUMS.txt').read_text().splitlines():
    digest, name = line.split(None, 1)
    entries[name.strip()] = digest
for name in ('komari-linux-amd64', 'komari-linux-arm64', 'komari', 'update-komari.sh', 'Glass.zip'):
    if name not in entries: raise SystemExit('missing release asset: ' + name)
(out/'release-manifest.json').write_text(json.dumps({'tag': version, 'artifacts': entries}, indent=2, sort_keys=True) + '\n')
PY
(
  cd "$out"
  sha256sum release-manifest.json >> SHA256SUMS.txt
  sha256sum -c SHA256SUMS.txt
)
printf 'Built release artifacts in %s\n' "$out"

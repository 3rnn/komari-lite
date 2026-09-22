#!/usr/bin/env bash
# Build the monitoring-only Agent for the panel's local Agent distribution.
set -euo pipefail

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
version="${1:?usage: $0 <version> [output-directory]}"
out="${2:-$root/release/agent-$version}"

command -v go >/dev/null || { echo 'Go is required' >&2; exit 1; }
rm -rf "$out"
mkdir -p "$out"

targets=(
  linux/amd64 linux/arm64 linux/386 linux/arm linux/loong64
  windows/amd64 windows/arm64 windows/386
  darwin/amd64 darwin/arm64
  freebsd/amd64 freebsd/arm64 freebsd/386 freebsd/arm
)

for target in "${targets[@]}"; do
  goos="${target%%/*}"
  goarch="${target##*/}"
  extension=""
  [[ "$goos" == windows ]] && extension=.exe
  artifact="komari-agent-$goos-$goarch$extension"
  echo "Building $artifact"
  (
    cd "$root/agent"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags '-s -w' -o "$out/$artifact" .
  )
done

python3 - "$out" "$version" <<'PY'
import hashlib, json, pathlib, sys
out = pathlib.Path(sys.argv[1])
artifacts = {}
for path in sorted(out.glob('komari-agent-*')):
    artifacts[path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
(out / 'manifest.json').write_text(
    json.dumps({'version': sys.argv[2], 'artifacts': artifacts}, indent=2) + '\n',
    encoding='utf-8',
)
(out / 'SHA256SUMS.txt').write_text(
    ''.join(f'{digest}  {name}\n' for name, digest in artifacts.items()),
    encoding='utf-8',
)
print(f'Built {len(artifacts)} artifacts in {out}')
PY

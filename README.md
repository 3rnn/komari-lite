# Komari Lite Monitor

A monitoring-focused, source-only fork of Komari. The repository is ready for public GitHub hosting: it contains **no** production database, user accounts, Agent tokens, TLS certificates, credentials, backups, release binaries, or inherited Git history.

[中文说明见 `README.zh-CN.md`](README.zh-CN.md)

## What this fork provides

- Nodes, live/historical metrics, Ping tasks, alerting, audit log, login and 2FA.
- A fixed local Glass public dashboard with Simplified Chinese and English support.
- A monitoring-only Agent distributed by the panel itself, rather than GitHub.
- Native systemd deployment support.

It intentionally excludes remote terminal/control/command execution, Cloudflare tunnel management, self-update/version checks, theme market/lifecycle actions, Nezha, reverse-route checks, and other UI languages.

## Layout

- `backend/` — Go panel service and its embedded public/admin assets.
- `frontend/` — React/Vite administrator UI source.
- `agent/` — monitoring-only Go Agent source.
- `scripts/build-agent-release.sh` — builds Agent releases for the panel.
- `deploy/` — generic native systemd update template; no production values.

## Requirements

- Go 1.25 or newer. Build the panel with CGO enabled for SQLite.
- Node.js 20+ and npm for the administrator UI.
- Linux tools: `bash`, `python3`, `sha256sum`, and optionally Caddy for TLS termination.

## 1. Build the panel

The Go panel embeds the compiled administrator UI, so build in this order:

```bash
git clone <YOUR_GITHUB_REPOSITORY_URL> komari-lite
cd komari-lite

cd frontend
npm ci
npm test
npm run build

# Copy the current UI build into the Go embed directory.
rm -rf ../backend/web/public/systemUI/dist
mkdir -p ../backend/web/public/systemUI/dist
cp -a dist/. ../backend/web/public/systemUI/dist/

cd ../backend
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go vet ./...
CGO_ENABLED=1 go build -trimpath -ldflags '-s -w' -o ../release/komari .
```

The panel binary is then `release/komari`.

## 2. Build the monitoring Agent

The Agent has no remote-control, terminal, command-execution, MCP, or self-update modules. Build all supported platform artifacts and a panel distribution manifest:

```bash
cd komari-lite
chmod +x scripts/build-agent-release.sh
./scripts/build-agent-release.sh 1.0.5
```

Output is written to `release/agent-1.0.5/`:

- 14 `komari-agent-<os>-<arch>` artifacts;
- `manifest.json` with the release version and SHA-256 for every artifact;
- `SHA256SUMS.txt` for offline verification.

The release directory is intentionally ignored by Git. Verify it before deployment:

```bash
cd release/agent-1.0.5
sha256sum -c SHA256SUMS.txt
```

## 3. Traditional native deployment (Debian/systemd)

These steps use no Docker. Replace example names and domain values with your own.

### On the server

```bash
sudo useradd --system --home /opt/komari --shell /usr/sbin/nologin komari
sudo install -d -m 0775 -o root -g komari /opt/komari
sudo install -d -o komari -g komari /opt/komari/data
sudo install -m 0755 release/komari /opt/komari/komari
sudo chown -R komari:komari /opt/komari/data
```

Create `/etc/systemd/system/komari.service`:

```ini
[Unit]
Description=Komari Lite Monitor
After=network-online.target
Wants=network-online.target

[Service]
User=komari
Group=komari
WorkingDirectory=/opt/komari
ExecStart=/opt/komari/komari server --listen 127.0.0.1:25774
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/opt/komari

[Install]
WantedBy=multi-user.target
```

`/opt/komari` is group-writable only by the service group so backup restore can create
same-parent staging directories and atomically exchange `data/`. The binary remains
root-owned and mode `0755`. `Restart=always` is required because restore deliberately
exits with status 0 after staging the archive, so systemd can start the restore pass.

Then start it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now komari.service
sudo systemctl status komari.service
```

### TLS reverse proxy with Caddy

Create `/etc/caddy/Caddyfile` and replace `monitor.example.com`:

```caddy
monitor.example.com {
    reverse_proxy 127.0.0.1:25774
}
```

Install/start Caddy according to its official documentation. The panel stays private on loopback; Caddy owns ports 80 and 443.

### Agent downloads

One-click deployment downloads the installer from the panel, then pins the Agent binary to the GitHub Release tag matching that panel version. A traditional deployment therefore only needs the panel binary; it does **not** need a `data/agent-release/` directory.

Before publishing a panel version, upload all 14 `komari-agent-*` artifacts to the same GitHub Release tag. The installer downloads the platform-matched artifact from:

```text
https://github.com/3rnn/komari-lite/releases/download/v<panel-version>/komari-agent-<os>-<arch>
```

## 4. Updating the panel binary

Copy a verified replacement binary to `/opt/komari/komari.new`, then run the generic helper on the server:

```bash
sudo install -m 0755 deploy/update-native.sh /opt/komari/update.sh

export EXPECTED_HOSTNAME="your-hostname"
export EXPECTED_MACHINE_ID="$(cat /etc/machine-id)"
sudo /opt/komari/update.sh /opt/komari/komari.new
```

`deploy/update-native.sh` keeps a timestamped previous binary and restores it if the update process fails. See [`deploy/README.md`](deploy/README.md).

## GitHub release hygiene

Never commit `data/`, Agent release artifacts, credentials, keys/certificates, databases, backups, or compiled binaries. `.gitignore` blocks these paths.

Before every push:

```bash
./scripts/github-preflight.sh
git diff --cached --check || true  # upstream/minified assets contain historic whitespace
```

`github-preflight.sh` rejects runtime data and recognizable private-key/token patterns. Review `git diff --cached` as the final safety check.

## License

The upstream project license is retained in [`LICENSE`](LICENSE) and [`backend/NOTICE`](backend/NOTICE). Preserve upstream notices and comply with their terms when redistributing this fork.

# Komari Lite Monitor

A source-only, monitoring-focused fork of Komari. This repository is prepared for public GitHub hosting: it contains no production database, user accounts, Agent tokens, TLS certificates, credentials, backups, release binaries, or Git history from the upstream worktrees.

## Scope

Kept: node monitoring, live and historical metrics, Ping, alerting, login/2FA, audit, backup/restore, public Glass dashboard, and local panel-hosted Agent distribution.

Removed: remote terminal/control/command execution, Cloudflare tunnel management, self-update/version checks, theme market, theme import/upload/update/switching, Nezha, reverse-route checks, and all but Simplified Chinese and English.

The public dashboard uses the fixed local `Glass` theme. It is initialized only when missing; later application builds do not overwrite an existing local Glass copy. Theme settings remain available, but theme lifecycle APIs are deliberately unavailable.

## Repository layout

- `backend/` — Go service and embedded public/system UI assets.
- `frontend/` — React/Vite administrator UI source.
- `deploy/` — native systemd deployment, update, and verification scripts.

## Build

Requirements: Go 1.25+ with CGO enabled, Node.js/npm.

```bash
cd frontend
npm ci
npm run build

# Embed the built administrator UI into the Go service.
rm -rf ../backend/web/public/systemUI/dist
mkdir -p ../backend/web/public/systemUI/dist
cp -a dist/. ../backend/web/public/systemUI/dist/

cd ../backend
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go vet ./...
CGO_ENABLED=1 go build -trimpath -ldflags '-s -w' -o ../komari .
```

## Run locally

```bash
mkdir -p data
./komari --database ./data/komari.db
```

Do not commit `data/`: it holds runtime state such as accounts, nodes, tokens, dashboard settings, and metrics configuration.

## Native deployment

Read `deploy/NATIVE-DEPLOYMENT.md`. The deployment scripts target a native systemd installation with the panel bound to loopback and Caddy handling TLS. Review target host values before use.

## Public-release hygiene

Before every push, run:

```bash
./scripts/github-preflight.sh
```

The check rejects runtime databases, cryptographic key material, credentials, generated binaries, backups, and common token/private-key signatures. It is a safety net, not a replacement for reviewing `git diff --cached`.

## License

The upstream project license is retained in [`backend/LICENSE`](backend/LICENSE). Preserve upstream notices and comply with its terms when redistributing this fork.

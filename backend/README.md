# Backend (panel service)

Go service that serves the administrator API, the public dashboard, and the embedded system UI.

Build instructions, native deployment, and the Agent release workflow live in the repository root:

- [`../README.md`](../README.md) — build, deploy, and release (English)
- [`../README.zh-CN.md`](../README.zh-CN.md) — 同上（简体中文）

## Layout

- `main.go`, `cmd/` — entrypoints and CLI commands.
- `database/` — SQLite models, migrations, and client state.
- `pkg/`, `utils/` — configuration, HTTP/HTTPS listeners, GeoIP, and helpers.
- `protocol/`, `web/rpc/jsonrpc/` — Agent protocol and the JSON-RPC v2 method surface.
- `web/api/` — REST handlers that stay outside the RPC bridge (authentication, uploads, installers, public asset routes).
- `web/public/` — embedded assets: `systemUI` (built administrator UI), `bundledThemes` (the fixed local Glass theme), `rescueTheme`.

## Testing

```bash
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go vet ./...
```

Keep the surface monitoring-only: no remote control, terminal, command execution, tunnel management, self-update, or theme lifecycle endpoints. Removed routes are blocked in `web/router/lite.go` and covered by `web/router/theme_lite_test.go`.

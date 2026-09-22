# Embedded Web Resources / 内嵌 Web 资源

Komari Lite packages two independent resource groups:

- `systemUI`: the administration UI built from `frontend/` and embedded into the panel binary.
- `bundledThemes/Glass`: the fixed local public dashboard theme. It is installed into `data/theme/Glass` only when that directory is missing, so later builds never overwrite the running copy.

`rescueTheme` is a minimal fallback shown only when the selected public theme is unavailable.

Public themes cannot serve system pages or `/system-assets/*`; custom Head/Body HTML is injected only into public dashboard documents.

Komari Lite 内嵌两组相互独立的资源：

- `systemUI`：由 `frontend/` 构建并嵌入面板二进制的后台管理界面。
- `bundledThemes/Glass`：固定的本地公开大屏主题；仅当 `data/theme/Glass` 缺失时安装，后续构建不会覆盖正在运行的副本。

`rescueTheme` 仅在选定的公开主题不可用时作为兜底页面。

大屏主题不能提供系统页面或 `/system-assets/*`；自定义 Head/Body HTML 只注入公开大屏页面。

## Build the system UI / 构建后台界面

```bash
cd frontend
npm ci
npm run build

rm -rf ../backend/web/public/systemUI/dist
mkdir -p ../backend/web/public/systemUI/dist
cp -a dist/. ../backend/web/public/systemUI/dist/
```

Before building the Go binary, verify these files exist:

```text
web/public/systemUI/dist/index.html
web/public/bundledThemes/Glass/dist/index.html
web/public/rescueTheme/dist/index.html
```

The built `systemUI/dist` and `rescueTheme/dist` trees are committed so that
`go build` works without a Node toolchain; regenerate them from `frontend/`
whenever the administrator UI changes. `backend/web/public/.gitignore` ignores
scratch `dist/` trees created next to the sources.

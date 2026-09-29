# Embedded Web Resources

Komari Lite packages two independent resource groups:

- `systemUI`: the administration UI built from `frontend/` and embedded into the panel binary.
- `bundledThemes/Glass`: the fixed local public dashboard theme. It is installed into `data/theme/Glass` only when that directory is missing, so later builds never overwrite the running copy.

`rescueTheme` is a fallback shown only when the selected public theme is unavailable.

Public themes cannot serve system pages or `/system-assets/*`; custom Head/Body HTML is injected only into public dashboard documents.

## Build the system UI

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

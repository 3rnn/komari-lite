import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");
const css = read("../src/global.css");

test("new visitors start with the dark jade Komari palette without removing appearance controls", () => {
  const theme = read("../src/contexts/ThemeContext.ts");
  assert.match(theme, /appearance:\s*"dark" as Appearance/);
  assert.match(theme, /color:\s*"jade" as Colors/);
  assert.match(theme, /allowedAppearances\s*=\s*\["light", "dark", "system"\]/);
  assert.match(theme, /allowedColors\s*=\s*\[/);
});

test("shared visual tokens distinguish background, surface, border and muted text in both modes", () => {
  for (const token of ["--km-canvas", "--km-sidebar", "--km-panel", "--km-panel-raised", "--km-border", "--km-muted", "--km-nav-accent", "--km-radius"]) {
    assert.ok(css.includes(`${token}:`), `missing ${token}`);
  }
  assert.match(css, /--km-nav-accent:\s*var\(--accent-11\)/);
  assert.match(css, /\.dark\s*\{[^}]*--km-canvas:\s*#[0-9a-f]{6}/s);
  assert.match(css, /:root\s*\{[^}]*--km-canvas:\s*#[0-9a-f]{6}/s);
  assert.match(css, /\.km-panel\s*\{[^}]*background:\s*var\(--km-panel\);[^}]*border:\s*1px solid var\(--km-border\)/s);
  assert.match(css, /\.km-kicker\s*\{[^}]*font-family:\s*var\(--km-metric-font\)/s);
});

test("standalone login uses the shared Komari panel instead of a gray surface", () => {
  const login = read("../src/components/Login.tsx");
  assert.match(login, /<section className="km-panel/);
  assert.doesNotMatch(login, /<section className="rounded-lg border border-\[var\(--gray-a5\)\] bg-\[var\(--color-panel-solid\)\]/);
});

test("admin shell and reused cards use the shared flat panel styling", () => {
  const shell = read("../src/components/admin/AdminPanelBar.tsx");
  const settings = read("../src/components/admin/SettingCard.tsx");
  const dashboard = read("../src/components/admin/DashboardPanels.tsx");
  assert.match(shell, /data-admin-shell[\s\S]*?className="km-admin-topbar/);
  assert.match(shell, /className="km-admin-sidebar/);
  assert.match(settings, /km-panel/);
  assert.match(dashboard, /km-panel/);
  assert.match(css, /\.km-admin-sidebar\s*\{[^}]*background:\s*var\(--km-sidebar\)/s);
});

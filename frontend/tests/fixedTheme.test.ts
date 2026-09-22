import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const themeMenu = JSON.parse(
  readFileSync(new URL("../src/config/menuConfig.json", import.meta.url), "utf8"),
) as { menu: Array<{ path: string; children?: Array<{ path: string }> }> };
const routesSource = readFileSync(
  new URL("../src/routes.ts", import.meta.url),
  "utf8",
);
const themeSettingsSource = readFileSync(
  new URL("../src/pages/admin/theme_managed.tsx", import.meta.url),
  "utf8",
);

function allPaths(items: Array<{ path: string; children?: Array<{ path: string }> }>): string[] {
  return items.flatMap((item) => [item.path, ...(item.children?.map((child) => child.path) ?? [])]);
}

test("keeps only the fixed Glass theme settings page", () => {
  const paths = allPaths(themeMenu.menu);
  assert.ok(paths.includes("/admin/theme_managed"));
  assert.equal(paths.includes("/admin/settings/theme"), false);
  assert.doesNotMatch(routesSource, /theme_raw/);
  assert.doesNotMatch(routesSource, /settings\/theme/);
  assert.match(routesSource, /path:\s*["']theme_managed["']/);
  assert.match(themeSettingsSource, /\/api\/admin\/theme\/settings/);
});

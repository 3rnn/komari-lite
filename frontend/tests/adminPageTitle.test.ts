import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const adminPages = [
  "src/pages/admin/dashboard.tsx",
  "src/pages/admin/index.tsx",
  "src/pages/admin/pingTask.tsx",
  "src/pages/admin/settings/notification.tsx",
  "src/pages/admin/notification/offline.tsx",
  "src/pages/admin/notification/load.tsx",
  "src/pages/admin/notification/traffic_report.tsx",
  "src/pages/admin/notification/ping_loss.tsx",
  "src/pages/admin/notification/general.tsx",
  "src/pages/admin/theme_managed.tsx",
  "src/pages/admin/settings/site.tsx",
  "src/pages/admin/settings/metrics.tsx",
  "src/pages/admin/settings/account-security.tsx",
  "src/pages/admin/settings/general.tsx",
  "src/pages/admin/log.tsx",
];

test("all primary admin pages use the shared page title", () => {
  for (const file of adminPages) {
    const source = readFileSync(path.resolve(file), "utf8");
    assert.match(source, /<AdminPageTitle(?:\s[^>]*)?>/, file);
  }
});

test("the shared page title keeps a compact admin hierarchy", () => {
  const source = readFileSync(
    path.resolve("src/components/admin/AdminPageTitle.tsx"),
    "utf8",
  );
  assert.match(source, /text-xl font-semibold leading-7/);
});

test("server and settings pages share the standard page spacing", () => {
  const serverPage = readFileSync(
    path.resolve("src/pages/admin/index.tsx"),
    "utf8",
  );
  const settingsLayout = readFileSync(
    path.resolve("src/pages/admin/settings/_layout.tsx"),
    "utf8",
  );

  assert.match(serverPage, /direction="column" gap="4" className="p-0 md:p-4"/);
  assert.match(settingsLayout, /direction="column" gap="4" className="p-0 md:p-4"/);
});

test("traffic report selection uses an aligned column and mobile row cards", () => {
  const source = readFileSync(
    path.resolve("src/pages/admin/notification/traffic_report.tsx"),
    "utf8",
  );

  assert.match(source, /TableHead className="w-12 px-3 text-center"/);
  assert.match(source, /admin-responsive-table[^"\n]*admin-selection-table[^"\n]*min-w-\[640px\]/);
  assert.match(source, /data-label=\{t\("common\.server"\)\}/);
  assert.match(source, /common\.deselect_all[\s\S]*common\.select_all/);
  assert.match(source, /admin-single-text-action/);
});

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync("src/pages/admin/settings/metrics.tsx", "utf8");
const css = readFileSync("src/global.css", "utf8");

test("storage settings exposes every tab on narrow phones without a horizontal swipe", () => {
  assert.match(source, /Tabs\.List className="admin-storage-tabs w-full min-w-0 sm:w-max sm:min-w-full"/);
  assert.match(css, /\.admin-storage-tabs \.rt-TabsTrigger \{[^}]*flex: 1 1 0;[^}]*min-width: 0;[^}]*padding-inline: 0\.25rem;[^}]*font-size: 0\.8125rem;/);

  for (const [full, compact] of [
    ["overview", "overview_mobile"],
    ["monitoring_data", "monitoring_mobile"],
    ["migration_maintenance", "migration_mobile"],
  ]) {
    assert.match(source, new RegExp(`aria-label=\\{t\\("settings\\.storage\\.${full}"\\)\\}`));
    assert.match(source, new RegExp(`t\\("settings\\.storage\\.${compact}"\\)`));
    for (const locale of ["en", "zh_CN"]) {
      const messages = JSON.parse(readFileSync(`src/i18n/locales/${locale}.json`, "utf8"));
      assert.ok(messages.settings.storage[full].length > messages.settings.storage[compact].length);
      assert.ok(messages.settings.storage[compact].length <= 8);
    }
  }
});

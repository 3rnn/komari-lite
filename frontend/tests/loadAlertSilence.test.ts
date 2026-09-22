import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { AVAILABLE_LOCALES } from "./localeFixtures.ts";

const pageSource = readFileSync(
  new URL("../src/pages/admin/notification/load.tsx", import.meta.url),
  "utf8",
);
const contextSource = readFileSync(
  new URL("../src/contexts/LoadAlertContext.tsx", import.meta.url),
  "utf8",
);
const globalStyles = readFileSync(
  new URL("../src/global.css", import.meta.url),
  "utf8",
);

for (const locale of AVAILABLE_LOCALES) {
  test(`${locale} includes the complete current-load-alert copy`, () => {
    const messages = JSON.parse(
      readFileSync(
        new URL(`../src/i18n/locales/${locale}.json`, import.meta.url),
        "utf8",
      ),
    ).notification.load;
    for (const key of [
      "configuration",
      "current_alerts",
      "silence_24h",
      "silence_3d",
      "silence_7d",
      "silence_forever",
      "silence_tips",
      "unsilence",
    ]) {
      assert.equal(typeof messages[key], "string", key);
      assert.notEqual(messages[key].trim(), "", key);
    }
  });
}

test("load notifications expose searchable configuration and current-alert tabs", () => {
  assert.match(pageSource, /value="configuration"/);
  assert.match(pageSource, /value="current"/);
  assert.match(pageSource, /normalizedSearch/);
  assert.match(pageSource, /notification_name[\s\S]*client_name[\s\S]*metric[\s\S]*status/);
  assert.match(contextSource, /\/api\/admin\/notification\/load\/current/);
  assert.match(pageSource, /\/api\/admin\/notification\/load\/silence/);
  assert.match(pageSource, /notification\.load\.silence_tips/);
  assert.match(pageSource, /<LoadListToolbar[\s\S]*showAdd[\s\S]*<LoadConfigurationTable/);
  assert.match(pageSource, /<LoadListToolbar[\s\S]*<CurrentLoadAlertsTable/);
  assert.match(pageSource, /"off" \| "24h" \| "3d" \| "7d" \| "forever"/);
});

test("current load alerts show the preferred node IP instead of the UUID", () => {
  assert.match(
    pageSource,
    /node\?\.ipv4\?\.trim\(\) \|\| node\?\.ipv6\?\.trim\(\) \|\| ""/,
  );
  assert.match(
    pageSource,
    /clientAddress \? \([\s\S]*text-xs text-muted-foreground[\s\S]*\{clientAddress\}/,
  );
  assert.doesNotMatch(
    pageSource,
    /text-xs text-muted-foreground">\{alert\.client\}/,
  );
});

test("load notification tabs animate normally and stop only for reduced motion", () => {
  assert.match(pageSource, /className="admin-tab-panel pt-3"/);
  assert.match(globalStyles, /\.admin-tab-panel\[data-state="active"\][^{]*\{[^}]*animation: admin-tab-panel-enter 180ms/);
  assert.match(globalStyles, /html\[data-reduce-motion="true"\][^,]*\.admin-tab-panel/);
});

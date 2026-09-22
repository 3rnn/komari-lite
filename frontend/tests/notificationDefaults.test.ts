import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { AVAILABLE_LOCALES } from "./localeFixtures.ts";

const offlineSource = readFileSync(
  "src/pages/admin/notification/offline.tsx",
  "utf8",
);
const pingLossSource = readFileSync(
  "src/pages/admin/notification/ping_loss.tsx",
  "utf8",
);
const trafficReportSource = readFileSync(
  "src/pages/admin/notification/traffic_report.tsx",
  "utf8",
);
const locales = AVAILABLE_LOCALES.map((locale) =>
  JSON.parse(readFileSync(`src/i18n/locales/${locale}.json`, "utf8")),
);

test("notification pages expose future-server defaults through their edit fields", () => {
  assert.match(offlineSource, /Settings2/);
  assert.match(offlineSource, /\/api\/admin\/notification\/offline\/default/);
  assert.match(offlineSource, /default_config_description/);
  assert.match(pingLossSource, /Settings2/);
  assert.match(pingLossSource, /\/api\/admin\/notification\/ping-loss\/default/);
  assert.match(pingLossSource, /PingLossConfigurationFields/);
  assert.match(pingLossSource, /default_config_description/);
  assert.match(trafficReportSource, /Settings2/);
  assert.match(
    trafficReportSource,
    /\/api\/admin\/notification\/traffic-report\/default/,
  );
  assert.match(trafficReportSource, /TrafficReportEditForm/);
  assert.match(trafficReportSource, /default_config_description/);
});

test("offline default dialog uses the same spacious field rhythm as latency defaults", () => {
  assert.match(offlineSource, /className="mt-4 flex flex-col gap-5"/);
  assert.match(offlineSource, /className="flex items-center justify-between gap-4"/);
  assert.match(offlineSource, /maxWidth="560px"/);
  assert.match(offlineSource, /const formId = React\.useId\(\)/);
});

test("traffic report dialogs share the spacious default configuration rhythm", () => {
  assert.match(trafficReportSource, /className="mt-4 flex flex-col gap-5"/);
  assert.match(
    trafficReportSource,
    /className="flex items-center justify-between gap-4"/,
  );
  assert.match(trafficReportSource, /<fieldset className="grid min-w-0 gap-3/);
  assert.match(trafficReportSource, /maxWidth="560px"/);
});

test("notification default controls are localized in every supported locale", () => {
  for (const locale of locales) {
    for (const section of [
      locale.notification.offline,
      locale.notification.ping_loss,
      locale.notification.traffic_report,
    ]) {
      assert.equal(typeof section.default_config, "string");
      assert.notEqual(section.default_config.trim(), "");
      assert.equal(typeof section.default_config_description, "string");
      assert.notEqual(section.default_config_description.trim(), "");
      assert.equal(typeof section.default_config_enabled, "string");
      assert.notEqual(section.default_config_enabled.trim(), "");
    }
  }
});

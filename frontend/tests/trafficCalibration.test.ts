import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { AVAILABLE_LOCALES } from "./localeFixtures.ts";

const source = readFileSync(path.resolve("src/pages/admin/index.tsx"), "utf8");

test("traffic calibration action sits between billing and delete", () => {
  const billing = source.indexOf("<BillingButton node={node} />");
  const calibration = source.indexOf("<TrafficCalibrationButton node={node} />");
  const deletion = source.indexOf("<DeleteButton node={node} />");
  assert.ok(billing >= 0);
  assert.ok(calibration > billing);
  assert.ok(deletion > calibration);
  assert.match(source, /<TrafficCalibrationButton node=\{node\} \/>/);
});

test("calibration dialog keeps form state independent from node polling", () => {
  const calibrationSource = source.slice(
    source.indexOf("function TrafficCalibrationButton"),
    source.indexOf("function RotateTokenButton"),
  );
  assert.doesNotMatch(calibrationSource, /\[open, node\.uuid\]/);
  assert.match(calibrationSource, /const prepareCalibration = async \(\) =>/);
  assert.match(calibrationSource, /signal: controller\.signal/);
  assert.match(calibrationSource, /setOpen\(true\)/);
  assert.match(calibrationSource, /if \(nextOpen\) void prepareCalibration\(\)/);
  assert.match(source, /target_up: up/);
  assert.match(source, /target_down: down/);
  assert.match(source, /grid grid-cols-1 gap-3 sm:grid-cols-2/);
  assert.match(source, /max-h-\[88vh\] overflow-y-auto/);
  assert.doesNotMatch(calibrationSource, /twoFactor|2fa_code/);
  assert.match(calibrationSource, /<strong className="font-semibold"/);
  assert.match(source, /snapshot\.cycle_start/);
  assert.match(source, /snapshot\.cycle_end/);
  assert.match(source, /timeZone: "Asia\/Shanghai"/);
});

test("all admin languages include the complete traffic calibration copy", () => {
  const required = [
    "title", "description", "currentCycle", "raw", "adjustment", "effective",
    "targetUp", "targetDown", "syncNotice", "history", "save", "saved",
    "invalidValue", "resetDayRequired",
  ];
  for (const locale of AVAILABLE_LOCALES) {
    const translations = JSON.parse(
      readFileSync(path.resolve(`src/i18n/locales/${locale}.json`), "utf8"),
    );
    const calibration = translations.admin.nodeTable.trafficCalibration;
    for (const key of required) assert.ok(calibration[key], `${locale}.${key}`);
  }
});

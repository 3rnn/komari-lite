import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { AVAILABLE_LOCALES } from "./localeFixtures.ts";

const source = readFileSync("src/pages/install.tsx", "utf8");

test("completed installations never expose an actionable installer", () => {
  assert.match(source, /if \(ready === null\)[\s\S]*if \(ready === false\)/);
  assert.match(source, /window\.location\.replace\("\/"\)/);
  assert.match(source, /INSTALL_REDIRECT_DELAY_MS = 2500/);
  assert.match(source, /setReady\(false\)/);
  assert.doesNotMatch(source, /window\.location\.assign\("\/"\), 1200/);
});

test("completed installation messaging is localized", () => {
  for (const locale of AVAILABLE_LOCALES) {
    const messages = JSON.parse(
      readFileSync(`src/i18n/locales/${locale}.json`, "utf8"),
    );
    assert.equal(typeof messages.install.completed_title, "string");
    assert.notEqual(messages.install.completed_title.trim(), "");
    assert.equal(typeof messages.install.completed, "string");
    assert.notEqual(messages.install.completed.trim(), "");
  }
});

test("completed installation reuses the full installation layout", () => {
  assert.match(source, /function InstallLayout/);
  assert.match(source, /<Container size="2">/);
  assert.match(source, /<InstallLayout step=\{INSTALL_STEPS\.length - 1\}>/);
  assert.match(source, /<Card size="3">/);
  assert.match(source, /className="py-10 text-center sm:py-14"/);
  assert.doesNotMatch(source, /max-w-md text-center/);
});

test("install restore uses staged progress and an explicit restart countdown", () => {
  assert.match(source, /phase_uploading/);
  assert.match(source, /phase_processing/);
  assert.match(source, /phase_restarting/);
  assert.match(source, /phase_completed/);
  assert.match(source, /setRestartCountdown\(5\)/);
  assert.match(source, /phase_redirect_countdown/);
  assert.doesNotMatch(source, /setRestoreProgress/);
});

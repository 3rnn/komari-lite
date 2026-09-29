import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

const config = readFileSync(new URL("../src/i18n/config.ts", import.meta.url), "utf8");
const en = JSON.parse(readFileSync(new URL("../src/i18n/locales/en.json", import.meta.url), "utf8"));
const legacy = JSON.parse(readFileSync(new URL("../src/i18n/locales/zh_CN.json", import.meta.url), "utf8"));

test("legacy Chinese locale identifiers resolve to English without appearing as a language choice", () => {
  assert.deepEqual(legacy, en);
  assert.match(config, /"zh-CN":\s*\{\s*translation:\s*zh_CN,\s*\}/);
});

test("English is the default and fallback for a new visitor", () => {
  assert.match(config, /fallbackLng:\s*"en-US"/);
  assert.match(config, /lng:\s*readStoredLanguage\(\)\s*\|\|\s*"en-US"/);
});

test("English-only surfaces do not bundle a redundant language switcher", () => {
  assert.equal(existsSync(new URL("../src/components/Language.tsx", import.meta.url)), false);
  for (const path of [
    "../src/components/Login.tsx",
    "../src/components/GuideHeader.tsx",
    "../src/components/admin/AdminPanelBar.tsx",
  ]) {
    assert.doesNotMatch(readFileSync(new URL(path, import.meta.url), "utf8"), /LanguageSwitch/);
  }
});

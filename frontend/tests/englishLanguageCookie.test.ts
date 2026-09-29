import assert from "node:assert/strict";
import test from "node:test";
import { writeLanguageCookie } from "../src/utils/language.ts";

test("legacy language preferences retain storage compatibility while the document and API use English", () => {
  const originalWindow = globalThis.window;
  const originalDocument = globalThis.document;
  const stored = new Map<string, string>();
  const document = { documentElement: { lang: "" }, cookie: "" };
  const window = { localStorage: { setItem: (key: string, value: string) => stored.set(key, value) } };
  Object.assign(globalThis, { window, document });
  try {
    writeLanguageCookie("zh-CN");
    assert.equal(stored.get("language"), "zh-CN");
    assert.equal(document.documentElement.lang, "en");
    assert.match(document.cookie, /language=en-US;/);
  } finally {
    Object.assign(globalThis, { window: originalWindow, document: originalDocument });
  }
});

import assert from "node:assert/strict";
import test from "node:test";

import {
  normalizeAccountPreferenceColor,
  normalizeAccountPreferenceLanguage,
  saveAccountPreferences,
} from "../src/utils/adminAuth.ts";

test("normalizes supported administrator language and color preferences", () => {
  // 面板只保留英文与简体中文：港台与日文/印尼文偏好统一回落到简体中文或空
  assert.equal(normalizeAccountPreferenceLanguage("zh_HK"), "zh-CN");
  assert.equal(normalizeAccountPreferenceLanguage("zh_TW"), "zh-CN");
  assert.equal(normalizeAccountPreferenceLanguage("ja_JP"), "");
  assert.equal(normalizeAccountPreferenceLanguage("en"), "en-US");
  assert.equal(normalizeAccountPreferenceLanguage("fr-FR"), "");
  assert.equal(normalizeAccountPreferenceColor("jade"), "jade");
  assert.equal(normalizeAccountPreferenceColor("invalid"), "");
});

test("saves preferences through the current administrator account", async () => {
  await saveAccountPreferences(
    { language: "zh-CN", color: "jade" },
    async (input, init) => {
      assert.equal(input, "/api/rpc2");
      assert.equal(init?.method, "POST");
      assert.deepEqual(JSON.parse(String(init?.body)), {
        jsonrpc: "2.0",
        id: 1,
        method: "admin:updateAccountPreferences",
        params: { language: "zh-CN", color: "jade" },
      });
      return new Response(
        JSON.stringify({ jsonrpc: "2.0", id: 1, result: null }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    },
  );
});

test("reports server preference failures without changing local fallback", async () => {
  await assert.rejects(
    () =>
      saveAccountPreferences({ color: "iris" }, async () =>
        new Response(
          JSON.stringify({ error: { message: "preference write failed" } }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    /preference write failed/,
  );
});

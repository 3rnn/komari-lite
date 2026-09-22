import assert from "node:assert/strict";
import test from "node:test";

import { normalizeOptionalServiceUrl } from "../src/utils/serviceUrl.ts";

test("无协议服务地址跟随当前页面的安全协议", () => {
  assert.equal(
    normalizeOptionalServiceUrl("panel.example.com/", "https:"),
    "https://panel.example.com",
  );
  assert.equal(
    normalizeOptionalServiceUrl("panel.example.com/", "http:"),
    "http://panel.example.com",
  );
});

test("显式协议保持用户配置", () => {
  assert.equal(
    normalizeOptionalServiceUrl("http://proxy.example.com/", "https:"),
    "http://proxy.example.com",
  );
  assert.equal(
    normalizeOptionalServiceUrl("https://proxy.example.com/", "http:"),
    "https://proxy.example.com",
  );
});

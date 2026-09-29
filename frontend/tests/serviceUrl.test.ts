import assert from "node:assert/strict";
import test from "node:test";

import { normalizeOptionalServiceUrl } from "../src/utils/serviceUrl.ts";

test("service URLs without a scheme inherit the current page's secure scheme", () => {
  assert.equal(
    normalizeOptionalServiceUrl("panel.example.com/", "https:"),
    "https://panel.example.com",
  );
  assert.equal(
    normalizeOptionalServiceUrl("panel.example.com/", "http:"),
    "http://panel.example.com",
  );
});

test("explicit schemes preserve the configured URL", () => {
  assert.equal(
    normalizeOptionalServiceUrl("http://proxy.example.com/", "https:"),
    "http://proxy.example.com",
  );
  assert.equal(
    normalizeOptionalServiceUrl("https://proxy.example.com/", "http:"),
    "https://proxy.example.com",
  );
});

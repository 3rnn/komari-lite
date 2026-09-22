import assert from "node:assert/strict";
import test from "node:test";

import { getRegionCode } from "../src/utils/regionHelper.ts";

test("getRegionCode returns a stable fallback for missing or invalid regions", () => {
  assert.equal(getRegionCode(undefined), "UN");
  assert.equal(getRegionCode(null), "UN");
  assert.equal(getRegionCode(""), "UN");
  assert.equal(getRegionCode("unknown"), "UN");
});

test("getRegionCode normalizes ISO codes and flag emoji", () => {
  assert.equal(getRegionCode("us"), "US");
  assert.equal(getRegionCode("🇸🇬"), "SG");
});

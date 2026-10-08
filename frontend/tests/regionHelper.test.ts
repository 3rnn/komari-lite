import assert from "node:assert/strict";
import test from "node:test";

import {
  getRegionCode,
  getRegionDisplayName,
  getSupportedRegions,
  isRegionMatch,
} from "../src/utils/regionHelper.ts";

test("legacy region searches still work while both locale display names are English", () => {
  assert.equal(isRegionMatch("🇭🇰", "\u9999\u6e2f"), true);
  assert.equal(isRegionMatch("🇨🇳", "\u4e2d\u534e\u4eba\u6c11\u5171\u548c\u56fd"), true);
  assert.equal(isRegionMatch("🇹🇼", "\u53f0\u7063"), true);
  assert.equal(getRegionDisplayName("🇭🇰", "zh"), "Hong Kong");
  assert.equal(getRegionDisplayName("🇭🇰", "en"), "Hong Kong");
});

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

test("country selector exposes the complete ISO catalog", () => {
  const countries = getSupportedRegions();
  assert.ok(countries.length >= 249);
  assert.ok(countries.includes("🇦🇽"));
  assert.ok(countries.includes("🇧🇶"));
  assert.ok(countries.includes("🇿🇦"));
});

test("United Kingdom aliases always normalize to GB", () => {
  assert.equal(getRegionCode("UK"), "GB");
  assert.equal(getRegionCode(" uk "), "GB");
  assert.equal(getRegionCode("🇬🇧"), "GB");
  assert.equal(getRegionDisplayName("UK", "en"), "United Kingdom");
  assert.equal(isRegionMatch("GB", "UK"), true);
  assert.equal(isRegionMatch("🇬🇧", "britain"), true);
  assert.ok(getSupportedRegions().includes("🇬🇧"));
});

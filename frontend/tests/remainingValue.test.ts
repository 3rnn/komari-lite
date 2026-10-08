import assert from "node:assert/strict";
import test from "node:test";

import {
  buildVpsJsqListing,
  calculateVpsJsqRemainingValue,
  formatCny,
  getVpsJsqDefaultCnyRate,
} from "../src/lib/remainingValue.ts";

test("uses the billing day count directly without calendar-renewal conversion", () => {
  const result = calculateVpsJsqRemainingValue({
    price: "100",
    billingCycle: "92",
    expiredAt: "2026-04-01",
    transactionDate: "2026-01-31",
    cnyPerUnit: "7.2",
    adjustmentMode: "add",
    adjustmentValue: "0",
  });

  assert.equal(result.status, "ready");
  assert.equal(result.remainingDays, 60);
  assert.equal(result.remainingPercentage, 60 / 92 * 100);
  assert.equal(result.baseResidualValueCny, 720 / 92 * 60);
});

test("preserves VPS-JSQ's unrounded arithmetic and target-price behavior", () => {
  const result = calculateVpsJsqRemainingValue({
    price: "100",
    billingCycle: "184",
    expiredAt: "2026-07-15",
    transactionDate: "2026-01-15",
    cnyPerUnit: "7.2",
    adjustmentMode: "target",
    adjustmentValue: "299",
  });

  assert.equal(result.status, "ready");
  assert.equal(result.finalResalePriceCny, 299);
  assert.equal(result.premiumOrDiscountCny, 299 - result.baseResidualValueCny!);
  assert.equal(formatCny(result.baseResidualValueCny!), "¥708.26");
  assert.equal(formatCny(result.finalResalePriceCny!), "¥299.00");
});

test("keeps expired fixed discounts instead of clamping the final price", () => {
  const result = calculateVpsJsqRemainingValue({
    price: "100",
    billingCycle: "365",
    expiredAt: "2026-01-01",
    transactionDate: "2026-01-02",
    cnyPerUnit: "7.2",
    adjustmentMode: "sub",
    adjustmentValue: "10",
  });

  assert.equal(result.status, "ready");
  assert.equal(result.remainingDays, 0);
  assert.equal(result.baseResidualValueCny, 0);
  assert.equal(result.premiumOrDiscountCny, -10);
  assert.equal(result.finalResalePriceCny, -10);
});

test("requires a positive day count and exposes VPS-JSQ-compatible defaults", () => {
  const unsupported = calculateVpsJsqRemainingValue({
    price: "100",
    billingCycle: "-1",
    expiredAt: "2026-02-01",
    transactionDate: "2026-01-01",
    cnyPerUnit: "7.2",
    adjustmentMode: "add",
    adjustmentValue: "0",
  });

  assert.equal(unsupported.status, "unsupported-cycle");
  assert.equal(getVpsJsqDefaultCnyRate("¥"), "1");
  assert.equal(getVpsJsqDefaultCnyRate("$"), "7.25");
  assert.equal(getVpsJsqDefaultCnyRate("₽"), "");
});

test("builds a listing from calculator-only values", () => {
  const result = calculateVpsJsqRemainingValue({
    price: "100",
    billingCycle: "30",
    expiredAt: "2026-02-15",
    transactionDate: "2026-01-30",
    cnyPerUnit: "7.2",
    adjustmentMode: "add",
    adjustmentValue: "10",
  });
  assert.equal(result.status, "ready");

  const listing = buildVpsJsqListing({
    sourceCurrency: "$",
    price: "100",
    billingCycle: "30",
    expiredAt: "2026-02-15",
    transactionDate: "2026-01-30",
    cnyPerUnit: "7.2",
    result,
  });
  assert.match(listing, /Remaining days: 16/);
  assert.match(listing, /Listing price: ¥394\.00/);
});

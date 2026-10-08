import assert from "node:assert/strict";
import test from "node:test";

import {
  calculateVpsJsqRemainingValue,
  VPS_JSQ_ADJUSTMENT_MODES,
} from "../src/lib/remainingValue.ts";

type ReferenceCase = {
  id: string;
  input: {
    price: string;
    billingCycle: string;
    expiredAt: string;
    transactionDate: string;
    cnyPerUnit: string;
    adjustmentMode: "add" | "sub" | "target";
    adjustmentValue: string;
  };
  vpsJsq: {
    remainingDays: number;
    remainingPercentage: number;
    baseResidualValueCny: number;
    premiumOrDiscountCny: number;
    finalResalePriceCny: number;
  };
};

// Captured by executing the public VPS-JSQ inline application source fetched from
// https://jsq.8586898.xyz/ on 2026-10-08. Source SHA-256:
// 31a8d58d5a188801abd6a358f22fb9d1459ffe81305551ff2d500531dc7f6c4e
const referenceCases: ReferenceCase[] = [
  {
    id: "30-day USD fixed premium",
    input: { price: "100", billingCycle: "30", expiredAt: "2026-02-15", transactionDate: "2026-01-30", cnyPerUnit: "7.2", adjustmentMode: "add", adjustmentValue: "10" },
    vpsJsq: { remainingDays: 16, remainingPercentage: 53.333333333333336, baseResidualValueCny: 384, premiumOrDiscountCny: 10, finalResalePriceCny: 394 },
  },
  {
    id: "92-day USD fixed discount near a month boundary",
    input: { price: "100", billingCycle: "92", expiredAt: "2026-04-01", transactionDate: "2026-01-31", cnyPerUnit: "7.2", adjustmentMode: "sub", adjustmentValue: "12.34" },
    vpsJsq: { remainingDays: 60, remainingPercentage: 65.21739130434783, baseResidualValueCny: 469.5652173913044, premiumOrDiscountCny: -12.34, finalResalePriceCny: 457.2252173913044 },
  },
  {
    id: "184-day USD target price",
    input: { price: "100", billingCycle: "184", expiredAt: "2026-07-15", transactionDate: "2026-01-15", cnyPerUnit: "7.2", adjustmentMode: "target", adjustmentValue: "299" },
    vpsJsq: { remainingDays: 181, remainingPercentage: 98.36956521739131, baseResidualValueCny: 708.2608695652174, premiumOrDiscountCny: -409.2608695652174, finalResalePriceCny: 299 },
  },
  {
    id: "365-day USD leap-year boundary",
    input: { price: "100", billingCycle: "365", expiredAt: "2028-02-29", transactionDate: "2027-02-28", cnyPerUnit: "7.2", adjustmentMode: "add", adjustmentValue: "0" },
    vpsJsq: { remainingDays: 366, remainingPercentage: 100.27397260273973, baseResidualValueCny: 721.972602739726, premiumOrDiscountCny: 0, finalResalePriceCny: 721.972602739726 },
  },
  {
    id: "custom 45-day USD cycle",
    input: { price: "10", billingCycle: "45", expiredAt: "2026-02-28", transactionDate: "2026-02-01", cnyPerUnit: "7.2", adjustmentMode: "add", adjustmentValue: "0" },
    vpsJsq: { remainingDays: 27, remainingPercentage: 60, baseResidualValueCny: 43.2, premiumOrDiscountCny: 0, finalResalePriceCny: 43.2 },
  },
  {
    id: "expired service with fixed discount",
    input: { price: "100", billingCycle: "365", expiredAt: "2026-01-01", transactionDate: "2026-01-02", cnyPerUnit: "7.2", adjustmentMode: "sub", adjustmentValue: "10" },
    vpsJsq: { remainingDays: 0, remainingPercentage: 0, baseResidualValueCny: 0, premiumOrDiscountCny: -10, finalResalePriceCny: -10 },
  },
];

const numericFields = [
  "remainingPercentage",
  "baseResidualValueCny",
  "premiumOrDiscountCny",
  "finalResalePriceCny",
] as const;

for (const referenceCase of referenceCases) {
  test(`matches VPS-JSQ: ${referenceCase.id}`, () => {
    const actual = calculateVpsJsqRemainingValue(referenceCase.input);
    assert.equal(actual.status, "ready");
    assert.equal(actual.remainingDays, referenceCase.vpsJsq.remainingDays);
    for (const field of numericFields) {
      const difference = actual[field]! - referenceCase.vpsJsq[field];
      assert.ok(Math.abs(difference) < 1e-9, `${field} difference: ${difference}`);
    }
  });
}

test("uses VPS-JSQ's only supported adjustment modes rather than inventing percentage pricing", () => {
  assert.deepEqual(VPS_JSQ_ADJUSTMENT_MODES, ["add", "sub", "target"]);
  assert.equal(VPS_JSQ_ADJUSTMENT_MODES.includes("percent" as never), false);
});

import assert from "node:assert/strict";
import test from "node:test";

import {
  buildResaleListing,
  calculateRemainingValue,
  getBillingCycleKind,
  advanceBillingCycle,
} from "../src/lib/remainingValue.ts";

test("uses the backend calendar-cycle ranges rather than treating every cycle as days", () => {
  assert.equal(getBillingCycleKind(30), "month");
  assert.equal(getBillingCycleKind(92), "quarter");
  assert.equal(getBillingCycleKind(184), "half-year");
  assert.equal(getBillingCycleKind(365), "year");
  assert.equal(getBillingCycleKind(45), "days");
  assert.equal(getBillingCycleKind(-1), null);

  assert.equal(advanceBillingCycle("2026-01-15", 92), "2026-04-15");
  assert.equal(advanceBillingCycle("2026-01-15", 45), "2026-03-01");
});

test("calculates CNY remaining value from date-only cycle boundaries", () => {
  const result = calculateRemainingValue({
    price: "100",
    billingCycle: "30",
    expiredAt: "2026-02-15",
    cycleStartedAt: "2026-01-15",
    cnyPerUnit: "7.2",
    today: "2026-01-30",
    adjustments: [],
  });

  assert.equal(result.status, "ready");
  assert.equal(result.cycleDays, 31);
  assert.equal(result.remainingDays, 16);
  assert.equal(result.cyclePriceCny, 720);
  assert.equal(result.remainingValueCny, 720 * 16 / 31);
  assert.equal(result.resalePriceCny, 720 * 16 / 31);
});

test("applies fixed and percentage premiums or discounts in listed order", () => {
  const result = calculateRemainingValue({
    price: "100",
    billingCycle: "30",
    expiredAt: "2026-02-15",
    cycleStartedAt: "2026-01-15",
    cnyPerUnit: "1",
    today: "2026-01-30",
    adjustments: [
      { kind: "fixed-cny", value: "10" },
      { kind: "percent", value: "10" },
      { kind: "fixed-cny", value: "-30" },
    ],
  });

  assert.equal(result.status, "ready");
  assert.equal(result.resalePriceCny, ((100 * 16 / 31 + 10) * 1.1) - 30);
  assert.equal(result.resalePriceSource, result.resalePriceCny);
});

test("does not invent a value for one-time, expired, or mismatched cycles", () => {
  const shared = {
    price: "100",
    expiredAt: "2026-02-15",
    cycleStartedAt: "2026-01-15",
    cnyPerUnit: "1",
    adjustments: [],
  };

  assert.equal(calculateRemainingValue({ ...shared, billingCycle: "-1", today: "2026-01-30" }).status, "unsupported-cycle");
  assert.equal(calculateRemainingValue({ ...shared, billingCycle: "30", today: "2026-03-01" }).status, "expired");
  assert.equal(calculateRemainingValue({ ...shared, billingCycle: "92", today: "2026-01-30" }).status, "cycle-mismatch");
});

test("clamps resale value at zero and produces a copy-ready English listing", () => {
  const result = calculateRemainingValue({
    price: "10",
    billingCycle: "30",
    expiredAt: "2026-02-15",
    cycleStartedAt: "2026-01-15",
    cnyPerUnit: "1",
    today: "2026-01-30",
    adjustments: [{ kind: "fixed-cny", value: "-100" }],
  });
  assert.equal(result.status, "ready");
  assert.equal(result.resalePriceCny, 0);

  const listing = buildResaleListing({
    name: "Tokyo VPS",
    sourceCurrency: "$",
    billingCycle: 30,
    expiredAt: "2026-02-15",
    cnyPerUnit: 1,
    result,
  });
  assert.match(listing, /Tokyo VPS/);
  assert.match(listing, /Billing cycle: 30 days/);
  assert.match(listing, /Remaining value: ¥/);
  assert.match(listing, /Resale price: ¥0\.00/);
});

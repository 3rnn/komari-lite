import { resolveBillingCurrencyCode } from "./exchangeRates.ts";

export const VPS_JSQ_ADJUSTMENT_MODES = ["add", "sub", "target"] as const;

export type VpsJsqAdjustmentMode = (typeof VPS_JSQ_ADJUSTMENT_MODES)[number];

export type VpsJsqRemainingValueInput = {
  price: string;
  billingCycle: string;
  expiredAt: string;
  transactionDate: string;
  cnyPerUnit: string;
  adjustmentMode: VpsJsqAdjustmentMode;
  adjustmentValue: string;
};

export type VpsJsqRemainingValueStatus =
  | "ready"
  | "missing-input"
  | "invalid-input"
  | "unsupported-cycle";

export type VpsJsqRemainingValueResult = {
  status: VpsJsqRemainingValueStatus;
  remainingDays?: number;
  remainingPercentage?: number;
  cyclePriceCny?: number;
  baseResidualValueCny?: number;
  remainingValueCny?: number;
  premiumOrDiscountCny?: number;
  finalResalePriceCny?: number;
  resalePriceCny?: number;
};

const MS_PER_DAY = 24 * 60 * 60 * 1000;

function parseDate(value: string): Date | undefined {
  const date = new Date(value.trim());
  return Number.isNaN(date.getTime()) ? undefined : date;
}

function parseRequiredNumber(value: string): number | undefined {
  const normalized = value.trim();
  if (!normalized) return undefined;
  const parsed = Number(normalized);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function parseOptionalNumber(value: string): number | undefined {
  const normalized = value.trim();
  if (!normalized) return 0;
  const parsed = Number(normalized);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function getVpsJsqDefaultCnyRate(currency: string): string {
  return resolveBillingCurrencyCode(currency) === "CNY" ? "1" : "";
}

export function todayVpsJsqDateInput(now = new Date()): string {
  return now.toISOString().split("T")[0];
}

/**
 * Matches the public VPS-JSQ calculator's date and pricing formula exactly.
 * Billing-cycle days are always a numeric denominator; this intentionally does
 * not use the panel's calendar-renewal semantics.
 */
export function calculateVpsJsqRemainingValue(input: VpsJsqRemainingValueInput): VpsJsqRemainingValueResult {
  const price = parseOptionalNumber(input.price);
  const cycle = parseRequiredNumber(input.billingCycle);
  const cnyPerUnit = parseRequiredNumber(input.cnyPerUnit);
  const adjustmentValue = parseOptionalNumber(input.adjustmentValue);
  const transactionDate = parseDate(input.transactionDate);
  const expirationDate = parseDate(input.expiredAt);

  if (price === undefined || cycle === undefined || cnyPerUnit === undefined || !transactionDate || !expirationDate) {
    return { status: "missing-input" };
  }
  if (!Number.isInteger(cycle) || !Number.isFinite(cnyPerUnit) || adjustmentValue === undefined) {
    return { status: "invalid-input" };
  }
  if (cycle <= 0) return { status: "unsupported-cycle" };

  const remainingDays = Math.max(0, Math.ceil((expirationDate.getTime() - transactionDate.getTime()) / MS_PER_DAY));
  const remainingPercentage = (remainingDays / cycle) * 100;
  const cyclePriceCny = price * cnyPerUnit;
  const remainingValueCny = (cyclePriceCny / cycle) * remainingDays;
  const premiumOrDiscountCny = input.adjustmentMode === "add"
    ? adjustmentValue
    : input.adjustmentMode === "sub"
      ? -adjustmentValue
      : adjustmentValue - remainingValueCny;

  const finalResalePriceCny = remainingValueCny + premiumOrDiscountCny;

  return {
    status: "ready",
    remainingDays,
    remainingPercentage,
    cyclePriceCny,
    baseResidualValueCny: remainingValueCny,
    remainingValueCny,
    premiumOrDiscountCny,
    finalResalePriceCny,
    resalePriceCny: finalResalePriceCny,
  };
}

export function formatCny(value: number): string {
  return `¥${value.toFixed(2)}`;
}

function formatSourceAmount(value: string): string {
  return Number(value).toFixed(2);
}

export function buildVpsJsqListing(args: {
  sourceCurrency: string;
  price: string;
  billingCycle: string;
  expiredAt: string;
  transactionDate: string;
  cnyPerUnit: string;
  result: VpsJsqRemainingValueResult;
}): string {
  const { sourceCurrency, price, billingCycle, expiredAt, cnyPerUnit, result } = args;
  if (result.status !== "ready") return "";

  const currencyCode = resolveBillingCurrencyCode(sourceCurrency) ?? sourceCurrency.trim().toUpperCase();
  const adjustment = result.premiumOrDiscountCny!;
  const lines = [
    "VPS Transfer",
    `Renewal: ${sourceCurrency}${formatSourceAmount(price)} ${currencyCode} / ${billingCycle} days`,
    `Expires: ${expiredAt}`,
    `Remaining: ${result.remainingDays} days (${result.remainingPercentage!.toFixed(1)}%)`,
  ];

  if (currencyCode !== "CNY") {
    lines.push(`Exchange rate: 1 ${currencyCode} = ¥${Number(cnyPerUnit).toFixed(2)}`);
  }
  lines.push(`Remaining value: ${formatCny(result.remainingValueCny!)}`);
  if (adjustment > 0) lines.push(`Premium: +¥${adjustment.toFixed(2)}`);
  if (adjustment < 0) lines.push(`Discount: -¥${Math.abs(adjustment).toFixed(2)}`);
  lines.push(`Asking price: ${formatCny(result.resalePriceCny!)}`);

  return lines.join("\n");
}

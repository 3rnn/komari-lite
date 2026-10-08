export type BillingCycleKind =
  | "month"
  | "quarter"
  | "half-year"
  | "year"
  | "two-years"
  | "three-years"
  | "five-years"
  | "days";

export type ResaleAdjustment = {
  kind: "fixed-cny" | "percent";
  value: string;
};

export type RemainingValueInput = {
  price: string;
  billingCycle: string;
  expiredAt: string;
  cycleStartedAt: string;
  cnyPerUnit: string;
  today?: string;
  adjustments: ResaleAdjustment[];
};

export type RemainingValueStatus =
  | "ready"
  | "missing-input"
  | "invalid-input"
  | "unsupported-cycle"
  | "cycle-mismatch"
  | "expired";

export type RemainingValueResult = {
  status: RemainingValueStatus;
  cycleDays?: number;
  remainingDays?: number;
  cyclePriceCny?: number;
  remainingValueCny?: number;
  resalePriceCny?: number;
  resalePriceSource?: number;
};

const DATE_INPUT_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

function parseDateInput(value: string): Date | undefined {
  const match = DATE_INPUT_PATTERN.exec(value.trim());
  if (!match) return undefined;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(0);
  date.setFullYear(year, month - 1, day);
  date.setHours(0, 0, 0, 0);
  if (
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    return undefined;
  }
  return date;
}

function formatDateInput(date: Date): string {
  const year = String(date.getFullYear()).padStart(4, "0");
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function parseFiniteNumber(value: string): number | undefined {
  const normalized = value.trim();
  if (!normalized) return undefined;
  const number = Number(normalized);
  return Number.isFinite(number) ? number : undefined;
}

function calendarDaysBetween(start: Date, end: Date): number {
  const startUtc = Date.UTC(start.getFullYear(), start.getMonth(), start.getDate());
  const endUtc = Date.UTC(end.getFullYear(), end.getMonth(), end.getDate());
  return Math.round((endUtc - startUtc) / 86_400_000);
}

function todayDateInput(): string {
  return formatDateInput(new Date());
}

export function getBillingCycleKind(cycle: number): BillingCycleKind | null {
  if (!Number.isInteger(cycle) || cycle <= 0) return null;
  if (cycle >= 27 && cycle <= 32) return "month";
  if (cycle >= 87 && cycle <= 95) return "quarter";
  if (cycle >= 175 && cycle <= 185) return "half-year";
  if (cycle >= 360 && cycle <= 370) return "year";
  if (cycle >= 720 && cycle <= 750) return "two-years";
  if (cycle >= 1080 && cycle <= 1150) return "three-years";
  if (cycle >= 1800 && cycle <= 1850) return "five-years";
  return "days";
}

export function advanceBillingCycle(startInput: string, cycle: number): string | undefined {
  const start = parseDateInput(startInput);
  const kind = getBillingCycleKind(cycle);
  if (!start || !kind) return undefined;

  const end = new Date(start.getTime());
  switch (kind) {
    case "month":
      end.setMonth(end.getMonth() + 1);
      break;
    case "quarter":
      end.setMonth(end.getMonth() + 3);
      break;
    case "half-year":
      end.setMonth(end.getMonth() + 6);
      break;
    case "year":
      end.setFullYear(end.getFullYear() + 1);
      break;
    case "two-years":
      end.setFullYear(end.getFullYear() + 2);
      break;
    case "three-years":
      end.setFullYear(end.getFullYear() + 3);
      break;
    case "five-years":
      end.setFullYear(end.getFullYear() + 5);
      break;
    case "days":
      end.setDate(end.getDate() + cycle);
      break;
  }
  return formatDateInput(end);
}

export function calculateRemainingValue(input: RemainingValueInput): RemainingValueResult {
  const price = parseFiniteNumber(input.price);
  const cycle = parseFiniteNumber(input.billingCycle);
  const cnyPerUnit = parseFiniteNumber(input.cnyPerUnit);
  const start = parseDateInput(input.cycleStartedAt);
  const due = parseDateInput(input.expiredAt);
  const today = parseDateInput(input.today ?? todayDateInput());

  if (
    price === undefined ||
    cycle === undefined ||
    cnyPerUnit === undefined ||
    !start ||
    !due ||
    !today
  ) {
    return { status: "missing-input" };
  }
  if (price < 0 || cnyPerUnit <= 0 || !Number.isInteger(cycle)) {
    return { status: "invalid-input" };
  }
  const kind = getBillingCycleKind(cycle);
  if (!kind) return { status: "unsupported-cycle" };
  if (advanceBillingCycle(input.cycleStartedAt, cycle) !== input.expiredAt.trim()) {
    return { status: "cycle-mismatch" };
  }

  const cycleDays = calendarDaysBetween(start, due);
  if (cycleDays <= 0) return { status: "invalid-input" };
  const remainingDays = Math.max(0, Math.min(cycleDays, calendarDaysBetween(today, due)));
  if (remainingDays === 0) return { status: "expired", cycleDays, remainingDays };

  const cyclePriceCny = price * cnyPerUnit;
  const remainingValueCny = cyclePriceCny * remainingDays / cycleDays;
  let resalePriceCny = remainingValueCny;
  for (const adjustment of input.adjustments) {
    const value = parseFiniteNumber(adjustment.value);
    if (value === undefined) continue;
    if (adjustment.kind === "fixed-cny") resalePriceCny += value;
    else resalePriceCny *= 1 + value / 100;
  }
  resalePriceCny = Math.max(0, resalePriceCny);

  return {
    status: "ready",
    cycleDays,
    remainingDays,
    cyclePriceCny,
    remainingValueCny,
    resalePriceCny,
    resalePriceSource: resalePriceCny / cnyPerUnit,
  };
}

export function formatCny(value: number): string {
  return `¥${value.toFixed(2)}`;
}

export function buildResaleListing({
  name,
  sourceCurrency,
  billingCycle,
  expiredAt,
  cnyPerUnit,
  result,
}: {
  name: string;
  sourceCurrency: string;
  billingCycle: number;
  expiredAt: string;
  cnyPerUnit: number;
  result: RemainingValueResult;
}): string {
  if (result.status !== "ready") return "";
  return [
    name,
    `Billing cycle: ${billingCycle} days`,
    `Expires: ${expiredAt}`,
    `Remaining: ${result.remainingDays}/${result.cycleDays} days`,
    `Remaining value: ${formatCny(result.remainingValueCny!)} (rate: 1 ${sourceCurrency} = ¥${cnyPerUnit.toFixed(4)})`,
    `Resale price: ${formatCny(result.resalePriceCny!)}`,
  ].join("\n");
}

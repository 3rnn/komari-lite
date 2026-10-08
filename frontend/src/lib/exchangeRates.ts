export const EXCHANGE_RATE_API_BASE_URL = "https://open.er-api.com/v6/latest";
export const EXCHANGE_RATE_CACHE_PREFIX = "komari:exchange-rate:cny:";

const BILLING_CURRENCY_CODES: Record<string, string> = {
  "¥": "CNY",
  "$": "USD",
  "€": "EUR",
  "£": "GBP",
  "₽": "RUB",
  "₣": "CHF",
  "₹": "INR",
  "₫": "VND",
  "฿": "THB",
  "C$": "CAD",
};

export type ExchangeRateStorage = Pick<Storage, "getItem" | "setItem">;

type CachedExchangeRate = {
  currencyCode: string;
  cnyPerUnit: string;
  updatedAt: number;
  nextUpdateAt: number;
};

export type ExchangeRateResult =
  | {
    kind: "fixed";
    currencyCode: "CNY";
    cnyPerUnit: "1";
    source: "CNY";
  }
  | {
    kind: "live" | "cached" | "stale-cache";
    currencyCode: string;
    cnyPerUnit: string;
    source: "ExchangeRate-API";
    updatedAt: number;
    nextUpdateAt: number;
  }
  | {
    kind: "error";
    currencyCode?: string;
    reason: "unsupported-currency" | "timeout" | "rate-limited" | "invalid-response" | "unavailable";
  };

export type FetchCnyExchangeRateOptions = {
  fetchImpl?: typeof fetch;
  storage?: ExchangeRateStorage;
  now?: () => number;
  timeoutMs?: number;
  force?: boolean;
};

export function resolveBillingCurrencyCode(currency: string): string | undefined {
  const normalized = currency.trim();
  if (BILLING_CURRENCY_CODES[normalized]) return BILLING_CURRENCY_CODES[normalized];
  return /^[A-Za-z]{3}$/.test(normalized) ? normalized.toUpperCase() : undefined;
}

function browserStorage(): ExchangeRateStorage | undefined {
  try {
    return typeof window === "undefined" ? undefined : window.localStorage;
  } catch {
    return undefined;
  }
}

function cacheKey(currencyCode: string): string {
  return `${EXCHANGE_RATE_CACHE_PREFIX}${currencyCode}`;
}

function readCachedRate(storage: ExchangeRateStorage | undefined, currencyCode: string): CachedExchangeRate | undefined {
  if (!storage) return undefined;
  try {
    const parsed: unknown = JSON.parse(storage.getItem(cacheKey(currencyCode)) ?? "null");
    if (!parsed || typeof parsed !== "object") return undefined;
    const cache = parsed as Partial<CachedExchangeRate>;
    if (
      cache.currencyCode !== currencyCode
      || typeof cache.cnyPerUnit !== "string"
      || !isPositiveFiniteRate(cache.cnyPerUnit)
      || !Number.isFinite(cache.updatedAt)
      || !Number.isFinite(cache.nextUpdateAt)
    ) return undefined;
    return cache as CachedExchangeRate;
  } catch {
    return undefined;
  }
}

function writeCachedRate(storage: ExchangeRateStorage | undefined, cache: CachedExchangeRate): void {
  if (!storage) return;
  try {
    storage.setItem(cacheKey(cache.currencyCode), JSON.stringify(cache));
  } catch {
    // Storage is optional. A successful provider response remains usable for this session.
  }
}

function isPositiveFiniteRate(value: unknown): value is string | number {
  const parsed = typeof value === "number" ? value : Number(value);
  return Number.isFinite(parsed) && parsed > 0;
}

function rateText(rate: number): string {
  return String(rate);
}

function failureReason(error: unknown): Extract<ExchangeRateResult, { kind: "error" }>['reason'] {
  if (error instanceof DOMException && error.name === "AbortError") return "timeout";
  return "unavailable";
}

export async function fetchCnyExchangeRate(currency: string, options: FetchCnyExchangeRateOptions = {}): Promise<ExchangeRateResult> {
  const currencyCode = resolveBillingCurrencyCode(currency);
  if (!currencyCode) return { kind: "error", reason: "unsupported-currency" };
  if (currencyCode === "CNY") return { kind: "fixed", currencyCode: "CNY", cnyPerUnit: "1", source: "CNY" };

  const now = options.now ?? Date.now;
  const storage = options.storage ?? browserStorage();
  const cached = readCachedRate(storage, currencyCode);
  if (!options.force && cached && cached.nextUpdateAt > now()) {
    return { kind: "cached", ...cached, source: "ExchangeRate-API" };
  }

  const fetchImpl = options.fetchImpl ?? globalThis.fetch;
  if (typeof fetchImpl !== "function") {
    return cached
      ? { kind: "stale-cache", ...cached, source: "ExchangeRate-API" }
      : { kind: "error", currencyCode, reason: "unavailable" };
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), options.timeoutMs ?? 8_000);
  try {
    const response = await fetchImpl(`${EXCHANGE_RATE_API_BASE_URL}/${currencyCode}`, {
      signal: controller.signal,
      headers: { Accept: "application/json" },
    });
    if (response.status === 429) {
      return cached
        ? { kind: "stale-cache", ...cached, source: "ExchangeRate-API" }
        : { kind: "error", currencyCode, reason: "rate-limited" };
    }
    if (!response.ok) {
      return cached
        ? { kind: "stale-cache", ...cached, source: "ExchangeRate-API" }
        : { kind: "error", currencyCode, reason: "unavailable" };
    }

    const body: unknown = await response.json();
    const data = body as {
      result?: unknown;
      base_code?: unknown;
      rates?: { CNY?: unknown };
      time_last_update_unix?: unknown;
      time_next_update_unix?: unknown;
    };
    const providerRate = data.rates?.CNY;
    const updatedAt = Number(data.time_last_update_unix) * 1_000;
    const nextUpdateAt = Number(data.time_next_update_unix) * 1_000;
    if (
      data.result !== "success"
      || data.base_code !== currencyCode
      || !isPositiveFiniteRate(providerRate)
      || !Number.isFinite(updatedAt)
      || !Number.isFinite(nextUpdateAt)
    ) {
      return cached
        ? { kind: "stale-cache", ...cached, source: "ExchangeRate-API" }
        : { kind: "error", currencyCode, reason: "invalid-response" };
    }

    const fresh: CachedExchangeRate = {
      currencyCode,
      cnyPerUnit: rateText(Number(providerRate)),
      updatedAt,
      nextUpdateAt,
    };
    writeCachedRate(storage, fresh);
    return { kind: "live", ...fresh, source: "ExchangeRate-API" };
  } catch (error) {
    return cached
      ? { kind: "stale-cache", ...cached, source: "ExchangeRate-API" }
      : { kind: "error", currencyCode, reason: failureReason(error) };
  } finally {
    clearTimeout(timeout);
  }
}

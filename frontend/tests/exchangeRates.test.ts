import assert from "node:assert/strict";
import test from "node:test";

import {
  EXCHANGE_RATE_CACHE_PREFIX,
  fetchCnyExchangeRate,
  resolveBillingCurrencyCode,
  type ExchangeRateStorage,
} from "../src/lib/exchangeRates.ts";

function memoryStorage(): ExchangeRateStorage & { values: Map<string, string> } {
  const values = new Map<string, string>();
  return {
    values,
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
  };
}

function providerResponse(base: string, rate: number, now = 1_800_000_000): Response {
  return new Response(JSON.stringify({
    result: "success",
    base_code: base,
    rates: { CNY: rate },
    time_last_update_unix: now - 60,
    time_next_update_unix: now + 86_400,
  }), { status: 200, headers: { "content-type": "application/json" } });
}

test("maps every built-in Billing currency to its ExchangeRate-API ISO code", () => {
  assert.equal(resolveBillingCurrencyCode("¥"), "CNY");
  assert.equal(resolveBillingCurrencyCode("$"), "USD");
  assert.equal(resolveBillingCurrencyCode("€"), "EUR");
  assert.equal(resolveBillingCurrencyCode("£"), "GBP");
  assert.equal(resolveBillingCurrencyCode("₽"), "RUB");
  assert.equal(resolveBillingCurrencyCode("₣"), "CHF");
  assert.equal(resolveBillingCurrencyCode("₹"), "INR");
  assert.equal(resolveBillingCurrencyCode("₫"), "VND");
  assert.equal(resolveBillingCurrencyCode("฿"), "THB");
  assert.equal(resolveBillingCurrencyCode("C$"), "CAD");
  assert.equal(resolveBillingCurrencyCode("jpy"), "JPY");
  assert.equal(resolveBillingCurrencyCode("unknown"), undefined);
});

test("retrieves USD-to-CNY, saves the provider timestamps, and reuses a fresh cache", async () => {
  const storage = memoryStorage();
  let calls = 0;
  const fetchImpl: typeof fetch = async (url) => {
    calls += 1;
    assert.equal(url, "https://open.er-api.com/v6/latest/USD");
    return providerResponse("USD", 7.25);
  };

  const first = await fetchCnyExchangeRate("$", { fetchImpl, storage, now: () => 1_800_000_000_000 });
  assert.deepEqual(first, {
    kind: "live",
    currencyCode: "USD",
    cnyPerUnit: "7.25",
    source: "ExchangeRate-API",
    updatedAt: 1_799_999_940_000,
    nextUpdateAt: 1_800_086_400_000,
  });
  assert.equal(storage.values.has(`${EXCHANGE_RATE_CACHE_PREFIX}USD`), true);

  const cached = await fetchCnyExchangeRate("USD", { fetchImpl, storage, now: () => 1_800_000_000_000 });
  assert.equal(cached.kind, "cached");
  assert.equal(cached.cnyPerUnit, "7.25");
  assert.equal(calls, 1);
});

test("returns one for CNY without a network request", async () => {
  const result = await fetchCnyExchangeRate("¥", {
    fetchImpl: async () => { throw new Error("CNY must not fetch"); },
    storage: memoryStorage(),
  });
  assert.deepEqual(result, {
    kind: "fixed",
    currencyCode: "CNY",
    cnyPerUnit: "1",
    source: "CNY",
  });
});

test("supports other provider currencies and explicit refresh", async () => {
  const storage = memoryStorage();
  let calls = 0;
  const fetchImpl: typeof fetch = async (url) => {
    calls += 1;
    assert.equal(url, "https://open.er-api.com/v6/latest/EUR");
    return providerResponse("EUR", calls === 1 ? 7.8 : 7.81);
  };

  const first = await fetchCnyExchangeRate("€", { fetchImpl, storage, now: () => 1_800_000_000_000 });
  const refreshed = await fetchCnyExchangeRate("EUR", { fetchImpl, storage, force: true, now: () => 1_800_000_000_000 });
  assert.equal(first.cnyPerUnit, "7.8");
  assert.equal(refreshed.kind, "live");
  assert.equal(refreshed.cnyPerUnit, "7.81");
  assert.equal(calls, 2);
});

test("returns a clearly labeled cached rate after timeout, HTTP 429, or invalid provider data", async () => {
  const storage = memoryStorage();
  storage.setItem(`${EXCHANGE_RATE_CACHE_PREFIX}USD`, JSON.stringify({
    currencyCode: "USD",
    cnyPerUnit: "7.25",
    updatedAt: 1_700_000_000_000,
    nextUpdateAt: 1_700_086_400_000,
  }));

  for (const response of [
    async () => { throw new DOMException("Timed out", "AbortError"); },
    async () => new Response("slow down", { status: 429 }),
    async () => new Response(JSON.stringify({ result: "success", rates: { CNY: 0 } }), { status: 200 }),
  ]) {
    const result = await fetchCnyExchangeRate("USD", { fetchImpl: response as typeof fetch, storage, force: true, now: () => 1_800_000_000_000 });
    assert.equal(result.kind, "stale-cache");
    assert.equal(result.cnyPerUnit, "7.25");
  }
});

test("reports a manual-entry error instead of inventing a fallback rate", async () => {
  const result = await fetchCnyExchangeRate("USD", {
    fetchImpl: async () => new Response("rate limited", { status: 429 }),
    storage: memoryStorage(),
    force: true,
  });
  assert.deepEqual(result, { kind: "error", currencyCode: "USD", reason: "rate-limited" });
});

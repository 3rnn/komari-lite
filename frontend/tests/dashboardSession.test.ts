import assert from "node:assert/strict";
import test from "node:test";

import {
  clearDashboardSession,
  isDashboardUnauthorized,
  purgeLegacyDashboardResponses,
  readDashboardSession,
  writeDashboardSession,
  type DashboardSessionStorage,
} from "../src/utils/dashboardSession.ts";
import { readFileSync } from "node:fs";

function memoryStorage(): DashboardSessionStorage {
  const values = new Map<string, string>();
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key),
    key: (index) => [...values.keys()][index] ?? null,
    get length() { return values.size; },
  };
}

test("dashboard settings snapshots are isolated by administrator and request shape", () => {
  const storage = memoryStorage();
  writeDashboardSession("settings", "admin-a", "latency:5", { value: 1 }, { storage, now: 1000 });

  assert.deepEqual(
    readDashboardSession("settings", "admin-a", "latency:5", { storage, now: 1500 }),
    { value: 1 },
  );
  assert.equal(readDashboardSession("settings", "admin-b", "latency:5", { storage, now: 1500 }), null);
  assert.equal(readDashboardSession("settings", "admin-a", "latency:10", { storage, now: 1500 }), null);
});

test("dashboard session snapshots expire without blocking rendering", () => {
  const storage = memoryStorage();
  writeDashboardSession("settings", "admin-a", "active", { layout: "overview" }, { storage, now: 1000 });

  assert.equal(readDashboardSession("settings", "admin-a", "active", {
    storage,
    now: 3001,
    maxAgeMs: 2000,
  }), null);
});

test("dashboard response payloads never persist in sessionStorage, including legacy snapshots", () => {
  const storage = memoryStorage();
  const key = "komari:admin-dashboard:v1:admin-a:summary";
  storage.setItem(key, JSON.stringify({ key: "servers:5", savedAt: 1000, data: { private: true } }));
  assert.equal(readDashboardSession("summary", "admin-a", "servers:5", { storage, now: 1500 }), null);
  assert.equal(storage.getItem(key), null);
  writeDashboardSession("summary", "admin-a", "servers:5", { private: true }, { storage, now: 1500 });
  writeDashboardSession("charts", "admin-a", "traffic:5", { private: true }, { storage, now: 1500 });
  assert.equal(storage.getItem(key), null);
  assert.equal(storage.getItem("komari:admin-dashboard:v1:admin-a:charts"), null);
});

test("logout removes every administrator dashboard entry without clearing unrelated session data", () => {
  const storage = memoryStorage();
  storage.setItem("komari:admin-dashboard:v1:admin-a:summary", "legacy-summary");
  storage.setItem("komari:admin-dashboard:v1:admin-b:charts", "legacy-charts");
  writeDashboardSession("settings", "admin-a", "active", { theme: "dark" }, { storage, now: 1000 });
  writeDashboardSession("view", "admin-b", "overview", { scrollTop: 40 }, { storage, now: 1000 });
  storage.setItem("unrelated", "keep");
  clearDashboardSession({ storage });
  assert.equal(storage.getItem("komari:admin-dashboard:v1:admin-a:summary"), null);
  assert.equal(storage.getItem("komari:admin-dashboard:v1:admin-b:charts"), null);
  assert.equal(readDashboardSession("settings", "admin-a", "active", { storage, now: 1500 }), null);
  assert.equal(readDashboardSession("view", "admin-b", "overview", { storage, now: 1500 }), null);
  assert.equal(storage.getItem("unrelated"), "keep");
});

test("initial cleanup removes unread legacy response payloads across administrators", () => {
  const storage = memoryStorage();
  storage.setItem("komari:admin-dashboard:v1:admin-a:summary", "old-summary");
  storage.setItem("komari:admin-dashboard:v1:admin-b:charts", "old-charts");
  storage.setItem("komari:admin-dashboard:v1:admin-a:settings", "preferences");
  storage.setItem("other", "keep");
  purgeLegacyDashboardResponses({ storage });
  assert.equal(storage.getItem("komari:admin-dashboard:v1:admin-a:summary"), null);
  assert.equal(storage.getItem("komari:admin-dashboard:v1:admin-b:charts"), null);
  assert.equal(storage.getItem("komari:admin-dashboard:v1:admin-a:settings"), "preferences");
  assert.equal(storage.getItem("other"), "keep");
});

test("blocked sessionStorage enumeration cannot interrupt logout", () => {
  const storage = {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
    key: () => null,
    get length(): number { throw new Error("storage disabled"); },
  };
  assert.doesNotThrow(() => clearDashboardSession({ storage }));
});

test("a restricted removeItem is attempted once and cannot escape a session read", () => {
  for (const value of ["invalid json", JSON.stringify({ key: "wrong", savedAt: 1, data: {} })]) {
    let removals = 0;
    const storage: DashboardSessionStorage = {
      getItem: () => value,
      setItem: () => {},
      removeItem: () => { removals++; throw new Error("storage disabled"); },
    };
    assert.equal(readDashboardSession("settings", "admin-a", "active", { storage }), null);
    assert.equal(removals, 1);
  }
});

test("revocation recognition uses HTTP status, not mutable response text", () => {
  assert.equal(isDashboardUnauthorized(Object.assign(new Error("Translated response"), { status: 401 })), true);
  assert.equal(isDashboardUnauthorized(Object.assign(new Error("Unauthorized"), { status: 503 })), false);
  assert.equal(isDashboardUnauthorized(new Error("HTTP 401")), false);
});

test("dashboard revocation drops both displayed payloads and rejects late responses", () => {
  const source = readFileSync("src/pages/admin/dashboard.tsx", "utf8");
  assert.match(source, /if \(isDashboardUnauthorized\(reason\)\)/);
  assert.match(source, /subscribeAdminRevocation\(revokeDashboard\)/);
  const apiSource = readFileSync("src/utils/dashboardApi.ts", "utf8");
  assert.match(apiSource, /registerAdminCacheClearer\(clearDashboardDataSnapshots\)/);
  assert.match(source, /setData\(null\)/);
  assert.match(source, /setCharts\(null\)/);
  assert.match(source, /if \(revokedRef\.current\) return/);
  assert.match(source, /\{revoked \? \(/);
});

test("dashboard view snapshots keep only the scroll position and module anchor", () => {
  const storage = memoryStorage();
  const view = { scrollTop: 820, moduleId: "alerts", moduleOffset: 148 };
  writeDashboardSession("view", "admin-a", "overview", view, { storage, now: 1000 });
  assert.deepEqual(
    readDashboardSession("view", "admin-a", "overview", { storage, now: 1500 }),
    view,
  );
});

test("dashboard restores the clicked module after layout stabilization without scroll listeners", () => {
  const source = readFileSync("src/pages/admin/dashboard.tsx", "utf8");
  assert.match(source, /data-dashboard-module=/);
  assert.match(source, /moduleOffset: moduleElement\.getBoundingClientRect\(\)\.top/);
  assert.match(source, /window\.requestAnimationFrame\(restore\)/);
  assert.match(source, /window\.addEventListener\("pagehide", saveBeforePageHide\)/);
  assert.match(source, /document\.addEventListener\("visibilitychange", saveWhenHidden\)/);
  assert.doesNotMatch(source, /addEventListener\("scroll"/);
});

test("logout prevents pagehide from recreating dashboard entries", (t) => {
  const storage = memoryStorage();
  const previousWindow = globalThis.window;
  globalThis.window = { sessionStorage: storage } as Window & typeof globalThis;
  t.after(() => { globalThis.window = previousWindow; });
  writeDashboardSession("view", "admin-a", "overview", { scrollTop: 10 });
  clearDashboardSession();
  writeDashboardSession("view", "admin-a", "overview", { scrollTop: 20 });
  assert.equal(storage.getItem("komari:admin-dashboard:v1:admin-a:view"), null);
});

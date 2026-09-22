import assert from "node:assert/strict";
import test from "node:test";

import {
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
  };
}

test("dashboard session snapshots are isolated by administrator and request shape", () => {
  const storage = memoryStorage();
  writeDashboardSession("charts", "admin-a", "latency:5", { value: 1 }, { storage, now: 1000 });

  assert.deepEqual(
    readDashboardSession("charts", "admin-a", "latency:5", { storage, now: 1500 }),
    { value: 1 },
  );
  assert.equal(readDashboardSession("charts", "admin-b", "latency:5", { storage, now: 1500 }), null);
  assert.equal(readDashboardSession("charts", "admin-a", "latency:10", { storage, now: 1500 }), null);
});

test("dashboard session snapshots expire without blocking rendering", () => {
  const storage = memoryStorage();
  writeDashboardSession("summary", "admin-a", "servers:5", { online: 3 }, { storage, now: 1000 });

  assert.equal(readDashboardSession("summary", "admin-a", "servers:5", {
    storage,
    now: 3001,
    maxAgeMs: 2000,
  }), null);
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

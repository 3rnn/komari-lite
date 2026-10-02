import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { register } from "node:module";
import test from "node:test";

const sourceRoot = new URL("../src/", import.meta.url).href;
register("data:text/javascript," + encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    return nextResolve(specifier.startsWith("@/")
      ? ${JSON.stringify(sourceRoot)} + specifier.slice(2) + ".ts"
      : specifier, context);
  }
`));
const api = await import("../src/utils/dashboardApi.ts");

const summaryKey = "servers:5";
const chartsKey = "traffic:5";

test("dashboard payloads remain cached in memory but expire without a storage read", async (t) => {
  let now = 1000;
  t.mock.method(Date, "now", () => now);
  t.mock.method(globalThis, "fetch", async () => new Response(JSON.stringify({ private: true }), { status: 200 }));
  await api.requestDashboard(["servers"], 5, "memory-a");
  await api.requestDashboardCharts(["traffic"], 5, "memory-a");
  assert.deepEqual(api.getDashboardSnapshot(summaryKey, "memory-a"), { private: true });
  assert.deepEqual(api.getDashboardChartsSnapshot(chartsKey, "memory-a"), { private: true });
  now += 10 * 60 * 1000 + 1;
  assert.equal(api.getDashboardSnapshot(summaryKey, "memory-a"), null);
  assert.equal(api.getDashboardChartsSnapshot(chartsKey, "memory-a"), null);
});

test("logout invalidates cached and in-flight dashboard responses", async (t) => {
  let complete!: (response: Response) => void;
  t.mock.method(globalThis, "fetch", () => new Promise<Response>((resolve) => { complete = resolve; }));
  const inFlight = api.requestDashboard(["servers"], 5, "logout-a");
  api.clearDashboardSnapshots();
  complete(new Response(JSON.stringify({ private: true }), { status: 200 }));
  await assert.rejects(inFlight, (error: unknown) => error instanceof Error && error.name === "AbortError");
  assert.equal(api.getDashboardSnapshot(summaryKey, "logout-a"), null);
});

test("logout rejects an in-flight charts body, without caching it", async (t) => {
  let complete!: (value: unknown) => void;
  t.mock.method(globalThis, "fetch", async () => ({
    ok: true, status: 200, json: () => new Promise((resolve) => { complete = resolve; }),
  } as Response));
  const pending = api.requestDashboardCharts(["traffic"], 5, "logout-charts");
  await new Promise((resolve) => setTimeout(resolve, 0));
  api.clearDashboardSnapshots();
  complete({ private: true });
  await assert.rejects(pending, (error: unknown) => error instanceof Error && error.name === "AbortError");
  assert.equal(api.getDashboardChartsSnapshot(chartsKey, "logout-charts"), null);
});

test("dashboard endpoints preserve 401 status even when the response message changes", async (t) => {
  t.mock.method(globalThis, "fetch", async () => new Response(JSON.stringify({ message: "Session expired" }), { status: 401 }));
  await assert.rejects(api.requestDashboard(["servers"], 5, "revoked-a"), (reason: unknown) =>
    reason instanceof Error && reason.message === "Session expired" && (reason as Error & { status?: number }).status === 401);
  await assert.rejects(api.requestDashboardCharts(["traffic"], 5, "revoked-a"), (reason: unknown) =>
    reason instanceof Error && reason.message === "Session expired" && (reason as Error & { status?: number }).status === 401);
});

test("admin logout clears mounted state then ends the session before navigating", () => {
  const source = readFileSync("src/components/admin/AdminPanelBar.tsx", "utf8");
  const revocation = readFileSync("src/utils/adminRevocation.ts", "utf8");
  assert.match(source, /logoutAdminSession\(\(\) =>/);
  assert.match(revocation, /revokeAdminSession\(\);[\s\S]*await fetch\("\/api\/logout"/);
  assert.match(revocation, /credentials: "same-origin"/);
});

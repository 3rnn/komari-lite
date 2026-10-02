import assert from "node:assert/strict";
import test from "node:test";
import { register } from "node:module";
import { readFileSync } from "node:fs";

const sourceRoot = new URL("../src/", import.meta.url).href;
register("data:text/javascript," + encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    return nextResolve(specifier.startsWith("@/")
      ? ${JSON.stringify(sourceRoot)} + specifier.slice(2) + ".ts"
      : specifier, context);
  }
`));
const alerts = await import("../src/utils/adminAlertFilters.ts");
const dashboard = await import("../src/utils/dashboardApi.ts");
const revocation = await import("../src/utils/adminRevocation.ts");

const result = (name: string) => new Response(JSON.stringify({ items: [{ node_uuid: name }] }), { status: 200 });

test("dashboard cache invalidation removes alert snapshots before revoked-session navigation", async (t) => {
  t.mock.method(globalThis, "fetch", async () => result("old-node"));
  await alerts.prefetchDashboardAlertItems("offline", "revoked-navigation");
  assert.equal(alerts.getDashboardAlertItemsSnapshot("offline", "revoked-navigation")?.items[0]?.node_uuid, "old-node");
  dashboard.clearDashboardSnapshots();
  assert.equal(alerts.getDashboardAlertItemsSnapshot("offline", "revoked-navigation"), null);
});

test("an in-flight prefetch cannot repopulate or deliver alert items after invalidation", async (t) => {
  let finish!: (response: Response) => void;
  t.mock.method(globalThis, "fetch", () => new Promise<Response>((resolve) => { finish = resolve; }));
  const pending = alerts.prefetchDashboardAlertItems("resource", "revoked-in-flight");
  dashboard.clearDashboardSnapshots();
  finish(result("old-node"));
  await assert.rejects(pending, (error: unknown) => error instanceof Error && error.name === "AbortError");
  assert.equal(alerts.getDashboardAlertItemsSnapshot("resource", "revoked-in-flight"), null);
});

test("an in-flight route request cannot deliver old alert items after logout", async (t) => {
  let finish!: (response: Response) => void;
  t.mock.method(globalThis, "fetch", () => new Promise<Response>((resolve) => { finish = resolve; }));
  const pending = alerts.requestDashboardAlertItems("offline", undefined, "revoked-route");
  dashboard.clearDashboardSnapshots();
  finish(result("old-route-node"));
  await assert.rejects(pending, (error: unknown) => error instanceof Error && error.name === "AbortError");
  assert.equal(alerts.getDashboardAlertItemsSnapshot("offline", "revoked-route"), null);
});

test("late old requests cannot overwrite a new session's alert cache", async (t) => {
  let finishOld!: (response: Response) => void;
  let calls = 0;
  t.mock.method(globalThis, "fetch", () => {
    calls++;
    return calls === 1 ? new Promise<Response>((resolve) => { finishOld = resolve; }) : Promise.resolve(result("new-node"));
  });
  const oldRequest = alerts.prefetchDashboardAlertItems("traffic", "same-account");
  dashboard.clearDashboardSnapshots();
  const newRequest = alerts.prefetchDashboardAlertItems("traffic", "same-account");
  assert.equal((await newRequest).items[0]?.node_uuid, "new-node");
  finishOld(result("old-node"));
  await assert.rejects(oldRequest, (error: unknown) => error instanceof Error && error.name === "AbortError");
  assert.equal(alerts.getDashboardAlertItemsSnapshot("traffic", "same-account")?.items[0]?.node_uuid, "new-node");
  assert.equal(calls, 2);
});

test("a cached alert promise cannot deliver private items to its route after revocation", async (t) => {
  t.mock.method(globalThis, "fetch", async () => result("private-old"));
  await alerts.prefetchDashboardAlertItems("offline", "cached-route");
  const generation = revocation.getAdminRevocationGeneration();
  const pending = alerts.requestDashboardAlertItems("offline", undefined, "cached-route")
    .then((response) => alerts.currentRouteAlertItems(response, generation, false));
  revocation.revokeAdminSession();
  assert.equal(await pending, null);
  const route = readFileSync(new URL("../src/pages/admin/index.tsx", import.meta.url), "utf8");
  assert.match(route, /currentRouteAlertItems\(response, requestGeneration, alertRevokedRef\.current\)/);
});

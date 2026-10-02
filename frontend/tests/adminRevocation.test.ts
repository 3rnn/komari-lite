import assert from "node:assert/strict";
import test from "node:test";
import { register } from "node:module";

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

const items = (name: string) => new Response(JSON.stringify({ items: [{ node_uuid: name }] }), { status: 200 });

test("an alert endpoint 401 revokes all category and dashboard snapshots before its body is read", async (t) => {
  let finishBody!: (value: unknown) => void;
  t.mock.method(globalThis, "fetch", async (url: string) => {
    if (url.includes("/dashboard/alerts?kind=resource")) {
      return { ok: false, status: 401, json: () => new Promise((resolve) => { finishBody = resolve; }) } as Response;
    }
    return url.includes("/dashboard/alerts?") ? items("private-node")
      : new Response(JSON.stringify({ private: true }), { status: 200 });
  });
  await alerts.requestDashboardAlertItems("offline", undefined, "revocation-a");
  await dashboard.requestDashboard(["servers"], 5, "revocation-a");
  const pending = alerts.requestDashboardAlertItems("resource", undefined, "revocation-a");
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(alerts.getDashboardAlertItemsSnapshot("offline", "revocation-a"), null);
  assert.equal(dashboard.getDashboardSnapshot("servers:5", "revocation-a"), null);
  finishBody({ message: "Expired" });
  await assert.rejects(pending, (error: unknown) => (error as { status?: number }).status === 401);
});

test("an aborted alert route still revokes on a delivered 401 before reading its body", async (t) => {
  const controller = new AbortController();
  let finish!: (response: Response) => void;
  let readBody = false;
  const seen: number[] = [];
  const unsubscribe = revocation.subscribeAdminRevocation(() => seen.push(revocation.getAdminRevocationGeneration()));
  t.after(unsubscribe);
  t.mock.method(globalThis, "fetch", () => new Promise<Response>((resolve) => { finish = resolve; }));
  const pending = alerts.requestDashboardAlertItems("offline", controller.signal, "aborted-401");
  controller.abort();
  finish({ ok: false, status: 401, json: async () => { readBody = true; return {}; } } as Response);
  await assert.rejects(pending);
  assert.equal(seen.length, 1);
  assert.equal(readBody, true);
});

test("a mounted subscriber is notified on dashboard 401 and old alert responses cannot be delivered", async (t) => {
  let finishAlert!: (response: Response) => void;
  const seen: number[] = [];
  const unsubscribe = revocation.subscribeAdminRevocation(() => seen.push(revocation.getAdminRevocationGeneration()));
  t.after(unsubscribe);
  t.mock.method(globalThis, "fetch", (url: string) => url.includes("/dashboard/alerts?")
    ? new Promise<Response>((resolve) => { finishAlert = resolve; })
    : Promise.resolve(new Response("{}", { status: 401 })));
  const pendingAlert = alerts.requestDashboardAlertItems("billing", undefined, "revocation-b");
  await assert.rejects(dashboard.requestDashboard(["servers"], 5, "revocation-b"));
  finishAlert(items("old-node"));
  await assert.rejects(pendingAlert, (error: unknown) => error instanceof Error && error.name === "AbortError");
  assert.equal(seen.length, 1);
  assert.equal(alerts.getDashboardAlertItemsSnapshot("billing", "revocation-b"), null);
});

test("charts 401 clears all cached categories and summaries before body parsing", async (t) => {
  let finishBody!: (value: unknown) => void;
  t.mock.method(globalThis, "fetch", async (url: string) => url.includes("/dashboard/charts?")
    ? { ok: false, status: 401, json: () => new Promise((resolve) => { finishBody = resolve; }) } as Response
    : url.includes("/dashboard/alerts?") ? items("old-node")
      : new Response(JSON.stringify({ private: true }), { status: 200 }));
  await alerts.requestDashboardAlertItems("traffic", undefined, "revocation-c");
  await dashboard.requestDashboard(["servers"], 5, "revocation-c");
  const pending = dashboard.requestDashboardCharts(["traffic"], 5, "revocation-c");
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(alerts.getDashboardAlertItemsSnapshot("traffic", "revocation-c"), null);
  assert.equal(dashboard.getDashboardSnapshot("servers:5", "revocation-c"), null);
  finishBody({ message: "Session expired" });
  await assert.rejects(pending, (error: unknown) => (error as { status?: number }).status === 401);
});

test("an old 401 after logout cannot revoke the replacement session", async (t) => {
  let finishOld!: (response: Response) => void;
  let calls = 0;
  const seen: number[] = [];
  const unsubscribe = revocation.subscribeAdminRevocation(() => seen.push(revocation.getAdminRevocationGeneration()));
  t.after(unsubscribe);
  t.mock.method(globalThis, "fetch", () => ++calls === 1
    ? new Promise<Response>((resolve) => { finishOld = resolve; })
    : Promise.resolve(new Response(JSON.stringify({ private: true }), { status: 200 })));
  const old = dashboard.requestDashboard(["servers"], 5, "same-account");
  dashboard.clearDashboardSnapshots();
  await dashboard.requestDashboard(["servers"], 5, "same-account");
  finishOld(new Response("{}", { status: 401 }));
  await assert.rejects(old, (error: unknown) => error instanceof Error && error.name === "AbortError");
  assert.deepEqual(seen, []);
  assert.deepEqual(dashboard.getDashboardSnapshot("servers:5", "same-account"), { private: true });
});

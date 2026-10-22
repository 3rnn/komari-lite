import assert from "node:assert/strict";
import { register } from "node:module";
import test from "node:test";
const sourceRoot = new URL("../src/", import.meta.url).href;
register("data:text/javascript," + encodeURIComponent(`export async function resolve(s,c,n){return n(s.startsWith("@/")?${JSON.stringify(sourceRoot)}+s.slice(2)+".ts":s,c)}`));
const api = await import("../src/utils/dashboardApi.ts");
test("revocation aborts both dashboard network requests", async (t) => {
 const signals: AbortSignal[] = [];
 t.mock.method(globalThis, "fetch", (_url, options) => new Promise<Response>((_resolve,reject)=>{
  const signal = options?.signal as AbortSignal; if (signal) { signals.push(signal); signal.addEventListener("abort",()=>reject(signal.reason)); }
 }));
 const a=api.requestDashboard(["servers"],5,"cancel");
 const b=api.requestDashboardCharts(["traffic"],5,"cancel");
 const observed=[a.catch(e=>e),b.catch(e=>e)];
 assert.equal(signals.length,2);
 api.clearDashboardSnapshots();
 assert.ok(signals.every(s=>s.aborted));
 await Promise.all(observed);
});
test("dashboard deadline spans response body and releases pending key", async(t)=>{
 t.mock.timers.enable({apis:["setTimeout"]});
 t.mock.method(globalThis,"fetch",async()=>new Response(new ReadableStream({start(){}})));
 const result=api.requestDashboard(["servers"],5,"deadline");
 const rejected=assert.rejects(result,(e:unknown)=>e instanceof Error && e.name==="TimeoutError");
 await new Promise<void>(resolve=>queueMicrotask(resolve));
 t.mock.timers.tick(30_001);
 await rejected;
 t.mock.method(globalThis,"fetch",async()=>new Response(JSON.stringify({ok:true})));
 assert.deepEqual(await api.requestDashboard(["servers"],5,"deadline"),{ok:true});
});

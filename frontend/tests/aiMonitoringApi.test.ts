import assert from "node:assert/strict";
import test from "node:test";
import { aiRequest } from "../src/utils/aiMonitoringApi.ts";
import { revokeAdminSession, getAdminRevocationGeneration } from "../src/utils/adminRevocation.ts";


test("AI requests reject late bodies after revocation and revoke immediately on 401", async t => {
 let finish!:(value:unknown)=>void;
 t.mock.method(globalThis,"fetch",async()=>({ok:true,status:200,json:()=>new Promise(resolve=>{finish=resolve;})} as Response));
 const pending=aiRequest("overview");
 await new Promise(resolve=>setTimeout(resolve,0));
 revokeAdminSession(); finish({nodes:[{name:"private"}]});
 await assert.rejects(pending,/invalidated/);
 const generation=getAdminRevocationGeneration();
 t.mock.method(globalThis,"fetch",async()=>new Response("{}",{status:401}));
 await assert.rejects(aiRequest("settings"),/401/);
 assert.equal(getAdminRevocationGeneration(),generation+1);
});

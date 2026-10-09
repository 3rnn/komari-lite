import assert from "node:assert/strict";
import test from "node:test";
import { aiRequest, referenceRule } from "../src/utils/aiMonitoringApi.ts";
import { revokeAdminSession, getAdminRevocationGeneration } from "../src/utils/adminRevocation.ts";

test("reference selection preserves exact local model, published SKU, tier and mode without implicit activation", () => {
 const rule = referenceRule({provider:"openai",model:"published",processing_mode:"standard",context_tier:"long",input_per_million:"2",output_per_million:"8",cache_read_per_million:"0.2",cache_write_per_million:null},{source:"official",source_url:"https://example.test",version:"v1"},"local-alias","oauth","2026-10-09T00:00:00Z");
 assert.equal(rule.model,"local-alias"); assert.equal(rule.reference_model,"published"); assert.equal(rule.reference_tier,"long"); assert.equal(rule.processing_mode,"standard"); assert.equal(rule.input_per_million,2); assert.equal(rule.cache_write_per_million,null);
});
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

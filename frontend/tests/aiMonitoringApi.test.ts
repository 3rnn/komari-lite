import assert from "node:assert/strict";
import test from "node:test";
import { aiRequest, defaultOfficialReference, referenceRule } from "../src/utils/aiMonitoringApi.ts";
import { manualPricingModelOptions, modelRegistryOptions } from "../src/utils/aiMonitoring.ts";
import { revokeAdminSession, getAdminRevocationGeneration } from "../src/utils/adminRevocation.ts";

test("reference selection preserves exact local model, published SKU, tier and mode without implicit activation", () => {
 const rule = referenceRule({provider:"openai",model:"published",processing_mode:"standard",context_tier:"long",input_per_million:"2",output_per_million:"8",cache_read_per_million:"0.2",cache_write_per_million:null},{source:"official",source_url:"https://example.test",version:"v1"},"local-alias","oauth","2026-10-09T00:00:00Z");
 assert.equal(rule.model,"local-alias"); assert.equal(rule.reference_model,"published"); assert.equal(rule.reference_tier,"long"); assert.equal(rule.processing_mode,"standard"); assert.equal(rule.input_per_million,2); assert.equal(rule.cache_write_per_million,null);
});
test("one registry prioritizes gpt-6.1-sol, keeps custom models, and preserves observed legacy filters", () => {
 const options=manualPricingModelOptions(["example-custom"]);
 assert.deepEqual(options.slice(0,4).map(option=>option.value),["gpt-6.1-sol","gpt-6-sol","gpt-5.6-terra","gpt-5.6-sol"]);
 assert.ok(options.some(option=>option.value==="gpt-6-luna"));
 assert.ok(options.some(option=>option.value==="example-custom"));
 assert.ok(!options.some(option=>option.value.includes("-900k")));
 assert.equal(options.at(-1)?.label,"Add Custom Model…");
 assert.deepEqual(modelRegistryOptions(["legacy-observed","gpt-6.1-sol","legacy-observed"],["example-custom"]).map(option=>option.value).slice(-2),["example-custom","legacy-observed"]);
});

test("official defaults select only an exact verified standard short SKU", () => {
 const entries=[
  {provider:"openai",model:"gpt-6.1-sol",processing_mode:"standard",context_tier:"short",input_per_million:2,output_per_million:10,cache_read_per_million:0.1,cache_write_per_million:2.5},
  {provider:"openai",model:"gpt-6-sol",processing_mode:"standard",context_tier:"long",input_per_million:4,output_per_million:15,cache_read_per_million:0.4,cache_write_per_million:5},
 ];
 assert.equal(defaultOfficialReference(entries,"gpt-6.1-sol"),"1");
 assert.equal(defaultOfficialReference(entries,"gpt-6-sol"),"");
 assert.equal(defaultOfficialReference(entries,"unverified-local-alias"),"");
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

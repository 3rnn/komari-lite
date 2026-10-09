import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { Theme } from "@radix-ui/themes";
import { create, act } from "react-test-renderer";
import { readFileSync } from "node:fs";
import AIMonitoringPage from "../src/pages/admin/ai-monitoring.tsx";
import { AISelect } from "../src/components/admin/AIMonitoring.tsx";
import { decodeAISelectValue, encodeAISelectValue } from "../src/utils/aiMonitoring.ts";
import { revokeAdminSession } from "../src/utils/adminRevocation.ts";
async function renderAct(callback:()=>Promise<void>) { await act(callback); }
(globalThis as any).window = { setTimeout, clearTimeout };
(globalThis as any).React = React;
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

test("install integration persists opt-in and places control after detailed GPU", () => {
 const source=readFileSync("src/pages/admin/index.tsx","utf8");
 assert.ok(source.includes("enable_hermes_ai: installOptions.enableHermesAI"));
 assert.ok(source.includes("hermesInstallArgs(selectedPlatform, installOptions.enableHermesAI)"));
 assert.ok(source.indexOf("<HermesMonitoringToggle")>source.indexOf('"Enable detailed GPU monitoring"'));
 assert.ok(source.includes("hermesProfileEnabled(profile)"));
 assert.equal(JSON.parse(readFileSync("src/i18n/locales/en.json","utf8")).admin.nodeTable.onlineApplicable,"Applied after Agent acknowledgment");
});
test("AI monitoring is immediately after latency inside Monitoring, never a top-level item", () => {
 const config=JSON.parse(readFileSync("src/config/menuConfig.json","utf8"));
 const group=config.menu.find((m:any)=>m.path==="/admin/monitoring");
 assert.deepEqual(group.children.map((m:any)=>m.path),["/admin/ping","/admin/ai-monitoring"]);
 assert.equal(config.menu.some((m:any)=>m.path==="/admin/ai-monitoring"),false);
});
test("page loads real API data, sends filters, and clears private view on revocation", async t => {
 const urls:string[]=[];
 t.mock.method(globalThis,"fetch",async(url:string)=>{
 urls.push(url);
 return new Response(JSON.stringify(url.includes("overview") ? {accounts:[{account_key:"private-account",display_label:"Private account",provider:"openai",status:"unavailable",windows:[]}],nodes:[],models:[],tasks:[],daily:[],trends:[],live:{}}:url.includes("pricing") ? {rules:[],reference_catalog:{entries:[]}}:{timezone:"UTC",retention_days:30,detailed_retention_hours:24}),{status:200});
 });
 let renderer:any;
 await renderAct(async()=>{renderer=create(React.createElement(Theme, {}, React.createElement(AIMonitoringPage)));});
 assert.ok(JSON.stringify(renderer.toJSON()).includes("Private account"));
 assert.ok(urls.some(u=>u.includes("timezone=UTC")));
 assert.ok(!urls.some(u=>u.includes("overview") && u.includes("timezone=Asia%2FShanghai")));
 await renderAct(async()=>{renderer.root.findAllByType("button").find((b:any)=>b.props.children==="7D").props.onClick();});
 assert.ok(urls.filter(u=>u.includes("overview")).length>=2);
 await renderAct(async()=>{revokeAdminSession();});
 assert.ok(!JSON.stringify(renderer.toJSON()).includes("Private account"));
 assert.ok(JSON.stringify(renderer.toJSON()).includes("Session revoked"));
 await renderAct(async()=>renderer.unmount());
});

test("filter select keeps exact identifiers that equal the previous all sentinel", () => {
 assert.equal(decodeAISelectValue(encodeAISelectValue("__all")), "__all");
 assert.equal(decodeAISelectValue(encodeAISelectValue("")), "");
});

test("explicit catalog selection appends exact rates and verifies normalized timestamp readback", async t => {
 let posted:any; const rules:any[]=[]; let corruptReadback=false;
 const catalog={source:"official_docs_snapshot",source_url:"https://example.test/pricing",version:"v1",entries:[{provider:"openai",model:"published-sku",processing_mode:"standard",context_tier:"long",input_per_million:"2",output_per_million:"8",cache_read_per_million:"0.2",cache_write_per_million:null}]};
 t.mock.method(globalThis,"fetch",async(url:string,init:RequestInit)=>{
  if(init.method==="POST") {posted=JSON.parse(init.body as string);rules.splice(0,rules.length,{...posted,effective_at:posted.effective_at.replace(".000Z","Z"),...(corruptReadback ? {input_per_million:999,billing_mode:"wrong"}:{})});}
  return new Response(JSON.stringify(url.includes("pricing") ? {rules,reference_catalog:catalog}:url.includes("settings") ? {timezone:"Asia/Shanghai",retention_days:30,detailed_retention_hours:24}:{}),{status:200});
 });
 let renderer:any;
 await renderAct(async()=>{renderer=create(React.createElement(Theme, {}, React.createElement(AIMonitoringPage)));});
 await renderAct(async()=>{
  renderer.root.findAllByType(AISelect).find((c:any)=>c.props.label.startsWith("Official reference")).props.onChange("1");
  for(const [label,value] of [["Exact observed local model (no alias guessing)","local-alias"],["Exact billing mode (blank only matches blank)","oauth"],["Effective at (ISO timestamp with offset)","2026-10-09T00:00:00Z"]])renderer.root.findAllByType("input").find((i:any)=>i.props["aria-label"]===label).props.onChange({target:{value}});
 });
 await renderAct(async()=>{await renderer.root.findAllByType("form")[0].props.onSubmit({preventDefault(){}});});
 assert.equal(posted.model,"local-alias");assert.equal(posted.reference_model,"published-sku");assert.equal(posted.reference_tier,"long");assert.equal(posted.processing_mode,"standard");assert.equal(posted.cache_write_per_million,null);
 assert.ok(JSON.stringify(renderer.toJSON()).includes("Pricing version appended and verified"));
 corruptReadback=true;
 await renderAct(async()=>{await renderer.root.findAllByType("form")[0].props.onSubmit({preventDefault(){}});});
 const text=JSON.stringify(renderer.toJSON());
 await renderAct(async()=>renderer.unmount());
 assert.ok(text.includes("Appended pricing rule could not be verified"));
 assert.ok(!text.includes("Pricing version appended and verified"));
});

test("filtering to one model retains other discovered model choices", async t => {
 t.mock.method(globalThis,"fetch",async(url:string)=>new Response(JSON.stringify(url.includes("overview") ? {models: url.includes("model=one") ? [{model:"one",calls:1}]:[{model:"one",calls:1},{model:"two",calls:2}]}:url.includes("pricing") ? {rules:[]}:{timezone:"Asia/Shanghai",retention_days:30,detailed_retention_hours:24}),{status:200}));
 let renderer:any;
 await renderAct(async()=>{renderer=create(React.createElement(Theme, {}, React.createElement(AIMonitoringPage)));});
 t.after(async()=>{await renderAct(async()=>renderer.unmount());});
 await renderAct(async()=>{renderer.root.findAllByType(AISelect).find((c:any)=>c.props.label==="Model").props.onChange("one");});
 assert.ok(renderer.root.findAllByType(AISelect).find((c:any)=>c.props.label==="Model").props.options.some((o:any)=>o.value==="two"));
});

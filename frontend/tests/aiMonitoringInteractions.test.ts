import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { Theme } from "@radix-ui/themes";
import { create, act } from "react-test-renderer";
import { AIMonitoringView, HermesMonitoringToggle } from "../src/components/admin/AIMonitoring.tsx";
import AIMonitoringPage from "../src/pages/admin/ai-monitoring.tsx";
import { hermesInstallArgs } from "../src/utils/aiMonitoring.ts";
import { linuxInstallCommand, windowsInstallCommand } from "../src/utils/shellQuote.ts";

(globalThis as any).window = { setTimeout, clearTimeout };
(globalThis as any).React = React;
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
const renderAct = async (callback:()=>Promise<void>) => { await act(callback); };

test("complete installer commands carry Linux opt-in but never enable Windows", () => {
 const args=["-e","https://panel.example.test","-t","fixture-token"];
 const linux=linuxInstallCommand("https://example.test/install.sh", [...args,...hermesInstallArgs("linux",true)]);
 assert.match(linux,/--enable-hermes-ai/);
 assert.doesNotMatch(linuxInstallCommand("https://example.test/install.sh",[...args,...hermesInstallArgs("linux",false)]),/--enable-hermes-ai/);
 const windows=windowsInstallCommand("https://example.test/install.ps1",[...args,...hermesInstallArgs("windows",false)]);
 const program=Buffer.from(windows.split("-EncodedCommand ")[1],"base64").toString("utf16le");
 assert.match(program,/install.ps1/);
 assert.doesNotMatch(program,/--enable-hermes-ai/);
 assert.throws(()=>windowsInstallCommand("https://example.test/install.ps1",hermesInstallArgs("windows",true)),/unsupported/);
});

test("toggle calls the opt-in handler and permits clearing an invalid Windows profile", async () => {
 let renderer:any;const changes:boolean[]=[];
 await renderAct(async()=>{renderer=create(React.createElement(Theme,{},React.createElement(HermesMonitoringToggle,{platform:"linux",enabled:false,onChange:value=>changes.push(value)})));});
 const checkbox=renderer.root.findAllByType("button").find((b:any)=>b.props.role==="checkbox");
 await renderAct(async()=>checkbox.props.onClick({defaultPrevented:false,isPropagationStopped:()=>false,stopPropagation(){}}));
 assert.deepEqual(changes,[true]);
 await renderAct(async()=>renderer.unmount());
});

test("daily model bars switch denominators and model columns sort actual rows", async () => {
 let renderer:any;
 const models=[{model:"small",calls:1,total_tokens:900,api_equivalent_cost_usd:3,cost_share:0.75},{model:"large",calls:3,total_tokens:100,api_equivalent_cost_usd:1,cost_share:0.25}];
 await renderAct(async()=>{renderer=create(React.createElement(Theme,{},React.createElement(AIMonitoringView,{timezone:"Asia/Shanghai",data:{models,daily:[{date:"2026-10-09",calls:4,total_tokens:1000,api_equivalent_cost_usd:4,models}]}})));});
 const widths=()=>renderer.root.findAllByType("div").filter((d:any)=>d.props.style?.width).map((d:any)=>d.props.style.width);
 assert.deepEqual(widths(),["90%","10%"]);
 const button=(text:string)=>renderer.root.findAllByType("button").find((b:any)=>b.props.children===text);
 await renderAct(async()=>button("Call share").props.onClick());
 assert.deepEqual(widths(),["25%","75%"]);
 await renderAct(async()=>button("Cost share").props.onClick());
 assert.deepEqual(widths(),["75%","25%"]);
 const firstModel=()=>renderer.root.findByType("tbody").findAllByType("tr")[0].findAllByType("td")[0].findAllByType("span")[0].children[0];
 assert.equal(firstModel(),"large");
 await renderAct(async()=>button("Calls").props.onClick());
 assert.equal(firstModel(),"small");
 await renderAct(async()=>button("Table").props.onClick());
 assert.ok(JSON.stringify(renderer.toJSON()).includes("Sampled tokens/s"));
 await renderAct(async()=>renderer.unmount());
});

test("node cards distinguish pending, rejected and acknowledged collection settings", async () => {
 let renderer:any;
 await renderAct(async()=>{renderer=create(React.createElement(Theme,{},React.createElement(AIMonitoringView,{timezone:"Asia/Shanghai",data:{nodes:[{uuid:"one",name:"Pending node",enabled:true,status:"unavailable",delivery_status:"sent"},{uuid:"two",name:"Rejected node",enabled:true,status:"unavailable",delivery_status:"failed"},{uuid:"three",name:"Acknowledged node",enabled:true,status:"collecting",delivery_status:"applied"}]}})));});
 const text=JSON.stringify(renderer.toJSON());
 assert.ok(text.includes("pending Agent acknowledgment"));
 assert.ok(text.includes("Failed · Agent rejected"));
 assert.ok(text.includes("Applied · Agent acknowledged"));
 await renderAct(async()=>renderer.unmount());
});

test("overview failure shows a recoverable error rather than stale private usage", async t => {
 let fail=true;let renderer:any;
 t.mock.method(globalThis,"fetch",async(url:string)=>new Response(JSON.stringify(url.includes("overview") ? {models:[{model:"recovered-model",calls:2}]}:url.includes("pricing") ? {rules:[]}:{timezone:"Asia/Shanghai",retention_days:30,detailed_retention_hours:24}),{status:url.includes("overview")&&fail ? 503:200}));
 await renderAct(async()=>{renderer=create(React.createElement(Theme,{},React.createElement(AIMonitoringPage)));});
 t.after(async()=>{await renderAct(async()=>renderer.unmount());});
 assert.ok(JSON.stringify(renderer.toJSON()).includes("HTTP 503"));
 assert.ok(!JSON.stringify(renderer.toJSON()).includes("recovered-model"));
 fail=false;
 await renderAct(async()=>renderer.root.findAllByType("button").find((b:any)=>b.props.children==="Retry").props.onClick());
 assert.ok(JSON.stringify(renderer.toJSON()).includes("recovered-model"));
 assert.ok(!JSON.stringify(renderer.toJSON()).includes("HTTP 503"));
});

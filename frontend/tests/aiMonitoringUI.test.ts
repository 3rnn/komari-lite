import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { AIAdvanced, AIMonitoringView, HermesMonitoringToggle } from "../src/components/admin/AIMonitoring.tsx";

test("default dashboard has exactly four compact monitoring sections", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{data:null,timezone:"Asia/Shanghai"}));
 for(const label of ["Account Quotas","Live Overview","Usage Trend","Daily Usage Summary"]) assert.ok(html.includes(label),label);
 assert.ok(!html.includes("Model statistics"));
 for(const label of ["Average tokens / call","Average calls / hour","Collected tokens","Collected calls"]) assert.ok(html.includes(label),label);
 assert.ok(!html.includes("Token / Second"));
 assert.ok(!html.includes("Requests / Minute"));
 assert.ok(!html.includes("API-Equivalent Cost"));
});
test("quota cards make remaining and used meaning explicit without exposing account keys", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{accounts:[{account_key:"opaque-private-account-hash",provider:"openai-codex",plan:"Plus",status:"available",last_success_at:0,windows:[{label:"Session",used_percent:9,resets_at:"2026-10-09T19:28:00Z"},{label:"Weekly",used_percent:70,resets_at:"2026-10-15T12:37:00Z"}]}]}}));
 assert.ok(html.includes("91% remaining · 9% used"));
 assert.ok(html.includes("30% remaining · 70% used"));
 assert.ok(html.includes("Session quota remaining"));
 assert.ok(!html.includes("opaque-private-account-hash"));
});
test("AI timestamps show UTC and Beijing dates in 24-hour time", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{accounts:[{provider:"openai-codex",status:"available",last_success_at:1791574080,windows:[{label:"Session",used_percent:9,resets_at:"2026-10-09T19:28:00Z"}]}]}}));
 assert.ok(html.includes("UTC 09 Oct 2026, 19:28 · Beijing 10 Oct 2026, 03:28"));
});
test("daily cards group visual model rows while retaining truthful partial prices and output rates", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{daily:[{date:"2026-10-11",calls:2,total_tokens:200,api_equivalent_cost_usd:.01,unpriced_calls:1,models:[{model:"gpt-5.6-terra",provider:"openai-codex",billing_mode:"subscription_included",calls:1,total_tokens:120,api_equivalent_cost_usd:.01,output_tps:2},{model:"gpt-5.6-terra",provider:"openai-codex",billing_mode:"unknown",calls:1,total_tokens:80,unpriced_calls:1,output_tps:1}]}]}}));
 const text=html.replace(/<!--.*?-->/g,"");
 assert.ok(text.includes("Sunday, 11 Oct 2026"));
 assert.ok(text.includes("Partial · $0.01 · 200 tokens · 2 calls"));
 assert.ok(text.includes("Partial · $0.01 · 200 · 3 output tok/s"));
 assert.equal((text.match(/output tok\/s/g)??[]).length,1);
 assert.doesNotMatch(text,/OpenAI Codex|subscription included|Billing mode unknown/);
 assert.match(html,/width:100%/);
});
test("healthy zero-delta periods use an empty state and suppress zero-only model rows", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{trends:[{timestamp:1,calls:0,total_tokens:0}],daily:[{date:"2026-10-09",calls:0,total_tokens:0,models:[{model:"zero-only",calls:0,total_tokens:0}]}]}}));
 assert.ok(html.includes("Collection is healthy, but no new usage occurred in this range."));
 assert.ok(html.includes("No new usage in this range."));
 assert.ok(!html.includes("zero-only"));
});
test("usage trend states explicit compact axes for token-scale data", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{trends:[{timestamp:1791574080,calls:1530,total_tokens:1250000,api_equivalent_cost_usd:18.327304},{timestamp:1791577680,calls:1600,total_tokens:1300000,api_equivalent_cost_usd:18.5}]}}));
 assert.ok(html.includes("Y axis: Tokens · X axis: UTC"));
 assert.ok(html.includes("Tokens on the vertical axis and UTC on the horizontal axis"));
});
test("range cards retain collected zeroes but mark incomplete coverage PARTIAL", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{range_usage:{average_calls_per_hour:0,collected_calls:0,collected_tokens:0,coverage:"partial"}}}));
 assert.ok(html.includes("PARTIAL"));
 assert.ok(html.includes("Collected tokens"));
 assert.ok(html.includes("Collected calls"));
 assert.ok(html.includes("0"));
});
test("Advanced is one compact detailed usage table with exact model identifiers", () => {
 const html=renderToStaticMarkup(React.createElement(AIAdvanced,{timezone:"UTC",data:{models:[{model:"gpt-6.1-sol",task:"chat",calls:1,total_tokens:10,api_equivalent_cost_usd:.01}]},children:React.createElement("div",null,"filters")}));
 assert.ok(html.includes("Detailed usage"));
 assert.ok(html.includes("gpt-6.1-sol"));
 assert.ok(!html.includes("Model: N/A"));
 for(const removed of ["Full model statistics","Task usage","Detailed token accounting","Collector diagnostics","Automatic pricing"]) assert.ok(!html.includes(removed),removed);
});
test("Windows checkbox is explicitly disabled and Linux defaults unchecked", () => {
 const windows=renderToStaticMarkup(React.createElement(HermesMonitoringToggle,{platform:"windows",enabled:false,onChange:()=>{}}));
 const linux=renderToStaticMarkup(React.createElement(HermesMonitoringToggle,{platform:"linux",enabled:false,onChange:()=>{}}));
 assert.ok(windows.includes("unsupported on Windows")); assert.ok(linux.includes('aria-checked="false"'));
});

import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { AIAdvanced, AIMonitoringView, HermesMonitoringToggle } from "../src/components/admin/AIMonitoring.tsx";

test("compact usage deduplicates quotas and never calls covered zero a complete range",()=>{
 const account={account_key:"same",provider:"openai-codex",status:"available",last_success_at:1,windows:[{label:"Weekly",used_percent:20}]};
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{accounts:[account,{...account,last_success_at:2}],range_usage:{coverage:"partial"},trends:[{timestamp:1,total_tokens:0,coverage:"covered"},{timestamp:2,total_tokens:0,coverage:"missing"}],daily:[{date:"2026-10-12",calls:2,total_tokens:12,models:[]}]}}));
 assert.equal((html.match(/Weekly quota remaining/g)||[]).length,1);
 assert.ok(html.includes("Active account unconfirmed"));
 assert.ok(html.includes("covered intervals only"));
 assert.equal((html.match(/Collection incomplete/g)||[]).length,1);
 assert.ok(html.includes("By day"));assert.ok(html.includes("By model"));
});
test("collection evidence distinguishes retained proofs, current node health and unknown routing",()=>{
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{range_usage:{coverage:"partial",coverage_reason:"rollup_interval_evidence_unavailable"},nodes:[{uuid:"a",name:"Alpha",enabled:true,status:"available",last_report_at:1791574080,last_success_at:1791574080}],daily:[{date:"2026-10-12",calls:1,total_tokens:1,coverage:"covered"}]}}));
 assert.ok(html.includes("Hourly rollups do not retain interval proofs"));assert.ok(html.includes("Current collection health"));assert.ok(html.includes("Alpha"));assert.ok(html.includes("Daily coverage: covered"));
});
test("authoritative pricing aggregate replaces average with explicit partial estimate",()=>{const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{pricing_coverage:{calls:3,unpriced_calls:1,api_equivalent_cost_usd:0},range_usage:{collected_calls:3,collected_tokens:30,average_tokens_per_call:10,coverage:"partial"}}}));assert.ok(html.includes("Priced subtotal $0 · 2 priced + 1 unpriced calls"));assert.ok(html.includes("API-equivalent estimate"));});
test("all-unpriced daily usage never presents zero dollars",()=>{
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{daily:[{date:"2026-10-14",calls:52,total_tokens:100,priced_calls:0,unpriced_calls:52,api_equivalent_cost_usd:0,models:[{model:"test",calls:52,total_tokens:100,priced_calls:0,unpriced_calls:52,api_equivalent_cost_usd:0}]}]}}));
 assert.ok(html.includes("Unpriced"));assert.ok(!html.includes("$0"));
});
test("daily tables omit an unavailable task dimension and expose scroll help outside the table",()=>{
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{daily:[{date:"2026-10-14",calls:1,total_tokens:10,models:[{model:"test",provider:"provider-fixture",billing_mode:"subscription_included",calls:1,total_tokens:10}]}]}}));
 assert.ok(!html.includes("Unknown task"));assert.ok(!html.includes(">Task<"));assert.ok(html.includes("provider-fixture"));assert.ok(html.includes("subscription_included"));
 assert.match(html,/Swipe horizontally to view all columns\.<\/p><div[^>]*overflow-x-auto/);
 assert.ok(!html.includes("<caption"));
});
test("quota details distinguish observation from success and disclose missing attempt telemetry",()=>{
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{accounts:[{provider:"openai-codex",status:"stale",observed_at:1791951436,last_success_at:1791950528}]}}));
 assert.ok(html.includes("Last observation"));assert.ok(html.includes("Refresh attempt"));assert.ok(html.includes("not reported"));
});
test("default dashboard has exactly four compact monitoring sections", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{data:null,timezone:"Asia/Shanghai"}));
 for(const label of ["Account Quotas","Usage Overview","Usage Trend","Daily Usage Summary"]) assert.ok(html.includes(label),label);
 assert.ok(!html.includes("Model statistics"));
 for(const label of ["Average tokens / call","Collected tokens","Collected calls"]) assert.ok(html.includes(label),label);
 assert.ok(!html.includes("Average calls / hour"));
 assert.ok(html.includes("Tokens include input, cache-read, cache-write, and output usage"));
 assert.ok(!html.includes("Token / Second"));
 assert.ok(!html.includes("Requests / Minute"));
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
test("daily cards use compact tokens, two-decimal API-equivalent estimates, and no output throughput", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{range_usage:{coverage:"covered"},daily:[{date:"2026-10-11",calls:2,total_tokens:21360000,api_equivalent_cost_usd:.01,unpriced_calls:1,models:[{model:"gpt-5.6-terra",provider:"openai-codex",billing_mode:"subscription_included",calls:1,total_tokens:12000000,api_equivalent_cost_usd:.01,output_tps:2},{model:"gpt-5.6-terra",provider:"openai-codex",billing_mode:"unknown",calls:1,total_tokens:9360000,unpriced_calls:1,output_tps:1}]}]}}));
 const text=html.replace(/<!--.*?-->/g,"");
 assert.ok(text.includes("Sunday, 11 Oct 2026"));
 assert.ok(text.includes("21.36M Tokens · 2 calls · Priced subtotal $0.01 · 1 priced + 1 unpriced calls"));
 assert.ok(text.includes("21.36M · 2 calls · Priced subtotal $0.01 · 1 priced + 1 unpriced calls"));
 assert.ok(text.includes("not an actual ChatGPT subscription charge"));
 assert.doesNotMatch(text,/output tok\/s/);
 assert.match(html,/width:100%/);
});
test("trend distinguishes verified zero activity from missing coverage", () => {
 const verified=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{range_usage:{coverage:"covered"},trends:[{timestamp:1,calls:0,total_tokens:0,coverage:"covered"}],daily:[{date:"2026-10-09",calls:0,total_tokens:0,models:[{model:"zero-only",calls:0,total_tokens:0}]}]}}));
 assert.ok(verified.includes("Verified collection shows no new usage in this range."));
 assert.ok(!verified.includes("zero-only"));
 const partial=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{range_usage:{coverage:"partial"},trends:[{timestamp:1,calls:0,total_tokens:0,coverage:"partial"}]}}));
 assert.ok(partial.includes("Collection incomplete"));
});
test("usage trend states explicit compact axes for token-scale data", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{range_usage:{coverage:"covered"},trends:[{timestamp:1791574080,calls:1530,total_tokens:1250000,coverage:"covered"},{timestamp:1791577680,calls:1600,total_tokens:1300000,coverage:"covered"}]}}));
 assert.ok(html.includes("Tokens · UTC"));
 assert.ok(html.includes("Tokens on the vertical axis and UTC on the horizontal axis"));
 assert.ok(!html.includes(">Cost<"));
});
test("range cards retain collected zeroes but mark incomplete coverage PARTIAL", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{range_usage:{collected_calls:0,collected_tokens:0,coverage:"partial"}}}));
 assert.ok(html.includes("Collection incomplete"));
 assert.ok(html.includes("Collected tokens"));
 assert.ok(html.includes("Collected calls"));
 assert.ok(html.includes("0"));
});
test("Advanced is one compact detailed usage table with exact model identifiers", () => {
 const html=renderToStaticMarkup(React.createElement(AIAdvanced,{timezone:"UTC",data:{models:[{model:"gpt-6.1-sol",task:"chat",calls:1,total_tokens:10,api_equivalent_cost_usd:.01},{model:"legacy-zero",task:"main",calls:0,total_tokens:0}]},children:React.createElement("div",null,"filters")}));
 assert.ok(html.includes("Detailed usage"));
 assert.ok(html.includes("gpt-6.1-sol"));
 assert.ok(!html.includes("legacy-zero"));
 assert.ok(!html.includes("Model: N/A"));
 for(const removed of ["Full model statistics","Task usage","Detailed token accounting","Collector diagnostics","Automatic pricing"]) assert.ok(!html.includes(removed),removed);
});
test("Windows checkbox is explicitly disabled and Linux defaults unchecked", () => {
 const windows=renderToStaticMarkup(React.createElement(HermesMonitoringToggle,{platform:"windows",enabled:false,onChange:()=>{}}));
 const linux=renderToStaticMarkup(React.createElement(HermesMonitoringToggle,{platform:"linux",enabled:false,onChange:()=>{}}));
 assert.ok(windows.includes("unsupported on Windows")); assert.ok(linux.includes('aria-checked="false"'));
});
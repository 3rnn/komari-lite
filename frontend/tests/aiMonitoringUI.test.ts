import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { AIMonitoringView, HermesMonitoringToggle } from "../src/components/admin/AIMonitoring.tsx";

test("default dashboard has exactly four compact monitoring sections", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{data:null,timezone:"Asia/Shanghai"}));
 for(const label of ["Account Quotas","Live Overview","Usage Trend","Daily Usage Summary"]) assert.ok(html.includes(label),label);
 assert.ok(!html.includes("Model statistics"));
 assert.ok(html.includes("Token / Second"));
 assert.ok(html.includes("API-Equivalent Cost"));
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
test("daily duplicate model rows disclose their verified grouping and unknown account", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{daily:[{date:"2026-10-11",calls:100,total_tokens:13883709,models:[{model:"gpt-5.6-terra",provider:"openai-codex",billing_mode:"subscription_included",calls:88,total_tokens:12669678},{model:"gpt-5.6-terra",provider:"openai-codex",billing_mode:"unknown",calls:12,total_tokens:1214031}]}]}}));
 assert.ok(html.includes("OpenAI Codex · subscription included · Account unknown"));
 assert.ok(html.includes("OpenAI Codex · Billing mode unknown · Account unknown"));
});
test("healthy zero-delta periods use an empty state and suppress zero-only model rows", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{trends:[{timestamp:1,calls:0,total_tokens:0}],daily:[{date:"2026-10-09",calls:0,total_tokens:0,models:[{model:"zero-only",calls:0,total_tokens:0}]}]}}));
 assert.ok(html.includes("Collection is healthy, but no new usage occurred in this range."));
 assert.ok(html.includes("No new usage in this range."));
 assert.ok(!html.includes("zero-only"));
});
test("Today Tokens render Unknown rather than a verified zero during a coverage gap", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{live:{today_tokens:0,today_coverage:"unknown",status:"stale"}}}));
 assert.ok(html.includes("Today Tokens"));
 assert.ok(html.includes("Unknown"));
 assert.ok(html.includes("Collection coverage gap; collected zero is not a verified total."));
});
test("Windows checkbox is explicitly disabled and Linux defaults unchecked", () => {
 const windows=renderToStaticMarkup(React.createElement(HermesMonitoringToggle,{platform:"windows",enabled:false,onChange:()=>{}}));
 const linux=renderToStaticMarkup(React.createElement(HermesMonitoringToggle,{platform:"linux",enabled:false,onChange:()=>{}}));
 assert.ok(windows.includes("unsupported on Windows")); assert.ok(linux.includes('aria-checked="false"'));
});

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
test("healthy zero-delta periods use an empty state and suppress zero-only model rows", () => {
 const html=renderToStaticMarkup(React.createElement(AIMonitoringView,{timezone:"UTC",data:{trends:[{timestamp:1,calls:0,total_tokens:0}],daily:[{date:"2026-10-09",calls:0,total_tokens:0,models:[{model:"zero-only",calls:0,total_tokens:0}]}]}}));
 assert.ok(html.includes("Collection is healthy, but no new usage occurred in this range."));
 assert.ok(html.includes("No new usage in this range."));
 assert.ok(!html.includes("zero-only"));
});
test("Windows checkbox is explicitly disabled and Linux defaults unchecked", () => {
 const windows=renderToStaticMarkup(React.createElement(HermesMonitoringToggle,{platform:"windows",enabled:false,onChange:()=>{}}));
 const linux=renderToStaticMarkup(React.createElement(HermesMonitoringToggle,{platform:"linux",enabled:false,onChange:()=>{}}));
 assert.ok(windows.includes("unsupported on Windows")); assert.ok(linux.includes('aria-checked="false"'));
});

import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { AIMonitoringView, HermesMonitoringToggle } from "../src/components/admin/AIMonitoring.tsx";

test("empty dashboard renders all seven sections with unavailable rates and no invented quota", () => {
 const html = renderToStaticMarkup(React.createElement(AIMonitoringView, { data: null, timezone: "Asia/Shanghai" }));
 for (const label of ["Account quotas", "Sampled throughput", "Usage trends", "Daily summaries", "Model statistics", "Task usage", "Monitored nodes", "10s average", "Rolling 60s", "N/A", "Quota unavailable"]) assert.ok(html.includes(label), label);
 assert.ok(html.includes("Estimated API-Equivalent Cost (USD)"));
 assert.ok(html.includes("not an actual ChatGPT Plus charge"));
});
test("quota windows stay per account; stale data and unknown tasks are not made zero", () => {
 const html = renderToStaticMarkup(React.createElement(AIMonitoringView, { timezone: "Asia/Shanghai", data: { accounts: [{ account_key: "one", provider: "openai", plan: "Plus", status: "stale", windows: [{label: "Weekly", used_percent: null, resets_at: "2026-10-10T00:00:00Z"}] }], tasks: [{task: "", calls: 3}], models: [], daily: [], nodes: [], trends: [], live: {tps:null} } }));
 assert.ok(html.includes("stale")); assert.ok(html.includes("Unknown task")); assert.ok(html.includes("Weekly")); assert.ok(!html.includes("0%")); assert.ok(html.includes("Resets"));
});
test("Windows checkbox is explicitly disabled and Linux defaults unchecked", () => {
 const windows = renderToStaticMarkup(React.createElement(HermesMonitoringToggle, { platform: "windows", enabled: false, onChange: () => {} }));
 assert.ok(windows.includes("disabled")); assert.ok(windows.includes("unsupported on Windows"));
 const linux = renderToStaticMarkup(React.createElement(HermesMonitoringToggle, { platform:"linux", enabled:false, onChange:()=>{} }));
 const legacyEnabled = renderToStaticMarkup(React.createElement(HermesMonitoringToggle, { platform:"windows", enabled:true, onChange:()=>{} }));
 assert.ok(!legacyEnabled.includes('disabled=""'), "Allow disabling an invalid enabled Windows profile");
 assert.ok(linux.includes('aria-checked="false"'));
});

test("model tables expose cost share and unavailable generation duration without inventing pricing", () => {
 const html = renderToStaticMarkup(React.createElement(AIMonitoringView, {timezone:"Asia/Shanghai", data:{models:[{model:"unknown",calls:4,api_equivalent_cost_usd:null,unpriced_calls:4,cost_share:null}]}}));
 assert.ok(html.includes("Cost share"));
 assert.ok(html.includes("Generation duration"));
 assert.match(html,/>Unpriced</);
 assert.ok(html.includes("N/A"));
});

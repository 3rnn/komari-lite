import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");

test("dashboard metrics share technical typography without changing labels", () => {
  const css = read("../src/global.css");
  const dashboard = read("../src/components/admin/DashboardPanels.tsx");
  assert.match(css, /--km-metric-font:\s*ui-monospace/);
  assert.match(css, /\.km-metric-value\s*\{[^}]*font-family:\s*var\(--km-metric-font\);[^}]*font-variant-numeric:\s*tabular-nums/s);
  const metricValues = dashboard.match(/km-metric-value[^>]*>\{value\}/g) ?? [];
  assert.ok(metricValues.length >= 4, "summary, ranking, alert, and latency values use the shared style");
});

test("long metric values can wrap inside narrow dashboard panels", () => {
  const css = read("../src/global.css");
  const dashboard = read("../src/components/admin/DashboardPanels.tsx");
  assert.match(css, /\.km-metric-value\s*\{[^}]*overflow-wrap:\s*anywhere/s);
  assert.match(dashboard, /<strong className=\{`(?=[^`]*\bmin-w-0\b)(?=[^`]*\bkm-metric-value\b)[^`]*`\}>\{value\}<\/strong>/);
  assert.match(dashboard, /grid-rows-\[minmax\(1rem,auto\)_0\.375rem_1rem\]/);
});

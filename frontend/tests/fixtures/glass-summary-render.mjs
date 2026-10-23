// Execute the actual shipped overview and its byte-unit formatter in isolation.
import assert from "node:assert/strict";
import { runInNewContext } from "node:vm";
import * as React from "react";
import { jsx, jsxs } from "react/jsx-runtime";
import { renderToStaticMarkup } from "react-dom/server";

export function summaryRenderer(code, context, globals = {}) {
  const start = code.indexOf("function rK(");
  const end = code.indexOf("function rW()", start);
  assert.ok(start > 0 && end > start);
  const unitsStart = code.indexOf("let ek=[");
  const unitsEnd = code.indexOf("function eA(", unitsStart);
  const icon = props => jsx("svg", { ...props, "aria-hidden": true });
  return runInNewContext(`${code.slice(unitsStart, unitsEnd)};${code.slice(start, end)};rH`, {
    R: React, L: { jsx, jsxs }, eB: () => context,
    rF: icon, rU: icon, rB: icon, rL: icon, r$: icon, rR: icon,
    ex: value => `¥${value}`, t9: (...values) => values.filter(Boolean).join(" "),
    Date, setTimeout, clearTimeout, ...globals,
  });
}
export function renderSummary(code, context) {
  return renderToStaticMarkup(jsx(summaryRenderer(code, context), {}));
}
export function summaryContext(nodes) {
  return {
    nodes,
    settings: { showStatsBar: true, showOnline: true, showAssets: true, showTraffic: true, showSpeed: true },
    overview: { online: 11, total: 12, offline: 1, totalValue: 821.71, remainingValue: 661.8, paidCount: 12,
      trafficTotal: 507.55 * 1024 ** 3, trafficUp: 197.68 * 1024 ** 3, trafficDown: 309.87 * 1024 ** 3,
      speedUp: 16 * 1024 ** 2, speedDown: 8 * 1024 ** 2 },
  };
}

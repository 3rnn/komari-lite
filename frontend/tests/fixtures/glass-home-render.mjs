// SSR fixture executes the shipped node-card DOM, not a hand-written facsimile.
// Ping is disabled in this isolated fixture: its real renderer shows no samples.
import assert from "node:assert/strict";
import { runInNewContext } from "node:vm";
import * as React from "react";
import { jsx, jsxs, Fragment } from "react/jsx-runtime";
import { renderToStaticMarkup } from "react-dom/server";
import { summaryRenderer } from "./glass-summary-render.mjs";

export function renderGlassHome(code, context, view = "cards") {
  const slice = (start, end) => {
    const from = code.indexOf(start), to = code.indexOf(end, from);
    assert.ok(from > 0 && to > from, `shipped ${start}`);
    return code.slice(from, to);
  };
  const icon = props => jsx("svg", { ...props, "aria-hidden": true });
  const support = slice("let ek=[", "function eP(") + slice("let rk=[", "// Pure node-card") + slice("function resolveNodePingSelection(", "function rP(") + slice("function ru(", "function rc(");
  const renderers = slice("function rP(", "function rz()");
  const Card = runInNewContext(`${support};${renderers};rP`, {
    R: React, L: { jsx, jsxs, Fragment },
    t9: (...classes) => classes.filter(Boolean).join(" "),
    rs: { success: "bg-success", info: "bg-info", warning: "bg-warning", error: "bg-destructive" },
    ey: () => false, em: () => "unknown", ep: () => 0,
    rw: label => Array.from({ length: 32 }, (_, i) => ({ key: String(i), className: "bg-muted-foreground/10", tooltip: label })),
    e0: icon, e1: icon, e2: icon, e3: icon, e4: icon, e5: icon, e6: icon,
    rl: () => null,
  });
  const Summary = summaryRenderer(code, context);
  const home = slice("function rW()", "let rV=").replace('R.useState("cards")', `R.useState(${JSON.stringify(view)})`);
  const Home = runInNewContext(`${home};rW`, {
    R: React, L: { jsx, jsxs }, rH: Summary,
    rz: () => jsx("div", { className: "node-view-grid grid grid-cols-1 items-start gap-3 sm:grid-cols-[repeat(auto-fill,minmax(320px,1fr))]", children: context.nodes.map(node => jsx(Card, { node, pingEnabled: false, showCarrierPing: false, onClick: () => {} }, node.uuid)) }),
  });
  return renderToStaticMarkup(jsx(Home, {}));
}

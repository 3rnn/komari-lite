import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import * as React from "react";
import { jsx, jsxs } from "react/jsx-runtime";
import { act, create } from "react-test-renderer";

globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const html = readFileSync(new URL("index.html", dist), "utf8");
const name = html.match(/3859ru-[a-f0-9]{8}\.js/)[0];
const code = readFileSync(new URL(`_next/static/chunks/${name}`, dist), "utf8");
function homepage(storage) {
  const start = code.indexOf("function rW()");
  const end = code.indexOf("let rV=", start);
  return runInNewContext(`${code.slice(start, end)};rW`, {
    R: React, L: { jsx, jsxs }, localStorage: storage,
    rH: () => jsx("section", { children: "Overview" }),
    rz: () => jsx("article", { children: "Unchanged node metrics" }),
  });
}
test("homepage defaults to cards and toggles the same nodes with accessible persisted state", async () => {
  const saved = new Map();
  const Home = homepage({ getItem: key => saved.get(key), setItem: (key, value) => saved.set(key, value) });
  let tree;
  await act(() => { tree = create(jsx(Home, {})); });
  const button = () => tree.root.findByType("button");
  assert.equal(button().props["aria-pressed"], false);
  assert.equal(button().props.title, "List view");
  assert.equal(button().props.children.type, "svg");
  assert.equal(button().props.children.props["aria-hidden"], true);
  const node = tree.root.findByType("article");
  await act(() => button().props.onClick());
  assert.equal(button().props["aria-pressed"], true);
  assert.equal(button().props.title, "Card view");
  assert.equal(tree.root.findByType("article"), node, "do not remount monitoring nodes");
  assert.equal(saved.get("komari-glass-node-view"), "list");
  await act(() => tree.unmount());
  await act(() => { tree = create(jsx(Home, {})); });
  assert.equal(button().props["aria-pressed"], true);
  await act(() => button().props.onClick());
  assert.equal(saved.get("komari-glass-node-view"), "cards");
  await act(() => tree.unmount());
});

test("invalid preferences and blocked storage fail safely", async () => {
  for (const storage of [
    { getItem: () => "invalid", setItem: () => {} },
    { getItem: () => { throw Error("disabled"); }, setItem: () => { throw Error("disabled"); } },
  ]) {
    let tree;
    await act(() => { tree = create(jsx(homepage(storage), {})); });
    const button = () => tree.root.findByType("button");
    assert.equal(button().props["aria-pressed"], false);
    assert.equal(button().props.type, "button");
    await act(() => button().props.onClick());
    assert.equal(button().props["aria-pressed"], true);
    await act(() => tree.unmount());
  }
});

test("list mode has a scoped full-width responsive layout without replacing the node renderer", () => {
  const cssName = html.match(/glass-visual-[a-f0-9]{8}\.css/)[0];
  const css = readFileSync(new URL(cssName, dist), "utf8");
  assert.match(code, /node-view-grid grid grid-cols-1/);
  assert.match(css, /\[data-node-view="list"\] \.node-view-grid/);
  assert.match(css, /grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(css, /@media \(min-width: 1100px\)/);
  assert.match(css, /\.node-card > div \{ display: contents; \}/, "flatten wrappers instead of preserving tall card body");
  assert.match(css, /\.node-card > header \{ padding: 0; border: 0; min-height: 0; \}/, "no full-height name divider");
  assert.match(css, /\.grid:nth-child\(4\) \{ grid-column: 1 \/ -1; grid-template-columns: repeat\(2, minmax\(0, 1fr\)\)/, "latency and loss stay side by side");
  assert.match(css, /\[data-node-view="list"\] \.node-data-panel \.tabular-nums \{ white-space: normal; overflow-wrap: anywhere;/, "list network values must remain readable in narrow metric columns");
  assert.match(code, /onKeyDown:e=>\{\("Enter"===e.key\|\|" "===e.key\)/);
});

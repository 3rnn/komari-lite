import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import * as React from "react";
import { jsx, jsxs } from "react/jsx-runtime";
import { act, create } from "react-test-renderer";
import { resolveNodePingSelection } from "../script/glass-ping-view.mjs";

globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const html = readFileSync(new URL("index.html", dist), "utf8");
const name = html.match(/3859ru-[a-f0-9]{8}\.js/)[0];
const code = readFileSync(new URL(`_next/static/chunks/${name}`, dist), "utf8");
const nodes = [
  { uuid: "green", name: "greencloud", online: true, display_ping_task_ids: [3] },
  { uuid: "edge", name: "edge", online: false },
];
function homepage(storage, globals = {}) {
  const start = code.indexOf("function rW()");
  const end = code.indexOf("let rV=", start);
  return runInNewContext(`${code.slice(start, end)};({ rW, glassNodeTable })`, {
    R: React, L: { jsx, jsxs }, localStorage: storage,
    eB: () => ({ nodes, sitename: "Test Panel", metricRetention: { pingHours: 24 } }),
    rH: () => jsx("section", { children: "Overview" }),
    rz: props => jsx("div", { id: "nodes", ...props }),
    eS: () => 0, eA: value => value, resolveNodePingSelection,
    ...globals,
  });
}
const viewButton = (tree, name) => tree.root.findAllByType("button").find(button => button.props.children === name);

test("list defaults, search filters both views and saved cards preferences survive reopening", async () => {
  const saved = new Map();
  const { rW } = homepage({ getItem: key => saved.get(key), setItem: (key, value) => saved.set(key, value) });
  let tree;
  await act(() => { tree = create(jsx(rW, {})); });
  assert.equal(viewButton(tree, "List").props["aria-pressed"], true);
  await act(() => tree.root.findByType("input").props.onChange({ target: { value: " GREEN " } }));
  assert.deepEqual(tree.root.findByProps({ id: "nodes" }).props.visibleNodes, [nodes[0]]);
  await act(() => viewButton(tree, "Cards").props.onClick());
  assert.equal(viewButton(tree, "Cards").props["aria-pressed"], true);
  assert.deepEqual(tree.root.findByProps({ id: "nodes" }).props.visibleNodes, [nodes[0]]);
  assert.equal(saved.get("komari-glass-node-view"), "cards");
  await act(() => tree.unmount());
  await act(() => { tree = create(jsx(rW, {})); });
  assert.equal(viewButton(tree, "Cards").props["aria-pressed"], true);
  await act(() => tree.unmount());
});

test("invalid preferences and blocked storage leave a usable list and toggle", async () => {
  for (const storage of [
    { getItem: () => "invalid", setItem: () => {} },
    { getItem: () => { throw Error("disabled"); }, setItem: () => { throw Error("disabled"); } },
  ]) {
    let tree;
    await act(() => { tree = create(jsx(homepage(storage).rW, {})); });
    assert.equal(viewButton(tree, "List").props["aria-pressed"], true);
    await act(() => viewButton(tree, "Cards").props.onClick());
    assert.equal(viewButton(tree, "Cards").props["aria-pressed"], true);
    await act(() => tree.unmount());
  }
});

test("list columns show the same selected probe and one-hour window as the card", async () => {
  const { glassNodeTable } = homepage({}, {
    useGlassNodePing(uuid, enabled, hours, selection) {
      assert.equal(uuid, "green");
      assert.equal(enabled, true);
      assert.equal(hours, 1);
      assert.equal(selection.preferredId, 3);
      assert.equal(selection.showAll, false);
      return { selectedName: "CU", latencyDisplay: "885 ms", lossDisplay: "3.4%" };
    },
  });
  let tree;
  await act(() => { tree = create(jsx(glassNodeTable, {
    nodes: [nodes[0]], settings: { showCarrierPing: true }, pingEnabled: true,
  })); });
  assert.equal(tree.root.findByType("a").props.href, "/instance/green");
  const out = JSON.stringify(tree.toJSON());
  assert.match(out, /Latency · 1h avg/);
  assert.match(out, /"CU"/);
  assert.match(out, /"885 ms"/);
  assert.match(out, /"3.4%"/);
  assert.doesNotMatch(out, /"CT"|"CM"/);
  await act(() => tree.unmount());
});

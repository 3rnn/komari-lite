import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { act, create, type ReactTestRenderer } from "react-test-renderer";
import { AIAdvanced } from "../src/components/admin/AIMonitoring.tsx";

test("usage table sorts numeric calls, tokens and costs in both directions", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  let view!: ReactTestRenderer;
  const rows = [
    { model: "ten", calls: 10, total_tokens: 20, api_equivalent_cost_usd: 2 },
    { model: "two", calls: 2, total_tokens: 100, api_equivalent_cost_usd: 10 },
    { model: "hundred", calls: 100, total_tokens: 3, api_equivalent_cost_usd: 0.5 },
  ];
  await act(async () => { view = create(React.createElement(AIAdvanced, { timezone: "UTC", data: { models: rows }, children: null })); });
  const names = () => view.root.findByType("tbody").findAllByType("tr").map(row => row.findAllByType("td")[0].children.join(""));
  const click = async (label: string) => { await act(async () => { view.root.findAllByType("button").find(button => button.children.join("") === label)!.props.onClick(); }); };
  assert.deepEqual(names(), ["hundred", "ten", "two"]);
  await click("Calls"); assert.deepEqual(names(), ["two", "ten", "hundred"]);
  await click("Calls"); assert.deepEqual(names(), ["hundred", "ten", "two"]);
  await click("Tokens"); assert.deepEqual(names(), ["two", "ten", "hundred"]);
  await click("Tokens"); assert.deepEqual(names(), ["hundred", "ten", "two"]);
  await click("API-Equivalent Estimate"); assert.deepEqual(names(), ["two", "ten", "hundred"]);
  await click("API-Equivalent Estimate"); assert.deepEqual(names(), ["hundred", "ten", "two"]);
  await act(async () => view.unmount());
});

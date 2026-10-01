import assert from "node:assert/strict";
import test from "node:test";
import { resolveNodePingSelection, selectPingTaskData } from "../script/glass-ping-view.mjs";

const tasks = [
  { id: 1, name: "CT", weight: 1 },
  { id: 2, name: "CU", weight: 2 },
  { id: 3, name: "CM", weight: 3 },
];
const sample = (task_id, value, second) => ({
  task_id,
  value,
  time: `2026-09-30T04:00:${String(second).padStart(2, "0")}Z`,
});
const makeHistory = (records) => records.map((record) => ({ time: record.time, latency: record.value < 0 ? null : record.value, loss: record.value < 0 ? 100 : 0 }));

test("separate mode displays each assigned task without cross-task means", () => {
  const view = selectPingTaskData(tasks, [sample(1, 30, 1), sample(1, -1, 2), sample(2, 80, 3)], true, 0, makeHistory);
  assert.deepEqual(view.rows.map((row) => [row.id, row.name, row.avgLatency, row.loss]), [
    [1, "CT", 30, 50], [2, "CU", 80, 0], [3, "CM", null, null],
  ]);
  assert.equal(view.selected, null);
  assert.deepEqual(view.rows[0].history.map((point) => point.latency), [30, null]);
});

test("single mode selects the configured task ID, never an aggregate", () => {
  const view = selectPingTaskData(tasks, [sample(1, 30, 1), sample(2, 80, 3)], false, 2, makeHistory);
  assert.equal(view.selected.id, 2);
  assert.equal(view.selected.avgLatency, 80);
  assert.equal(view.selected.loss, 0);
  assert.deepEqual(view.selected.history.map((point) => point.latency), [80]);
  assert.equal(view.rows.length, 0);
});

test("a deleted or unassigned preferred task falls back to the first assigned task even without samples", () => {
  const view = selectPingTaskData(tasks, [sample(2, 80, 3), sample(99, 1, 4)], false, 99, makeHistory);
  assert.equal(view.selected.id, 1);
  assert.equal(view.selected.avgLatency, null);
  assert.equal(view.selected.loss, null);
});

test("all lost packets show 100% loss but no invented latency", () => {
  const view = selectPingTaskData(tasks, [sample(3, -1, 1), sample(3, -1, 2)], false, 3, makeHistory);
  assert.equal(view.selected.avgLatency, null);
  assert.equal(view.selected.loss, 100);
  assert.equal(view.selected.history.length, 2);
});

test("fallback respects backend task order when public tasks omit weight", () => {
  // Public RPC preserves database weight order but does not serialize weight.
  const ordered = [{ id: 12, name: "CU" }, { id: 3, name: "CT" }];
  const view = selectPingTaskData(ordered, [sample(12, 45, 1), sample(3, 20, 2)], false, 0, makeHistory);
  assert.equal(view.selected.id, 12);
  assert.deepEqual(selectPingTaskData(ordered, [], true, 0, makeHistory).rows.map((row) => row.id), [12, 3]);
});

test("same-named tasks remain distinct by task ID", () => {
  const view = selectPingTaskData([{ id: 4, name: "CT" }, { id: 5, name: "CT" }], [sample(4, 20, 1), sample(5, 90, 2)], true, 0, makeHistory);
  assert.deepEqual(view.rows.map((row) => [row.id, row.avgLatency]), [[4, 20], [5, 90]]);
});

test("a positive server display ID overrides global multi-task mode", () => {
  const selection = resolveNodePingSelection(true, { preferredId: 1 }, 2);
  assert.deepEqual(selection, { showAll: false, preferredId: 2 });
  const view = selectPingTaskData(tasks, [sample(1, 30, 1), sample(2, 80, 3)], selection.showAll, selection.preferredId, makeHistory);
  assert.equal(view.selected.id, 2);
  assert.equal(view.selected.avgLatency, 80);
  assert.deepEqual(view.rows, []);
});

test("zero server display ID retains the theme's multi-task or single-task setting", () => {
  assert.deepEqual(resolveNodePingSelection(true, { preferredId: 1 }, 0), { showAll: true, preferredId: 1 });
  assert.deepEqual(resolveNodePingSelection(false, { preferredId: 2 }, 0), { showAll: false, preferredId: 2 });
});

test("an unassigned server display ID falls back to first assigned task, not record averages", () => {
  const selection = resolveNodePingSelection(true, { preferredId: 1 }, 99);
  const view = selectPingTaskData(tasks, [sample(2, 80, 3), sample(99, 1, 4)], selection.showAll, selection.preferredId, makeHistory);
  assert.equal(view.selected.id, 1);
  assert.equal(view.selected.avgLatency, null);
  assert.deepEqual(view.rows, []);
});

test("server display override preserves carrier probe selection", () => {
  const selection = resolveNodePingSelection(true, { preferredId: 1, telecom: "CT", unicom: "CU", mobile: "CM" }, 2);
  assert.deepEqual(selection, { showAll: false, preferredId: 2, telecom: "CT", unicom: "CU", mobile: "CM" });
});

import assert from "node:assert/strict";
import test from "node:test";

import { getDatabaseRuntimeHealth } from "../src/lib/databaseRuntime.ts";

const normal = {
  total: 21,
  last_step_at: "2026-07-26T08:00:00Z",
  checkpoint_pending: false,
  consecutive_checkpoint_failures: 0,
  consecutive_cycle_failures: 0,
};

test("classifies database runtime checkpoint failures", () => {
  assert.equal(getDatabaseRuntimeHealth(normal), "healthy");
  assert.equal(
    getDatabaseRuntimeHealth({
      ...normal,
      checkpoint_pending: true,
      consecutive_checkpoint_failures: 1,
    }),
    "pending",
  );
  assert.equal(
    getDatabaseRuntimeHealth({
      ...normal,
      checkpoint_pending: true,
      consecutive_checkpoint_failures: 2,
    }),
    "pending",
  );
  assert.equal(
    getDatabaseRuntimeHealth({
      ...normal,
      checkpoint_pending: true,
      consecutive_checkpoint_failures: 3,
    }),
    "attention",
  );
});

test("classifies idle and repeated failed compaction cycles", () => {
  assert.equal(
    getDatabaseRuntimeHealth({ ...normal, total: 0, last_step_at: null }),
    "idle",
  );
  assert.equal(
    getDatabaseRuntimeHealth({ ...normal, consecutive_cycle_failures: 1 }),
    "pending",
  );
  assert.equal(
    getDatabaseRuntimeHealth({ ...normal, consecutive_cycle_failures: 2 }),
    "attention",
  );
});

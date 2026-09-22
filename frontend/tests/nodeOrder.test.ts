import assert from "node:assert/strict";
import test from "node:test";

import { compareNodesByBackendOrder } from "../src/lib/nodeOrder.ts";

test("uses backend weight, creation time, and UUID order", () => {
  const nodes = [
    { uuid: "client-c", weight: 20, created_at: "2026-07-23T00:00:00Z" },
    { uuid: "client-b", weight: 10, created_at: "2026-07-23T00:01:00Z" },
    { uuid: "client-a", weight: 10, created_at: "2026-07-23T00:00:00Z" },
  ];

  assert.deepEqual(
    [...nodes].sort(compareNodesByBackendOrder).map((node) => node.uuid),
    ["client-a", "client-b", "client-c"]
  );
});

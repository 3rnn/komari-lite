import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { mergeLatestStatus } from "../src/utils/liveData.ts";

test("maps RPC2 latest status into terminal live data", () => {
  const result = mergeLatestStatus({
    "node-a": {
      client: "node-a",
      online: true,
      cpu: 12.5,
      ram: 1024,
      net_in: 20,
      net_out: 10,
      net_total_up: 300,
      net_total_down: 400,
      time: "2026-07-31T08:00:00Z",
    },
    "node-b": { client: "node-b", online: false },
  }, null);

  assert.deepEqual(result.data.online, ["node-a"]);
  assert.equal(result.data.data["node-a"].cpu.usage, 12.5);
  assert.equal(result.data.data["node-a"].network.totalUp, 300);
  assert.equal(result.data.data["node-a"].network.totalDown, 400);
});


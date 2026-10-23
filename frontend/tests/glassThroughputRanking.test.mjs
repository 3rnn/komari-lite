import assert from "node:assert/strict";
import test from "node:test";
import * as model from "../script/glass-network-summary.mjs";

const now = Date.parse("2026-10-23T12:00:00Z");
const sample = (uuid, speed, age = 0, extra = {}) => ({ uuid, name: uuid, net_out: speed, net_in: 0, online: true, updated_at_live: new Date(now - age).toISOString(), ...extra });
test("full ranking retains every valid sample, stable ties, and earliest expiry beyond the winner", () => {
  assert.equal(typeof model.rankGlassThroughput, "function");
  const nodes = [sample("z", 5), sample("a", 5), sample("slow", 0, 59_000), sample("offline", 99, 0, { online: false }), sample("hidden", 99, 0, { hidden: true }), sample("bad", Infinity), sample("negative", -1), sample("stale", 99, 60_001), sample("future", 99, -1)];
  const ranked = model.rankGlassThroughput(nodes, now);
  assert.deepEqual(ranked.map(n => n.uuid), ["a", "z", "slow"]);
  assert.equal(Math.min(...ranked.map(n => n.expiresAt)), now + 1001);
  assert.deepEqual(model.selectGlassTopThroughput(nodes, now), ranked[0]);
  assert.equal(nodes[0].uuid, "z");
  assert.deepEqual(model.rankGlassThroughput(undefined, now), []);
  assert.equal(model.selectGlassTopThroughput([], now), null);
});

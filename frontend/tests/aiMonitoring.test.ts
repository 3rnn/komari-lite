import assert from "node:assert/strict";
import test from "node:test";
import { hermesInstallArgs, hermesProfileEnabled, aiNumber, aiRange, canonicalTokens, taskLabel, sortUsage, aiCustomRange, aiISOSeconds, aiTrendTick, aiTrendTime, aiTrendUnit } from "../src/utils/aiMonitoring.ts";

test("Hermes opt-in is false for legacy profiles and Windows never generates an enabled command", () => {
  assert.equal(hermesProfileEnabled({}), false);
  assert.equal(hermesProfileEnabled({ enable_hermes_ai: true }), true);
  assert.deepEqual(hermesInstallArgs("linux", false), []);
  assert.deepEqual(hermesInstallArgs("linux", true), ["--enable-hermes-ai"]);
  assert.throws(() => hermesInstallArgs("windows", true), /unsupported on Windows/);
});

test("missing throughput stays N/A, reasoning is not double-counted, unknown tasks are explicit", () => {
  assert.equal(aiNumber(null), "N/A");
  assert.equal(aiNumber(undefined), "N/A");
  assert.equal(aiNumber(0), "0");
  assert.equal(canonicalTokens({ input_tokens: 10, cache_read_tokens: 20, cache_write_tokens: 3, output_tokens: 7, reasoning_tokens: 4 }), 40);
  assert.equal(taskLabel(""), "Unknown task");
  assert.equal(taskLabel("unknown"), "Unknown task");
  assert.deepEqual(aiRange("1H", 10000), { from: 6400, to: 10000 });
  assert.deepEqual(sortUsage([{ calls: 3 }, { calls: 9 }], "calls", false).map(x => x.calls), [9, 3]);
});
test("custom timestamps require explicit offsets and an increasing bounded range", () => {
 assert.equal(aiISOSeconds("2026-10-09T08:00:00+08:00"),aiISOSeconds("2026-10-09T00:00:00Z"));
 assert.throws(()=>aiISOSeconds("2026-10-09T08:00:00"),/offset/);
 assert.throws(()=>aiCustomRange("2026-10-09T00:00:00Z","2026-10-08T00:00:00Z"),/increasing/);
 assert.throws(()=>aiCustomRange("2026-09-01T00:00:00Z","2026-10-09T00:00:00Z"),/30 days/);
});
test("trend axes use compact units and readable time ticks", () => {
 assert.equal(aiTrendTick(1250000,"total_tokens"),"1.25M");
 assert.equal(aiTrendTick(1530,"calls"),"1.53K");
 assert.equal(aiTrendTick(18.327304,"api_equivalent_cost_usd"),"$18.33");
 assert.equal(aiTrendUnit("total_tokens"),"Tokens");
 assert.equal(aiTrendUnit("api_equivalent_cost_usd"),"USD");
 assert.equal(aiTrendTime(1791574080,"UTC",24*3600),"19:28");
 assert.equal(aiTrendTime(1791574080,"UTC",7*86400),"09 Oct");
 assert.equal(aiTrendTime(1791574080,"UTC",30*86400),"09 Oct 26");
});

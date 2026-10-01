import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const runner = readFileSync(new URL("../script/run-tests.mjs", import.meta.url), "utf8");

test("frontend test runner includes Glass .mjs regressions", () => {
  assert.match(runner, /\.test\.mjs/);
});

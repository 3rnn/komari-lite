import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync("src/components/PriceTags.tsx", "utf8");

test("billing tags treat a missing expiry as long term", () => {
  assert.match(source, /expired_at === null \|\| expired_at === undefined/);
  assert.match(source, /expirationDays === null \|\| expirationDays > 36500/);
  assert.match(source, /t\("common\.long_term"\)/);
});

test("billing prices do not expose floating point noise", () => {
  assert.match(source, /price\.toFixed\(2\)/);
  assert.match(source, /\$\{displayPrice\}/);
});

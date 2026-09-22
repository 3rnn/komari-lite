import assert from "node:assert/strict";
import test from "node:test";

import { publicVersion } from "../src/utils/version.ts";

test("shows stable Agent versions with three or four numeric segments", () => {
  assert.equal(publicVersion("2.1.11+abcdefg"), "2.1.11");
  assert.equal(publicVersion("2.1.11.1+abcdefg"), "2.1.11.1");
});

test("shows snapshot Agent versions with three or four numeric segments", () => {
  assert.equal(publicVersion("Snapshot-2.1.11-2607281423"), "2.1.11 Snapshot");
  assert.equal(publicVersion("Snapshot-2.1.11.0-2607281423"), "2.1.11.0 Snapshot");
});

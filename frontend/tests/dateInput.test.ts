import assert from "node:assert/strict";
import test from "node:test";

import {
  dateInputToISOString,
  timestampToDateInput,
} from "../src/lib/dateInput.ts";

const originalTimezone = process.env.TZ;

test.after(() => {
  if (originalTimezone === undefined) {
    delete process.env.TZ;
  } else {
    process.env.TZ = originalTimezone;
  }
});

test("keeps a Beijing calendar date stable after saving and reopening", () => {
  process.env.TZ = "Asia/Shanghai";

  const saved = dateInputToISOString("2026-08-01");
  assert.equal(saved, "2026-07-31T16:00:00.000Z");
  assert.equal(timestampToDateInput(saved!), "2026-08-01");
});

test("does not subtract another day when the unchanged date is saved again", () => {
  process.env.TZ = "Asia/Shanghai";

  const firstSave = dateInputToISOString("2026-08-01");
  const reopened = timestampToDateInput(firstSave!);
  const secondSave = dateInputToISOString(reopened);

  assert.equal(secondSave, firstSave);
});

test("also round trips dates in a negative UTC offset", () => {
  process.env.TZ = "America/Los_Angeles";

  const saved = dateInputToISOString("2026-08-01");
  assert.equal(saved, "2026-08-01T07:00:00.000Z");
  assert.equal(timestampToDateInput(saved!), "2026-08-01");
});

test("handles empty and invalid values", () => {
  assert.equal(dateInputToISOString(""), null);
  assert.equal(timestampToDateInput("not-a-date"), "");
  assert.throws(() => dateInputToISOString("2026-02-30"));
});

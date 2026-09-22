import assert from "node:assert/strict";
import test from "node:test";

import {
  currencyForDisplay,
  currencyForStorage,
} from "../src/lib/currency.ts";

test("displays stored CAD as the Canadian dollar symbol", () => {
  assert.equal(currencyForDisplay("CAD"), "C$");
  assert.equal(currencyForDisplay("cad"), "C$");
  assert.equal(currencyForDisplay("CA$"), "C$");
});

test("stores Canadian dollar display values as CAD", () => {
  assert.equal(currencyForStorage("C$"), "CAD");
  assert.equal(currencyForStorage("CA$"), "CAD");
  assert.equal(currencyForStorage(" cad "), "CAD");
});

test("keeps other custom currencies unchanged", () => {
  assert.equal(currencyForDisplay("AUD"), "AUD");
  assert.equal(currencyForStorage(" kr "), "kr");
});

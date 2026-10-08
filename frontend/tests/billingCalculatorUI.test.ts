import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const billingSource = readFileSync("src/pages/admin/index.tsx", "utf8");
const calculatorSource = readFileSync("src/components/admin/RemainingValueCalculator.tsx", "utf8");

test("Billing keeps calculator inputs local while using the unsaved billing draft", () => {
  assert.match(billingSource, /const \[priceInput, setPriceInput\]/);
  assert.match(billingSource, /const \[expiredAtInput, setExpiredAtInput\]/);
  assert.match(billingSource, /<RemainingValueCalculator[\s\S]*price=\{priceInput\}[\s\S]*billingCycle=\{billingCycle\}[\s\S]*expiredAt=\{expiredAtInput\}/);
  assert.match(billingSource, /const priceValue = priceInput \|\| "0"/);
  assert.match(billingSource, /expired_at: expiredAt/);
  assert.doesNotMatch(billingSource, /transactionDate|cnyPerUnit|adjustmentMode|adjustmentValue/);
});

test("calculator uses themed Select controls, automatic-rate controls, and no implementation-only help copy", () => {
  assert.match(calculatorSource, /import \{[\s\S]*Select,[\s\S]*\} from "@radix-ui\/themes"/);
  assert.match(calculatorSource, /<Select\.Root[\s\S]*<Select\.Trigger[\s\S]*<Select\.Content[\s\S]*<Select\.Item value="add"/);
  assert.match(calculatorSource, /RefreshCw/);
  assert.match(calculatorSource, /fetchCnyExchangeRate/);
  assert.match(calculatorSource, /ExchangeRate-API/);
  assert.match(calculatorSource, /rateStatus/);
  assert.match(calculatorSource, /grid-cols-1[\s\S]*sm:grid-cols-2/);
  assert.doesNotMatch(calculatorSource, /<select|calculatorHelp|Uses the VPS-JSQ day-count formula|Calculator-only values are not saved with Billing/);
});

test("calculator matches VPS-JSQ controls without requiring a current cycle start", () => {
  assert.match(calculatorSource, /Remaining Value Calculator/);
  assert.match(calculatorSource, /Transaction date/);
  assert.match(calculatorSource, /CNY per currency unit/);
  assert.match(calculatorSource, /Fixed premium/);
  assert.match(calculatorSource, /Fixed discount/);
  assert.match(calculatorSource, /Target price/);
  assert.match(calculatorSource, /Remaining percentage/);
  assert.match(calculatorSource, /Final resale price/);
  assert.match(calculatorSource, /Copy resale listing/);
  assert.match(calculatorSource, /writeClipboardText/);
  assert.doesNotMatch(calculatorSource, /cycleStartedAt|Current cycle start|<option value="percent"/);
});

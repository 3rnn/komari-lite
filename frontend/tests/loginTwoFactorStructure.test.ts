import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript";

// Inspect the login page with a real TSX parser rather than matching raw strings.
// Previously the one-time-code field and submit button were nested under !require2FA.
// With 2FA enabled, only a status line was visible and the user could not submit a code.
const LOGIN_SOURCE = readFileSync("src/components/Login.tsx", "utf8");
const sourceFile = ts.createSourceFile(
  "Login.tsx",
  LOGIN_SOURCE,
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.TSX,
);

type Conditional = { condition: string; whenTrue: string };

function collectConditionals(): Conditional[] {
  const found: Conditional[] = [];
  const visit = (node: ts.Node) => {
    if (ts.isConditionalExpression(node)) {
      found.push({
        condition: node.condition.getText(sourceFile),
        whenTrue: node.whenTrue.getText(sourceFile),
      });
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  return found;
}

const conditionals = collectConditionals();
// Keys unique to the 2FA step (login.two_factor_account is only a status line).
const twoFactorKeys = /login\.two_factor(?!_account)/;
const credentialBranches = conditionals.filter((entry) => /!\s*require2FA/.test(entry.condition));

test("login page has a branch for the non-2FA step", () => {
  assert.ok(credentialBranches.length > 0, "Missing !require2FA branch in the login page");
});

test("the 2FA UI is not nested in the non-2FA branch", () => {
  for (const branch of credentialBranches) {
    assert.doesNotMatch(
      branch.whenTrue,
      twoFactorKeys,
      "The 2FA UI is inside the non-2FA branch, so users cannot enter a code",
    );
  }
});

test("the 2FA step renders its one-time-code field", () => {
  const step = conditionals.find(
    (entry) => /require2FA/.test(entry.condition) && !/!/.test(entry.condition) && twoFactorKeys.test(entry.whenTrue),
  );
  assert.ok(step, "Missing branch for the one-time-code input when require2FA is true");
});

test("the submit button remains available on the 2FA step", () => {
  for (const branch of credentialBranches) {
    assert.doesNotMatch(
      branch.whenTrue,
      /type="submit"/,
      "Submit is inside the non-2FA branch, so users cannot send a code",
    );
  }
});

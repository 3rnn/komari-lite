import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript";

// 这一组断言用真正的 TSX 解析器检查登录页的渲染结构，而不是搜字符串：
// 曾经出现过「动态口令输入框与提交按钮被写进 `!require2FA` 分支」的回归——
// 账号一开 2FA，登录页就只剩「正在验证账号 xxx」一行，用户根本没法输入口令。
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
// 动态口令界面专有的 i18n key（`login.two_factor_account` 是 2FA 状态行，不算）
const twoFactorKeys = /login\.two_factor(?!_account)/;
const credentialBranches = conditionals.filter((entry) => /!\s*require2FA/.test(entry.condition));

test("登录页存在「未要求 2FA」的条件分支", () => {
  assert.ok(credentialBranches.length > 0, "没找到 !require2FA 分支，登录页结构可能被改写");
});

test("动态口令界面不能落在「未要求 2FA」的分支里", () => {
  for (const branch of credentialBranches) {
    assert.doesNotMatch(
      branch.whenTrue,
      twoFactorKeys,
      "动态口令界面被写进了「未要求 2FA」的分支：开启 2FA 后用户只能看到提示，无法输入口令",
    );
  }
});

test("要求 2FA 时必须渲染动态口令输入界面", () => {
  const step = conditionals.find(
    (entry) => /require2FA/.test(entry.condition) && !/!/.test(entry.condition) && twoFactorKeys.test(entry.whenTrue),
  );
  assert.ok(step, "找不到「require2FA 为真时渲染动态口令输入」的分支");
});

test("提交按钮不能被「未要求 2FA」的分支包住", () => {
  for (const branch of credentialBranches) {
    assert.doesNotMatch(
      branch.whenTrue,
      /type="submit"/,
      "提交按钮被写进「未要求 2FA」的分支：开启 2FA 后无法提交口令",
    );
  }
});

import assert from "node:assert/strict";
import test from "node:test";
import {
  initialLoginStepState,
  nextLoginStepState,
} from "../src/utils/loginFlow.ts";

// 服务端「需要动态口令」只是一个流程信号：界面要切到第二步，且不能把它当红色报错显示，
// 否则用户会以为账号/密码有问题（实际截图里就出现过红色的 "2FA code is required"）。
test("需要动态口令时：切到第二步且不显示报错", () => {
  const next = nextLoginStepState(initialLoginStepState, {
    ok: false,
    requiresTwoFactor: true,
    message: "2FA code is required",
  });
  assert.equal(next.require2FA, true);
  assert.equal(next.errorMsg, "");
});

// 口令错误必须留在第二步，并把服务端文案交给界面（界面会换成更友好的提示）。
test("动态口令错误时：保持在第二步并保留错误文案", () => {
  const twoFactorStep = { require2FA: true, errorMsg: "" };
  const next = nextLoginStepState(twoFactorStep, {
    ok: false,
    requiresTwoFactor: false,
    message: "Invalid 2FA code",
  });
  assert.equal(next.require2FA, true);
  assert.equal(next.errorMsg, "Invalid 2FA code");
});

// 账号密码错误：还在第一步，显示服务端文案。
test("账号密码错误时：停留在第一步并显示报错", () => {
  const next = nextLoginStepState(initialLoginStepState, {
    ok: false,
    requiresTwoFactor: false,
    message: "Invalid credentials",
  });
  assert.equal(next.require2FA, false);
  assert.equal(next.errorMsg, "Invalid credentials");
});

test("登录成功时：清空报错", () => {
  const next = nextLoginStepState({ require2FA: true, errorMsg: "旧报错" }, { ok: true });
  assert.equal(next.errorMsg, "");
});

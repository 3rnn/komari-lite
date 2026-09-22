import assert from "node:assert/strict";
import test from "node:test";

import {
  fetchAccount,
  isAdminNodeBootstrapLoading,
  resolveAdminAuthView,
  submitPasswordLogin,
} from "../src/utils/adminAuth.ts";

test("登录切换期间保持后台启动加载状态", () => {
  assert.equal(isAdminNodeBootstrapLoading(true, null, null), true);
  assert.equal(isAdminNodeBootstrapLoading(false, "account-1", null), true);
  assert.equal(
    isAdminNodeBootstrapLoading(false, "account-1", "__preauthenticated__", true),
    false,
  );
  assert.equal(
    isAdminNodeBootstrapLoading(false, "account-1", "account-1"),
    false,
  );
});

test("未登录时只进入登录视图", () => {
  assert.equal(
    resolveAdminAuthView({
      account: { logged_in: false },
      loading: false,
      error: null,
    }),
    "login",
  );
});

test("已登录后才进入后台视图", () => {
  assert.equal(
    resolveAdminAuthView({
      account: { logged_in: true },
      loading: false,
      error: null,
    }),
    "admin",
  );
});

test("账户接口失败时进入可重试错误视图", async () => {
  await assert.rejects(
    () => fetchAccount(async () => new Response(null, { status: 503 })),
    /Failed to fetch account data \(503\)/,
  );
  assert.equal(
    resolveAdminAuthView({
      account: null,
      loading: false,
      error: new Error("request failed"),
    }),
    "error",
  );
});

test("登录成功后刷新外层账户信息", async () => {
  let refreshCount = 0;

  const result = await submitPasswordLogin({
    username: "admin",
    password: "secret",
    fetcher: async (input, init) => {
      assert.equal(input, "/api/login");
      assert.equal(init?.method, "POST");
      assert.deepEqual(JSON.parse(String(init?.body)), {
        username: "admin",
        password: "secret",
      });
      return new Response("{}", {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    },
    refreshAccount: async () => {
      refreshCount += 1;
    },
  });

  assert.deepEqual(result, { ok: true });
  assert.equal(refreshCount, 1);
});

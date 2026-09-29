import assert from "node:assert/strict";
import test from "node:test";

import {
  fetchAccount,
  isAdminNodeBootstrapLoading,
  resolveAdminAuthView,
  submitPasswordLogin,
} from "../src/utils/adminAuth.ts";

test("loading remains active while switching to the admin login", () => {
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

test("unauthenticated users only see the login view", () => {
  assert.equal(
    resolveAdminAuthView({
      account: { logged_in: false },
      loading: false,
      error: null,
    }),
    "login",
  );
});

test("authenticated users enter the admin view", () => {
  assert.equal(
    resolveAdminAuthView({
      account: { logged_in: true },
      loading: false,
      error: null,
    }),
    "admin",
  );
});

test("account API errors enter a retryable error view", async () => {
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

test("a successful login refreshes the outer account", async () => {
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

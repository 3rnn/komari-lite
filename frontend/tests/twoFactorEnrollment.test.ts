import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { createRequire } from "node:module";
import React from "react";
import { act, create } from "react-test-renderer";
import ts from "typescript";

const account = readFileSync("src/pages/admin/account.tsx", "utf8");

test("factor replacement collects existing proof before requesting a new QR", () => {
  assert.match(account, /<TwoFactorDisabled replacing/);
  assert.match(account, /const \[existingCode, setExistingCode\]/);
  assert.match(account, /"X-2FA-Code": existingCode/);
  assert.match(account, /replacing && !existingCode/);
  assert.match(account, /Current authenticator code/);
  assert.match(account, /Replace authenticator/);
});

test("enrollment discards QR blobs and clears codes on dialog close", () => {
  assert.match(account, /URL\.revokeObjectURL\(/);
  assert.match(account, /setCode\(""\)/);
  assert.match(account, /setExistingCode\(""\)/);
  assert.match(account, /AbortController/);
  assert.match(account, /disabled=\{saving \|\| isLoading \|\| !qrcode\}/);
});

test("replacement UI sends old proof, then confirms the new factor and clears the QR", async () => {
  // Run the actual page hooks and event handlers; only surrounding UI/context
  // modules are replaced with host elements to avoid requiring a browser DOM.
  const require = createRequire(import.meta.url);
  const translate = (key: string) => key;
  let refreshed = 0;
  const errors: string[] = [];
  const ui = {
    Button: "button", Flex: "div", Skeleton: "div",
    TextField: { Root: "input" },
    Dialog: { Root: "dialog-root", Trigger: "div", Title: "h2", Description: "p" },
  };
  const module = { exports: {} as { default: React.ComponentType } };
  const compiled = ts.transpileModule(account, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true },
    fileName: "account.tsx",
  }).outputText;
  const mockedRequire = (name: string) => {
    if (name === "@radix-ui/themes") return ui;
    if (name === "react-i18next") return { useTranslation: () => ({ t: translate }) };
    if (name === "sonner") return { toast: { error: (message: string) => errors.push(message), success: () => {} } };
    if (name === "@/contexts/AccountContext") return {
      useAccount: () => ({ account: { "2fa_enabled": true }, refresh: () => { refreshed++; } }),
    };
    if (name === "@/components/admin/SettingCard") return { SettingCardLabel: "div" };
    if (name === "@/components/admin/AdminPageTitle") return { AdminSectionTitle: "div" };
    if (name.startsWith("@/")) return { __esModule: true, default: "div" };
    return require(name);
  };
  new Function("require", "module", "exports", compiled)(mockedRequire, module, module.exports);
  const requests: { url: string; options?: RequestInit }[] = [];
  const previousFetch = globalThis.fetch;
  const previousAct = (globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT;
  (globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT = true;
  globalThis.fetch = (async (url: string, options?: RequestInit) => {
    requests.push({ url, options });
    return { ok: true, blob: async () => new Blob(["test PNG"]), json: async () => ({}) } as Response;
  }) as typeof fetch;
  let renderer: ReturnType<typeof create> | undefined;
  try {
    await act(async () => { renderer = create(React.createElement(module.exports.default)); });
    const root = renderer!.root;
    const replacement = () => root.findAllByType("dialog-root" as React.ElementType).find((node) => node.findByType("h2").children.includes("Replace authenticator"))!;
    await act(async () => { replacement().props.onOpenChange(true); });
    assert.equal(requests.length, 0, "replacement must not generate before proving the old factor");
    await act(async () => {
      replacement().findByProps({ id: "existing_2fa_code" }).props.onChange({ target: { value: "012345" } });
    });
    await act(async () => { replacement().findByType("form").props.onSubmit({ preventDefault() {} }); });
    assert.equal(requests.length, 1);
    assert.equal(requests[0].url, "/api/admin/2fa/generate");
    assert.deepEqual(requests[0].options?.headers, { "X-2FA-Code": "012345" });
    assert.equal(requests[0].options?.cache, "no-store");
    assert.equal(replacement().findAllByProps({ id: "existing_2fa_code" }).length, 0);
    assert.match(replacement().findByType("img").props.src, /^blob:/);
    const qrURL = replacement().findByType("img").props.src;
    await act(async () => {
      replacement().findByProps({ id: "replace_2fa_code" }).props.onChange({ target: { value: "065432" } });
    });
    await act(async () => { replacement().findByType("form").props.onSubmit({ preventDefault() {} }); });
    assert.equal(requests[1].url, "/api/admin/2fa/enable?code=065432");
    assert.equal(requests[1].options?.method, "POST");
    assert.equal(refreshed, 1);
    assert.equal(replacement().props.open, false);
    assert.deepEqual(errors, []);
    await assert.rejects(previousFetch(qrURL)); // The QR object URL was revoked on close.
  } finally {
    if (renderer) await act(async () => { renderer!.unmount(); });
    globalThis.fetch = previousFetch;
    (globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT = previousAct;
  }
});

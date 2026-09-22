import assert from "node:assert/strict";
import test from "node:test";

import { writeClipboardText } from "../src/utils/clipboard.ts";

function fallbackEnvironment(copyResult = true) {
  let appendedValue = "";
  let removed = false;
  const textarea = {
    value: "",
    style: {},
    setAttribute() {},
    focus() {},
    select() {},
    setSelectionRange() {},
    remove() { removed = true; },
  };
  const documentObject = {
    body: {
      appendChild(node: typeof textarea) { appendedValue = node.value; },
    },
    createElement() { return textarea; },
    execCommand(command: string) {
      assert.equal(command, "copy");
      return copyResult;
    },
  };
  return {
    environment: { document: documentObject as unknown as Document },
    state: () => ({ appendedValue, removed }),
  };
}

test("confirms the secure clipboard API only after reading back the same text", async () => {
  const writes: string[] = [];
  const result = await writeClipboardText("agent command", {
    navigator: {
      clipboard: {
        writeText: async (text: string) => { writes.push(text); },
        readText: async () => writes.at(-1) || "",
      },
    } as unknown as Navigator,
  });
  assert.deepEqual(writes, ["agent command"]);
  assert.deepEqual(result, { confirmed: true, method: "clipboard" });
});

test("prefers the confirmable Edge clipboard API over the legacy fallback", async () => {
  const fallback = fallbackEnvironment();
  let asyncWriteAttempted = false;
  const result = await writeClipboardText("agent command", {
    navigator: {
      clipboard: {
        writeText: async () => { asyncWriteAttempted = true; },
        readText: async () => "agent command",
      },
    } as unknown as Navigator,
    ...fallback.environment,
  });
  assert.deepEqual(result, { confirmed: true, method: "clipboard" });
  assert.deepEqual(fallback.state(), { appendedValue: "agent command", removed: true });
  assert.equal(asyncWriteAttempted, true);
});

test("does not report success when Edge resolves a write but read-back differs", async () => {
  const fallback = fallbackEnvironment();
  const result = await writeClipboardText("agent command", {
    navigator: {
      clipboard: {
        writeText: async () => {},
        readText: async () => "previous clipboard value",
      },
    } as unknown as Navigator,
    ...fallback.environment,
  });
  assert.deepEqual(result, { confirmed: false, method: "legacy" });
  assert.deepEqual(fallback.state(), { appendedValue: "agent command", removed: true });
});

test("does not mask a rejected Edge write when no fallback is available", async () => {
  await assert.rejects(
    () => writeClipboardText("agent command", {
      navigator: {
        clipboard: {
          writeText: async () => { throw new Error("permission denied"); },
          readText: async () => "previous clipboard value",
        },
      } as unknown as Navigator,
    }),
    /permission denied/,
  );
});

test("leaves an unreadable clipboard result unconfirmed", async () => {
  const result = await writeClipboardText("agent command", {
    navigator: {
      clipboard: {
        writeText: async () => {},
        readText: async () => { throw new Error("read denied"); },
      },
    } as unknown as Navigator,
  });
  assert.deepEqual(result, { confirmed: false, method: "clipboard" });
});

test("marks the legacy fallback as unconfirmed when the Clipboard API is unavailable", async () => {
  const fallback = fallbackEnvironment();
  const result = await writeClipboardText("agent command", fallback.environment);
  assert.deepEqual(result, { confirmed: false, method: "legacy" });
  assert.deepEqual(fallback.state(), { appendedValue: "agent command", removed: true });
});

test("reports a copy failure when neither clipboard path is available", async () => {
  const fallback = fallbackEnvironment(false);
  await assert.rejects(() => writeClipboardText("agent command", fallback.environment));
  assert.equal(fallback.state().removed, true);
});

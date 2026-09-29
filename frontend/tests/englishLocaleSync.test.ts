import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { execFileSync } from "node:child_process";
import test from "node:test";

const script = new URL("../script/i18n-sync.mjs", import.meta.url);

test("locale sync copies English strings into the legacy locale without AI", () => {
  const dir = mkdtempSync(join(tmpdir(), "komari-locale-sync-"));
  try {
    const english = { button: { save: "Save", cancel: "Cancel" } };
    writeFileSync(join(dir, "en.json"), JSON.stringify(english));
    writeFileSync(join(dir, "zh_CN.json"), JSON.stringify({ button: { save: "Old label" } }));
    execFileSync(process.execPath, [script.pathname, "--no-ai"], {
      env: { ...process.env, I18N_LOCALES_DIR: dir, I18N_SOURCE: "", OPENAI_API_KEY: "" },
      stdio: "pipe",
    });
    assert.deepEqual(JSON.parse(readFileSync(join(dir, "zh_CN.json"), "utf8")), english);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

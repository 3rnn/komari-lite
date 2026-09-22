import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { resolveI18nText } from "../src/utils/i18nText.ts";

const repositoryRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);

test("theme text resolves locale aliases before English fallback", () => {
  const text = {
    en: "English",
    zh_CN: "简体中文",
  };
  assert.equal(resolveI18nText(text, "zh-CN"), "简体中文");
  assert.equal(resolveI18nText(text, "zh_CN"), "简体中文");
  assert.equal(resolveI18nText(text, "fr-FR"), "English");
});

test("theme text skips empty translations and keeps legacy strings", () => {
  assert.equal(resolveI18nText("Legacy", "zh-CN"), "Legacy");
  assert.equal(
    resolveI18nText({ zh_CN: "", en: "English", ja: "日本語" }, "zh-CN"),
    "English",
  );
  assert.equal(resolveI18nText({ zh_CN: "", ja: "日本語" }, "fr"), "日本語");
  assert.equal(resolveI18nText({ en: "   " }, "en"), undefined);
});

function assertKeysSorted(value: unknown, location: string): void {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    return;
  }

  const record = value as Record<string, unknown>;
  const keys = Object.keys(record);
  const sortedKeys = [...keys].sort((left, right) =>
    left.localeCompare(right),
  );
  assert.deepEqual(keys, sortedKeys, location + " keys must be sorted");

  for (const key of keys) {
    assertKeysSorted(record[key], location + "." + key);
  }
}

test("source locale keys are sorted", () => {
  const filename = "zh_CN.json";
  const contents = JSON.parse(
    readFileSync(
      path.join(repositoryRoot, "src", "i18n", "locales", filename),
      "utf8",
    ),
  );
  assertKeysSorted(contents, filename);
});

test("i18n sync workflow has no candidate branch job", () => {
  const workflow = readFileSync(
    path.join(repositoryRoot, ".github", "workflows", "i18n-sync.yml"),
    "utf8",
  );

  assert.equal(workflow.includes("validate-candidate:"), false);
  assert.equal(workflow.includes("refs/heads/candidate/"), false);
  assert.match(workflow, /contents: write/);
  assert.match(workflow, /git push origin/);
});

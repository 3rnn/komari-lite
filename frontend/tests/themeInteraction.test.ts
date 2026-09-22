import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { AVAILABLE_LOCALES } from "./localeFixtures.ts";

const dialogSource = readFileSync("src/components/AppDialogContent.tsx", "utf8");
const uploadDialogSource = readFileSync("src/components/UploadDialog.tsx", "utf8");
const archiveUploadSource = readFileSync("src/utils/archiveUpload.ts", "utf8");
const locales = AVAILABLE_LOCALES.map((locale) =>
  JSON.parse(readFileSync(`src/i18n/locales/${locale}.json`, "utf8")),
);

test("backup and install dialogs use the shared staged upload UI", () => {
  assert.match(dialogSource, /containsDialogDescription/);
  assert.match(dialogSource, /disabledDescriptionProps/);
  assert.match(uploadDialogSource, /normalizedState\.indeterminate/);
  assert.match(uploadDialogSource, /km-upload-indeterminate-bar/);
  assert.match(uploadDialogSource, /disabled=\{uploadActive\}/);
  assert.match(archiveUploadSource, /await delay\(UPLOAD_FINAL_PROGRESS_VISIBLE_MS\)/);
});

test("all application dialogs use the shared description contract", () => {
  const sourceFiles: string[] = [];
  const visit = (directory: string) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const fullPath = path.join(directory, entry.name);
      if (entry.isDirectory()) visit(fullPath);
      else if (entry.name.endsWith(".tsx")) sourceFiles.push(fullPath);
    }
  };
  visit("src");

  const directDialogContentUsers = sourceFiles
    .filter((file) => readFileSync(file, "utf8").includes("<Dialog.Content"))
    .map((file) => file.replaceAll("\\", "/"));
  assert.deepEqual(directDialogContentUsers, ["src/components/AppDialogContent.tsx"]);
});

test("site backup and install staged-progress copy is present in every locale", () => {
  for (const locale of locales) {
    assert.equal(typeof locale.settings.site.phase_preparing, "string");
    assert.equal(typeof locale.settings.site.phase_uploading, "string");
    assert.equal(typeof locale.settings.site.phase_processing, "string");
    assert.equal(typeof locale.settings.site.phase_restarting, "string");
    assert.equal(typeof locale.settings.site.phase_completed, "string");
    assert.equal(typeof locale.settings.site.phase_non_cancelable, "string");
    assert.equal(typeof locale.install.phase_preparing, "string");
    assert.equal(typeof locale.install.phase_uploading, "string");
    assert.equal(typeof locale.install.phase_processing, "string");
    assert.equal(typeof locale.install.phase_restarting, "string");
    assert.equal(typeof locale.install.phase_completed, "string");
    assert.equal(typeof locale.install.phase_non_cancelable, "string");
    assert.equal(typeof locale.install.phase_redirecting, "string");
    assert.equal(typeof locale.install.phase_redirect_countdown, "string");
  }
});

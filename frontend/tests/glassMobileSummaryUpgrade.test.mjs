import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { cpSync, copyFileSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
const repo = new URL("../..", import.meta.url).pathname;
const source = join(repo, "backend/web/public/bundledThemes/Glass");
const hashes = new Map([
  ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
  ["dist/index.html", "7923290bb879eb775df7769d1b193652a6587a079a4dd373e2f4bd5607c6d18d"],
  ["dist/_next/static/chunks/3859ru-fa8e62f6.js", "fa8e62f6e5e44480f37a2de0389d5bc6ae93cc56cfa835c03292c7250114b06a"],
  ["dist/glass-visual-8af54d66.css", "8af54d66f5eb4a5bd76e839f2ef8ae627744471e495401a1290f953b9b158115"],
]);
function fixture(t) {
  const parent = mkdtempSync(join(tmpdir(), "glass-rank-upgrade-"));
  t.after(() => rmSync(parent, { recursive: true, force: true }));
  const root = join(parent, "Glass");
  cpSync(source, root, { recursive: true });
  copyFileSync(join(repo, "frontend/tests/fixtures/glass-58d6998/index.html.fixture"), join(root, "dist/index.html"));
  copyFileSync(join(repo, "frontend/tests/fixtures/glass-58d6998/komari-theme.json"), join(root, "komari-theme.json"));
  for (const [name, hash] of hashes) assert.equal(createHash("sha256").update(readFileSync(join(root, name))).digest("hex"), hash, `immutable installed ${name}`);
  writeFileSync(join(root, "user-note.txt"), "preserve");
  const run = apply => spawnSync(process.execPath, [join(repo, "frontend/script/upgrade-glass-theme.mjs"), "--theme-dir", root, ...(apply ? ["--apply"] : [])], { encoding: "utf8" });
  return { parent, root, run };
}
test("exact 58d6998 installed ranking accepts a dry run and migrates to mobile summary with intact backup", t => {
  const { parent, root, run } = fixture(t);
  const dry = run(false);
  assert.equal(dry.status, 0, dry.stderr);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  const result = run(true);
  assert.equal(result.status, 0, result.stderr);
  const backup = JSON.parse(result.stdout.trim().split("\n").at(-1)).backup;
  for (const [name, hash] of hashes) assert.equal(createHash("sha256").update(readFileSync(join(backup, name))).digest("hex"), hash);
  assert.deepEqual(readFileSync(join(root, "dist/index.html")), readFileSync(join(source, "dist/index.html")));
  assert.equal(readFileSync(join(root, "user-note.txt"), "utf8"), "preserve");
});
for (const name of hashes.keys()) test(`modified 58d6998 ${name} is rejected without any migration writes`, t => {
  const { parent, root, run } = fixture(t);
  writeFileSync(join(root, name), "customized");
  const result = run(true);
  assert.notEqual(result.status, 0);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.equal(readFileSync(join(root, name), "utf8"), "customized");
});

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
  ["dist/index.html", "6661f600004f91ef1ddb624c26451a1b98fccd7b2bf0539467d1160d513f25dd"],
  ["dist/_next/static/chunks/3859ru-30af502c.js", "30af502c54497f844beb5c4b8d4429c8b981da0d875241c73a6ec2d73adfbd58"],
  ["dist/glass-visual-557fe8f7.css", "557fe8f73e245e930be75f4367502d84b158328b989dd7cc2c05cfbc84cedb39"],
]);
function fixture(t) {
  const parent = mkdtempSync(join(tmpdir(), "glass-rank-upgrade-"));
  t.after(() => rmSync(parent, { recursive: true, force: true }));
  const root = join(parent, "Glass");
  cpSync(source, root, { recursive: true });
  copyFileSync(join(repo, "frontend/tests/fixtures/glass-49914cf/index.html.fixture"), join(root, "dist/index.html"));
  copyFileSync(join(repo, "frontend/tests/fixtures/glass-49914cf/komari-theme.json"), join(root, "komari-theme.json"));
  for (const [name, hash] of hashes) assert.equal(createHash("sha256").update(readFileSync(join(root, name))).digest("hex"), hash, `immutable installed ${name}`);
  writeFileSync(join(root, "user-note.txt"), "preserve");
  const run = apply => spawnSync(process.execPath, [join(repo, "frontend/script/upgrade-glass-theme.mjs"), "--theme-dir", root, ...(apply ? ["--apply"] : [])], { encoding: "utf8" });
  return { parent, root, run };
}
test("exact 49914cf installed ranking accepts a dry run and migrates to desktop summary with intact backup", t => {
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
for (const name of hashes.keys()) test(`modified 49914cf ${name} is rejected without any migration writes`, t => {
  const { parent, root, run } = fixture(t);
  writeFileSync(join(root, name), "customized");
  const result = run(true);
  assert.notEqual(result.status, 0);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.equal(readFileSync(join(root, name), "utf8"), "customized");
});

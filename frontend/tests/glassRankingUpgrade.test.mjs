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
  ["dist/index.html", "b815ccf7e617f9c983f5c3ce0fd1c787a366657d9336e55aae83b60f16e94443"],
  ["dist/_next/static/chunks/3859ru-2f2505a3.js", "2f2505a35064a721b0b32313b4d8d0658f7f83fe845ac3c8aa5a65b778ce202d"],
  ["dist/glass-visual-ecf8ea54.css", "ecf8ea54d3d36f6beb72fe404a670af5a2869ae845836223720bd95ac139eb02"],
]);
function fixture(t) {
  const parent = mkdtempSync(join(tmpdir(), "glass-rank-upgrade-"));
  t.after(() => rmSync(parent, { recursive: true, force: true }));
  const root = join(parent, "Glass");
  cpSync(source, root, { recursive: true });
  copyFileSync(join(repo, "frontend/tests/fixtures/glass-c74f01c/index.html.fixture"), join(root, "dist/index.html"));
  copyFileSync(join(repo, "frontend/tests/fixtures/glass-c74f01c/komari-theme.json"), join(root, "komari-theme.json"));
  for (const [name, hash] of hashes) assert.equal(createHash("sha256").update(readFileSync(join(root, name))).digest("hex"), hash, `immutable installed ${name}`);
  writeFileSync(join(root, "user-note.txt"), "preserve");
  const run = apply => spawnSync(process.execPath, [join(repo, "frontend/script/upgrade-glass-theme.mjs"), "--theme-dir", root, ...(apply ? ["--apply"] : [])], { encoding: "utf8" });
  return { parent, root, run };
}
test("exact c74f01c installed annotation accepts a dry run and migrates to ranking with intact backup", t => {
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
for (const name of hashes.keys()) test(`modified c74f01c ${name} is rejected without any migration writes`, t => {
  const { parent, root, run } = fixture(t);
  writeFileSync(join(root, name), "customized");
  const result = run(true);
  assert.notEqual(result.status, 0);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.equal(readFileSync(join(root, name), "utf8"), "customized");
});

import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { chownSync, copyFileSync, mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

const repo = new URL("../..", import.meta.url).pathname;
const script = new URL("../script/upgrade-glass-theme.mjs", import.meta.url).pathname;
const prefix = "backend/web/public/bundledThemes/Glass/";
const fixturePrefix = "frontend/tests/fixtures/glass-vendor/";
const v109FixturePrefix = "frontend/tests/fixtures/glass-v109/";
const v1010FixturePrefix = "frontend/tests/fixtures/glass-v1010/";
const files = ["komari-theme.json", "dist/index.html", "dist/_next/static/chunks/3859ru-aa5bf30a.js", "dist/_next/static/chunks/1j6ltxvh3a3zn.css"];
// A pinned vendor fixture must survive this change becoming HEAD; git show HEAD
// would start reading the upgraded theme after the release commit.
const baseline = (path) => readFileSync(join(repo, path === "komari-theme.json" || path === "dist/index.html" ? fixturePrefix : prefix, path));
function fixture(t) {
  const parent = mkdtempSync(join(tmpdir(), "glass-upgrade-test-"));
  t.after(() => rmSync(parent, { recursive: true, force: true }));
  const root = join(parent, "Glass");
  for (const path of files) {
    const full = join(root, path);
    mkdirSync(join(full, ".."), { recursive: true });
    writeFileSync(full, baseline(path));
  }
  writeFileSync(join(root, "user-note.txt"), "retain me");
  return { parent, root };
}
const run = (root, apply = false) => spawnSync(process.execPath, [script, "--theme-dir", root, ...(apply ? ["--apply"] : [])], { encoding: "utf8" });
const legacyBootstrap = (html) => html.replace(/\|\|(\\*)"dark\1"/g, (_match, escapes) => `||${escapes}"system${escapes}"`);
const legacyManifest = () => readFileSync(join(repo, v109FixturePrefix, "komari-theme.json"), "utf8")
  .replace('"default": "dark"', '"default": "system"');

function previousUpgradeFixture(t) {
  const { parent, root } = fixture(t);
  const bundled = join(repo, prefix);
  const html = legacyBootstrap(readFileSync(join(repo, v109FixturePrefix, "dist/index.html"), "utf8")
    .replace(/<link rel="stylesheet" href="\/glass-visual-[a-f0-9]{8}\.css"\/>/, "")
    .replace(/3859ru-[a-f0-9]{8}\.js/g, "3859ru-ed32134c.js"));
  assert.equal(createHash("sha256").update(html).digest("hex"), "6f28762c9abbe4c230dc2ee92c2430f2aac2d2634dbb5b6d468c9a41c101755b");
  writeFileSync(join(root, "dist/index.html"), html);
  writeFileSync(join(root, "komari-theme.json"), legacyManifest());
  copyFileSync(join(bundled, "dist/_next/static/chunks/3859ru-ed32134c.js"), join(root, "dist/_next/static/chunks/3859ru-ed32134c.js"));
  return { parent, root };
}

test("previously upgraded Glass can safely receive the compact card layout", (t) => {
  const { parent, root } = previousUpgradeFixture(t);
  const dry = run(root);
  assert.equal(dry.status, 0, dry.stderr);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  const result = run(root, true);
  assert.equal(result.status, 0, result.stderr);
  const backup = join(parent, readdirSync(parent).find((name) => name.startsWith("Glass.backup-")));
  assert.equal(createHash("sha256").update(readFileSync(join(backup, "dist/index.html"))).digest("hex"), "6f28762c9abbe4c230dc2ee92c2430f2aac2d2634dbb5b6d468c9a41c101755b");
  assert.equal(readFileSync(join(root, "dist/index.html"), "utf8"), readFileSync(join(repo, prefix, "dist/index.html"), "utf8"));
  assert.equal(readFileSync(join(root, "user-note.txt"), "utf8"), "retain me");
});

test("deployed adaptive-layout Glass is recognized before any visual upgrade", (t) => {
  const { parent, root } = fixture(t);
  const bundled = join(repo, prefix);
  const html = legacyBootstrap(readFileSync(join(repo, v109FixturePrefix, "dist/index.html"), "utf8")
    .replace(/<link rel="stylesheet" href="\/glass-visual-[a-f0-9]{8}\.css"\/>/, ""));
  assert.equal(createHash("sha256").update(html).digest("hex"), "ff081dec062fae4d207dfab11ead2282c6c9fbf38584e9c05530cab73af1636c");
  writeFileSync(join(root, "dist/index.html"), html);
  writeFileSync(join(root, "komari-theme.json"), legacyManifest());
  copyFileSync(join(bundled, "dist/_next/static/chunks/3859ru-1c261694.js"), join(root, "dist/_next/static/chunks/3859ru-1c261694.js"));
  const result = run(root);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /eligible.*true/);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.equal(readFileSync(join(root, "dist/index.html"), "utf8"), html);
});

test("pinned v1.0.9 installed Glass is accepted without any write", (t) => {
  const { parent, root } = fixture(t);
  const bundled = join(repo, prefix);
  for (const name of ["komari-theme.json", "dist/index.html"]) {
    copyFileSync(join(repo, v109FixturePrefix, name), join(root, name));
  }
  copyFileSync(join(bundled, "dist/_next/static/chunks/3859ru-1c261694.js"), join(root, "dist/_next/static/chunks/3859ru-1c261694.js"));
  const result = run(root);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /eligible.*true/);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.deepEqual(readFileSync(join(root, "dist/index.html")), readFileSync(join(repo, v109FixturePrefix, "dist/index.html")));
});

test("v1.0.10 single-selection Glass upgrades to multi-selection without losing user files", (t) => {
  const { parent, root } = fixture(t);
  const bundled = join(repo, prefix);
  for (const name of ["komari-theme.json", "dist/index.html"]) {
    copyFileSync(join(repo, v1010FixturePrefix, name), join(root, name));
  }
  copyFileSync(join(bundled, "dist/_next/static/chunks/3859ru-0b886112.js"), join(root, "dist/_next/static/chunks/3859ru-0b886112.js"));
  const dry = run(root);
  assert.equal(dry.status, 0, dry.stderr);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  const result = run(root, true);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(join(root, "user-note.txt"), "utf8"), "retain me");
  assert.equal(readFileSync(join(root, "dist/index.html"), "utf8"), readFileSync(join(bundled, "dist/index.html"), "utf8"));
});

test("upgrade dry run confirms pinned installed Glass without writing", (t) => {
  const { parent, root } = fixture(t);
  const result = run(root);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /eligible.*true/);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.deepEqual(readFileSync(join(root, "dist/index.html")), baseline("dist/index.html"));
});

test("explicit apply upgrades pinned Glass and preserves backup, ownership and unrelated files", (t) => {
  const { parent, root } = fixture(t);
  if (process.getuid?.() === 0) {
    chownSync(root, 65534, 65534);
    chownSync(join(root, "komari-theme.json"), 65534, 65534);
    chownSync(join(root, "dist/index.html"), 65534, 65534);
  }
  const oldRoot = statSync(root);
  const oldManifest = statSync(join(root, "komari-theme.json"));
  const result = run(root, true);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(statSync(root).uid, oldRoot.uid);
  assert.equal(statSync(root).gid, oldRoot.gid);
  assert.equal(statSync(join(root, "komari-theme.json")).uid, oldManifest.uid);
  assert.equal(statSync(join(root, "dist/index.html")).uid, oldManifest.uid);
  const names = readdirSync(parent);
  assert.equal(names.filter((name) => name.startsWith("Glass.backup-")).length, 1);
  const backup = join(parent, names.find((name) => name.startsWith("Glass.backup-")));
  assert.deepEqual(readFileSync(join(backup, "dist/index.html")), baseline("dist/index.html"));
  assert.equal(readFileSync(join(root, "user-note.txt"), "utf8"), "retain me");
  const current = readFileSync(join(root, "dist/index.html"), "utf8");
  const [chunk] = current.match(/3859ru-[0-9a-f]{8}\.js/g) ?? [];
  assert.ok(chunk && chunk !== "3859ru-aa5bf30a.js");
  const data = readFileSync(join(root, "dist/_next/static/chunks", chunk));
  assert.equal(createHash("sha256").update(data).digest("hex").slice(0, 8), chunk.slice(7, 15));
  const [cssName] = current.match(/glass-visual-[a-f0-9]{8}\.css/g) ?? [];
  assert.ok(cssName, "the upgraded theme must include its referenced visual stylesheet");
  const css = readFileSync(join(root, "dist", cssName));
  assert.equal(createHash("sha256").update(css).digest("hex").slice(0, 8), cssName.slice(13, 21));
});

test("customized installed Glass is rejected without changing or backing up anything", (t) => {
  const { parent, root } = fixture(t);
  writeFileSync(join(root, "dist/index.html"), "customized");
  const result = run(root, true);
  assert.notEqual(result.status, 0);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.equal(readFileSync(join(root, "dist/index.html"), "utf8"), "customized");
});

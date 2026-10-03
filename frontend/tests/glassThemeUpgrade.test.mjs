import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { chownSync, copyFileSync, cpSync, mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

const repo = new URL("../..", import.meta.url).pathname;
const script = new URL("../script/upgrade-glass-theme.mjs", import.meta.url).pathname;
const prefix = "backend/web/public/bundledThemes/Glass/";
const fixturePrefix = "frontend/tests/fixtures/glass-vendor/";
const v109FixturePrefix = "frontend/tests/fixtures/glass-v109/";
const v1010FixturePrefix = "frontend/tests/fixtures/glass-v1010/";
const priorFixturePrefix = "frontend/tests/fixtures/glass-v1012/";
const installedFixturePrefix = "frontend/tests/fixtures/glass-v1016/";
const priorHashes = new Map([
  ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
  ["dist/index.html", "abb776b0255d54da4b5a8f8b0a34054c8e797b48e18fb029bf51ede40263674c"],
  ["dist/_next/static/chunks/3859ru-36ef0bd8.js", "36ef0bd863581ea218f5c4d2a183145132a780792c07d46dc97276d27345f9ea"],
]);
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
const run = (root, apply = false, preload = null, scriptPath = script) => spawnSync(process.execPath, [...(preload ? ["--import", preload] : []), scriptPath, "--theme-dir", root, ...(apply ? ["--apply"] : [])], { encoding: "utf8" });

test("migration stages an immutable, fingerprint-checked source snapshot", () => {
  const migration = readFileSync(script, "utf8");
  assert.match(migration, /const replacementFiles = new Map\(/);
  assert.match(migration, /digest\(replacementFiles\.get\(chunkPath\)\)\.slice\(0, 8\)/);
  assert.match(migration, /digest\(replacementFiles\.get\(cssPath\)\)\.slice\(0, 8\)/);
  assert.match(migration, /writeFileSync\(join\(stage, relative\), replacementFiles\.get\(relative\)\)/);
  assert.doesNotMatch(migration, /expectedStage\.set\(relative, digest\(readFileSync\(join\(source, relative\)\)\)\)/);
});
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
  copyFileSync(join(bundled, "dist/glass-visual-fc179bf5.css"), join(root, "dist/glass-visual-fc179bf5.css"));
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
  copyFileSync(join(bundled, "dist/glass-visual-fc179bf5.css"), join(root, "dist/glass-visual-fc179bf5.css"));
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

function currentInstalledFixture(t) {
  const { parent, root } = fixture(t);
  for (const [path, expected] of priorHashes) {
    // The prior manifest/HTML are immutable fixtures; the content-hashed old
    // chunk remains a pinned, tracked asset alongside the newer chunks.
    const bytes = readFileSync(join(repo, path.startsWith("dist/_next/") ? prefix : priorFixturePrefix, path));
    assert.equal(createHash("sha256").update(bytes).digest("hex"), expected, `pinned prior ${path}`);
    const target = join(root, path);
    mkdirSync(join(target, ".."), { recursive: true });
    writeFileSync(target, bytes);
  }
  copyFileSync(join(repo, prefix, "dist/glass-visual-fc179bf5.css"), join(root, "dist/glass-visual-fc179bf5.css"));
  return { parent, root };
}

test("customized installed visual CSS is rejected before any theme migration", (t) => {
  const { parent, root } = currentInstalledFixture(t);
  const css = join(root, "dist/glass-visual-fc179bf5.css");
  writeFileSync(css, "custom theme CSS");
  const result = run(root, true);
  assert.notEqual(result.status, 0);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.equal(readFileSync(css, "utf8"), "custom theme CSS");
});

test("a conflicting new visual stylesheet is not overwritten", (t) => {
  const { parent, root } = currentInstalledFixture(t);
  const current = readFileSync(join(repo, prefix, "dist/index.html"), "utf8");
  const [newName] = current.match(/glass-visual-[a-f0-9]{8}\.css/g) ?? [];
  assert.ok(newName);
  const conflict = join(root, "dist", newName);
  writeFileSync(conflict, "user custom CSS at destination");
  const result = run(root, true);
  assert.notEqual(result.status, 0);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.equal(readFileSync(conflict, "utf8"), "user custom CSS at destination");
});

test("a conflicting new node-card chunk is not overwritten", (t) => {
  const { parent, root } = fixture(t);
  const bundled = join(repo, prefix);
  for (const name of ["komari-theme.json", "dist/index.html"]) {
    copyFileSync(join(repo, v1010FixturePrefix, name), join(root, name));
  }
  copyFileSync(join(bundled, "dist/_next/static/chunks/3859ru-0b886112.js"), join(root, "dist/_next/static/chunks/3859ru-0b886112.js"));
  copyFileSync(join(bundled, "dist/glass-visual-fc179bf5.css"), join(root, "dist/glass-visual-fc179bf5.css"));
  const newIndex = readFileSync(join(bundled, "dist/index.html"), "utf8");
  const [newName] = newIndex.match(/3859ru-[a-f0-9]{8}\.js/g) ?? [];
  assert.ok(newName);
  const conflict = join(root, "dist/_next/static/chunks", newName);
  writeFileSync(conflict, "user custom JS at destination");
  const result = run(root, true);
  assert.notEqual(result.status, 0);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.equal(readFileSync(conflict, "utf8"), "user custom JS at destination");
});

test("a concurrent change after copying cannot pass the pinned theme validation", (t) => {
  const { parent, root } = currentInstalledFixture(t);
  const hook = join(parent, "race-hook.mjs");
  writeFileSync(hook, `import fs from "node:fs";
import { join } from "node:path";
import { syncBuiltinESMExports } from "node:module";
const copy = fs.cpSync;
fs.cpSync = (...args) => {
  const result = copy(...args);
  fs.writeFileSync(join(args[0], "dist/index.html"), "changed during staging");
  return result;
};
syncBuiltinESMExports();
`);
  const result = run(root, true, hook);
  assert.notEqual(result.status, 0);
  assert.deepEqual(readdirSync(parent).sort(), ["Glass", "race-hook.mjs"]);
  assert.equal(readFileSync(join(root, "dist/index.html"), "utf8"), "changed during staging");
});

test("a changed bundled asset during staging cannot bypass its original fingerprint", (t) => {
  const { parent, root } = currentInstalledFixture(t);
  const isolated = join(parent, "isolated");
  const isolatedScript = join(isolated, "frontend/script/upgrade-glass-theme.mjs");
  const isolatedBundle = join(isolated, prefix);
  mkdirSync(join(isolatedScript, ".."), { recursive: true });
  copyFileSync(script, isolatedScript);
  cpSync(join(repo, prefix), isolatedBundle, { recursive: true });
  const index = readFileSync(join(isolatedBundle, "dist/index.html"), "utf8");
  const [name] = index.match(/glass-visual-[a-f0-9]{8}\.css/g) ?? [];
  assert.ok(name);
  const sourceCss = join(isolatedBundle, "dist", name);
  const validatedCss = readFileSync(sourceCss);
  const hook = join(parent, "source-race-hook.mjs");
  writeFileSync(hook, `import fs from "node:fs";
import { syncBuiltinESMExports } from "node:module";
const copy = fs.cpSync;
fs.cpSync = (...args) => {
  const result = copy(...args);
  fs.writeFileSync(${JSON.stringify(sourceCss)}, "changed source after fingerprint validation");
  return result;
};
syncBuiltinESMExports();
`);
  const originalIndex = readFileSync(join(root, "dist/index.html"));
  const result = run(root, true, hook, isolatedScript);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readdirSync(parent).some((name) => name.startsWith("Glass.backup-")), true);
  assert.notDeepEqual(readFileSync(join(root, "dist/index.html")), originalIndex);
  assert.deepEqual(readFileSync(join(root, "dist", name)), validatedCss);
  assert.equal(readFileSync(sourceCss, "utf8"), "changed source after fingerprint validation");
});

test("exact prior installed Glass accepts dry-run and upgrades while preserving backup and unrelated files", (t) => {
  const { parent, root } = currentInstalledFixture(t);
  const dry = run(root);
  assert.equal(dry.status, 0, dry.stderr);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  const result = run(root, true);
  assert.equal(result.status, 0, result.stderr);
  const backup = join(parent, readdirSync(parent).find((name) => name.startsWith("Glass.backup-")));
  for (const [path, expected] of priorHashes) {
    assert.equal(createHash("sha256").update(readFileSync(join(backup, path))).digest("hex"), expected);
  }
  assert.equal(readFileSync(join(root, "user-note.txt"), "utf8"), "retain me");
  assert.deepEqual(readFileSync(join(root, "dist/index.html")), readFileSync(join(repo, prefix, "dist/index.html")));
});

for (const changed of priorHashes.keys()) {
  test(`customized prior Glass ${changed} is rejected without writes`, (t) => {
    const { parent, root } = currentInstalledFixture(t);
    writeFileSync(join(root, changed), "customized");
    const result = run(root, true);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /differs from all pinned versions/);
    assert.deepEqual(readdirSync(parent), ["Glass"]);
    assert.equal(readFileSync(join(root, changed), "utf8"), "customized");
    assert.equal(readFileSync(join(root, "user-note.txt"), "utf8"), "retain me");
  });
}

function deployedGlassFixture(t) {
  const { parent, root } = fixture(t);
  const html = readFileSync(join(repo, installedFixturePrefix, "index.html.fixture"));
  assert.equal(createHash("sha256").update(html).digest("hex"), "1920bc513d67a9226195e1ac31e87ae58957a25f044b5bbbc8019e57680f1f4b");
  writeFileSync(join(root, "dist/index.html"), html);
  copyFileSync(join(repo, priorFixturePrefix, "komari-theme.json"), join(root, "komari-theme.json"));
  copyFileSync(join(repo, prefix, "dist/_next/static/chunks/3859ru-36ef0bd8.js"), join(root, "dist/_next/static/chunks/3859ru-36ef0bd8.js"));
  copyFileSync(join(repo, prefix, "dist/glass-visual-5dee188d.css"), join(root, "dist/glass-visual-5dee188d.css"));
  return { parent, root, html };
}

test("exact deployed Glass upgrades to denser palette with its original HTML backed up", (t) => {
  const { parent, root, html } = deployedGlassFixture(t);
  const dry = run(root);
  assert.equal(dry.status, 0, dry.stderr);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  const applied = run(root, true);
  assert.equal(applied.status, 0, applied.stderr);
  const backup = join(parent, readdirSync(parent).find((name) => name.startsWith("Glass.backup-")));
  assert.deepEqual(readFileSync(join(backup, "dist/index.html")), html);
  assert.equal(readFileSync(join(root, "user-note.txt"), "utf8"), "retain me");
  assert.deepEqual(readFileSync(join(root, "dist/index.html")), readFileSync(join(repo, prefix, "dist/index.html")));
});

test("deployed Glass with modified visual CSS is refused without changing files", (t) => {
  const { parent, root, html } = deployedGlassFixture(t);
  const css = join(root, "dist/glass-visual-5dee188d.css");
  writeFileSync(css, "customized");
  const applied = run(root, true);
  assert.notEqual(applied.status, 0);
  assert.deepEqual(readdirSync(parent), ["Glass"]);
  assert.deepEqual(readFileSync(join(root, "dist/index.html")), html);
});

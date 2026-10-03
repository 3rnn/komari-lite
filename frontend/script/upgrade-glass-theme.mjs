// Explicit migration for a Glass theme with pinned core manifest, HTML and chunk.
// Dry-run by default; never deletes the existing theme or its backup.
// Other files are copied as-is: inspect custom CSS/JS before applying.
import { createHash, randomUUID } from "node:crypto";
import { chownSync, chmodSync, cpSync, existsSync, lstatSync, mkdtempSync, readFileSync, readdirSync, renameSync, rmSync, statSync, writeFileSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const knownVersions = [
  // v1.0.12 bundled Glass, verified byte-for-byte against the installed theme.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "abb776b0255d54da4b5a8f8b0a34054c8e797b48e18fb029bf51ede40263674c"],
    ["dist/_next/static/chunks/3859ru-36ef0bd8.js", "36ef0bd863581ea218f5c4d2a183145132a780792c07d46dc97276d27345f9ea"],
  ]),
  // The currently deployed v1.0.16 Glass has the same core chunk but a newer HTML/CSS.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "1920bc513d67a9226195e1ac31e87ae58957a25f044b5bbbc8019e57680f1f4b"],
    ["dist/_next/static/chunks/3859ru-36ef0bd8.js", "36ef0bd863581ea218f5c4d2a183145132a780792c07d46dc97276d27345f9ea"],
  ]),
  // Short-lived, pinned first-pass ping palette; retain a safe upgrade path.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "a7265b3e9b31871f6dc32a80ea4a86cfb639dc304287790b704f4ff5d9103501"],
    ["dist/_next/static/chunks/3859ru-52a576f3.js", "52a576f38c88d693c83f0fe0aec8ed76aea839cbdbcafde9c21b7718ebe11bc5"],
  ]),
  // Current installed ping theme before the latency-only width/legend adjustment.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "e172f917818ddb087c24a6c5fdfce6ae3513240b5649d09685a36a3d99b3648d"],
    ["dist/_next/static/chunks/3859ru-13dad3d8.js", "13dad3d889d56444ed0d101b98f89621cebe1e8ba64ac515eb0b61f5c1debc4a"],
  ]),
  new Map([
    ["komari-theme.json", "d2594e16f56f4a2160ccc873468588be96add67bf15ce09eb205e397fd7b8f60"],
    ["dist/index.html", "9c2a14485bbb7f70daec0b3b13010e50cb2fd3ebc7311faa23c792c2f98b03bd"],
    ["dist/_next/static/chunks/3859ru-aa5bf30a.js", "aa5bf30a27112ed3e37ccfc6a950664851b62bd58185f1fc40ee8a6b9bad6512"],
  ]),
  new Map([
    ["komari-theme.json", "6d3c229d87845154e563ca00faceab04ddfb9c779cc99680ccb5ddfbcbda9752"],
    ["dist/index.html", "6f28762c9abbe4c230dc2ee92c2430f2aac2d2634dbb5b6d468c9a41c101755b"],
    ["dist/_next/static/chunks/3859ru-ed32134c.js", "ed32134cfcbd0c4b68b93786816be7fb79ad080108883a67476405e622eba88d"],
  ]),
  new Map([
    ["komari-theme.json", "6d3c229d87845154e563ca00faceab04ddfb9c779cc99680ccb5ddfbcbda9752"],
    ["dist/index.html", "ff081dec062fae4d207dfab11ead2282c6c9fbf38584e9c05530cab73af1636c"],
    ["dist/_next/static/chunks/3859ru-1c261694.js", "1c261694026e109bf75e4c353869a9b5808b37c0eebbb5820d37495e5e8e3c28"],
  ]),
  new Map([
    ["komari-theme.json", "644f7c23a98d11d4c365884d768ab8ad135f99dd99ce90483dc0b973a335142e"],
    ["dist/index.html", "b067119ebef492a5f2bc8af0a68b9a2e7787d674374e44b1d38b07f6e0b92ec9"],
    ["dist/_next/static/chunks/3859ru-1c261694.js", "1c261694026e109bf75e4c353869a9b5808b37c0eebbb5820d37495e5e8e3c28"],
  ]),
  new Map([
    ["komari-theme.json", "7337cde54bba312a59f531368a9a4fc169742e52d0cf2eb709461608c936aa77"],
    ["dist/index.html", "df95c933824340e352c205160310e099cc6b419c469b6be590f4a2bbaccf93d8"],
    ["dist/_next/static/chunks/3859ru-0b886112.js", "0b8861122d6a1438fb32cb0a84cf0b98da596deb64744df9cf6408e690c67368"],
  ]),
];
const knownVisualStyles = new Map([
  ["glass-visual-fc179bf5.css", "fc179bf571b818b324c1d86465b8a6ee4bc590d551c90f5d50463e9937a3b3fd"],
  ["glass-visual-5dee188d.css", "5dee188db03783d3db47dbe65c148f1a07fa1747fd12983607dee5c6948b9169"],
  ["glass-visual-b3511731.css", "b35117319b1ad40b5515250a018c04f93301646be082192514270573ffe281ff"],
  ["glass-visual-ebb062ca.css", "ebb062cabd84536e3ec061eb91a819c09cd493f396b2724d1a1c1f80ac4000f3"],
]);
const source = fileURLToPath(new URL("../../backend/web/public/bundledThemes/Glass/", import.meta.url));
const args = process.argv.slice(2);
if ((args.length !== 2 && args.length !== 3) || args[0] !== "--theme-dir" || (args.length === 3 && args[2] !== "--apply")) {
  throw new Error("usage: node frontend/script/upgrade-glass-theme.mjs --theme-dir /path/to/Glass [--apply]");
}
const root = resolve(args[1]);
if (basename(root) !== "Glass" || !lstatSync(root).isDirectory() || !lstatSync(dirname(root)).isDirectory()) {
  throw new Error("expected an existing, non-symlink Glass theme directory");
}
function assertNoSymlinks(path) {
  for (const entry of readdirSync(path, { withFileTypes: true })) {
    if (entry.isSymbolicLink()) throw new Error(`refusing theme with symlink: ${entry.name}`);
    if (entry.isDirectory()) assertNoSymlinks(join(path, entry.name));
  }
}
assertNoSymlinks(root);
const digest = (data) => createHash("sha256").update(data).digest("hex");
const installed = knownVersions.find((files) => [...files].every(([relative, expected]) => {
  const file = join(root, relative);
  return existsSync(file) && lstatSync(file).isFile() && digest(readFileSync(file)) === expected;
}));
if (!installed) throw new Error("installed Glass differs from all pinned versions; no changes made");
const oldIndex = readFileSync(join(root, "dist/index.html"), "utf8");
const oldStyles = [...new Set([...oldIndex.matchAll(/glass-visual-[a-f0-9]{8}\.css/g)].map(([name]) => name))];
if (oldStyles.length > 1 || oldStyles.some((name) => {
  const file = join(root, "dist", name);
  return !knownVisualStyles.has(name) || !existsSync(file) || !lstatSync(file).isFile() || digest(readFileSync(file)) !== knownVisualStyles.get(name);
})) {
  throw new Error("installed Glass visual stylesheet differs from the pinned version; no changes made");
}
const installedChunk = [...installed.keys()].find((relative) => relative.startsWith("dist/_next/static/chunks/"));
const indexBytes = readFileSync(join(source, "dist/index.html"));
const index = indexBytes.toString("utf8");
const names = [...new Set([...index.matchAll(/3859ru-[a-f0-9]{8}\.js/g)].map(([name]) => name))];
if (names.length !== 1 || names[0] === "3859ru-aa5bf30a.js") throw new Error("built source does not reference exactly one new Glass node-card asset");
const chunkName = names[0];
const chunkPath = `dist/_next/static/chunks/${chunkName}`;
const cssNames = [...new Set([...index.matchAll(/glass-visual-[a-f0-9]{8}\.css/g)].map(([name]) => name))];
if (cssNames.length !== 1) throw new Error("built source does not reference exactly one Glass visual stylesheet");
const cssPath = `dist/${cssNames[0]}`;
const replacementFiles = new Map([
  ["komari-theme.json", readFileSync(join(source, "komari-theme.json"))],
  ["dist/index.html", indexBytes],
  [chunkPath, readFileSync(join(source, chunkPath))],
  [cssPath, readFileSync(join(source, cssPath))],
]);
if (!lstatSync(join(source, chunkPath)).isFile() || digest(replacementFiles.get(chunkPath)).slice(0, 8) !== chunkName.slice(7, 15)) {
  throw new Error("new Glass chunk does not match its content fingerprint");
}
if (!lstatSync(join(source, cssPath)).isFile() || digest(replacementFiles.get(cssPath)).slice(0, 8) !== cssNames[0].slice(13, 21)) {
  throw new Error("new Glass visual stylesheet does not match its content fingerprint");
}
const installedNewCss = join(root, cssPath);
if (existsSync(installedNewCss) && (!lstatSync(installedNewCss).isFile() || digest(readFileSync(installedNewCss)) !== digest(replacementFiles.get(cssPath)))) {
  throw new Error("installed Glass already has a conflicting new visual stylesheet; no changes made");
}
const installedNewChunk = join(root, chunkPath);
if (existsSync(installedNewChunk) && (!lstatSync(installedNewChunk).isFile() || digest(readFileSync(installedNewChunk)) !== digest(replacementFiles.get(chunkPath)))) {
  throw new Error("installed Glass already has a conflicting new node-card chunk; no changes made");
}
console.log(JSON.stringify({ eligible: true, apply: args.includes("--apply"), asset: chunkName }));
if (!args.includes("--apply")) process.exit(0);

const parent = dirname(root);
const backup = join(parent, `Glass.backup-${new Date().toISOString().replaceAll(":", "-")}-${randomUUID().slice(0, 8)}`);
const stage = mkdtempSync(join(parent, ".Glass-stage-"));
function inventory(directory) {
  const files = new Map();
  function visit(path, relative = "") {
    for (const entry of readdirSync(path, { withFileTypes: true })) {
      const name = relative ? join(relative, entry.name) : entry.name;
      const full = join(path, entry.name);
      if (entry.isDirectory()) visit(full, name);
      else if (entry.isFile()) files.set(name, digest(readFileSync(full)));
      else throw new Error(`refusing non-regular theme entry: ${name}`);
    }
  }
  visit(directory);
  return files;
}
function assertInventory(directory, expected) {
  const actual = inventory(directory);
  if (actual.size !== expected.size || [...expected].some(([name, hash]) => actual.get(name) !== hash)) {
    throw new Error("theme changed during staging; no replacement made");
  }
}
function preserveOwnershipAndMode(original, copy) {
  for (const entry of readdirSync(original, { withFileTypes: true })) {
    const oldPath = join(original, entry.name);
    const newPath = join(copy, entry.name);
    if (entry.isDirectory()) preserveOwnershipAndMode(oldPath, newPath);
    const { uid, gid, mode } = statSync(oldPath);
    chownSync(newPath, uid, gid);
    chmodSync(newPath, mode & 0o7777);
  }
  const { uid, gid, mode } = statSync(original);
  chownSync(copy, uid, gid);
  chmodSync(copy, mode & 0o7777);
}
try {
  const original = inventory(root);
  if ([...installed].some(([name, hash]) => original.get(name) !== hash) || oldStyles.some((name) => original.get(join("dist", name)) !== knownVisualStyles.get(name))) {
    throw new Error("installed Glass changed before staging; no replacement made");
  }
  for (const relative of [chunkPath, cssPath]) {
    if (original.has(relative) && original.get(relative) !== digest(replacementFiles.get(relative))) {
      throw new Error("installed Glass has a conflicting destination asset; no replacement made");
    }
  }
  cpSync(root, stage, { recursive: true });
  assertInventory(stage, original);
  assertInventory(root, original);
  for (const relative of ["komari-theme.json", "dist/index.html", chunkPath, cssPath]) {
    writeFileSync(join(stage, relative), replacementFiles.get(relative));
  }
  preserveOwnershipAndMode(root, stage);
  const originalChunk = statSync(join(root, installedChunk));
  chownSync(join(stage, chunkPath), originalChunk.uid, originalChunk.gid);
  chmodSync(join(stage, chunkPath), originalChunk.mode & 0o7777);
  const originalCss = statSync(join(root, "dist/_next/static/chunks/1j6ltxvh3a3zn.css"));
  chownSync(join(stage, cssPath), originalCss.uid, originalCss.gid);
  chmodSync(join(stage, cssPath), originalCss.mode & 0o7777);
  const expectedStage = new Map(original);
  for (const relative of ["komari-theme.json", "dist/index.html", chunkPath, cssPath]) {
    expectedStage.set(relative, digest(replacementFiles.get(relative)));
  }
  assertInventory(stage, expectedStage);
  assertInventory(root, original);
  // These renames are not an atomic exchange. On host/process death between
  // them, restore the verified backup as documented in GLASS_RECOVERY.md.
  renameSync(root, backup);
  try {
    renameSync(stage, root);
  } catch (error) {
    renameSync(backup, root);
    throw error;
  }
  console.log(JSON.stringify({ upgraded: true, backup }));
} finally {
  if (existsSync(stage)) rmSync(stage, { recursive: true, force: true });
}

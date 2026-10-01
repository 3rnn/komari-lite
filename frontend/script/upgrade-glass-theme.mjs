// Explicit migration for a Glass theme with pinned core manifest, HTML and chunk.
// Dry-run by default; never deletes the existing theme or its backup.
// Other files are copied as-is: inspect custom CSS/JS before applying.
import { createHash, randomUUID } from "node:crypto";
import { chownSync, chmodSync, copyFileSync, cpSync, existsSync, lstatSync, mkdtempSync, readFileSync, readdirSync, renameSync, rmSync, statSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const knownVersions = [
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
const installedChunk = [...installed.keys()].find((relative) => relative.startsWith("dist/_next/static/chunks/"));
const index = readFileSync(join(source, "dist/index.html"), "utf8");
const names = [...new Set([...index.matchAll(/3859ru-[a-f0-9]{8}\.js/g)].map(([name]) => name))];
if (names.length !== 1 || names[0] === "3859ru-aa5bf30a.js") throw new Error("built source does not reference exactly one new Glass node-card asset");
const chunkName = names[0];
const chunkPath = `dist/_next/static/chunks/${chunkName}`;
if (!lstatSync(join(source, chunkPath)).isFile() || digest(readFileSync(join(source, chunkPath))).slice(0, 8) !== chunkName.slice(7, 15)) {
  throw new Error("new Glass chunk does not match its content fingerprint");
}
const cssNames = [...new Set([...index.matchAll(/glass-visual-[a-f0-9]{8}\.css/g)].map(([name]) => name))];
if (cssNames.length !== 1) throw new Error("built source does not reference exactly one Glass visual stylesheet");
const cssPath = `dist/${cssNames[0]}`;
if (!lstatSync(join(source, cssPath)).isFile() || digest(readFileSync(join(source, cssPath))).slice(0, 8) !== cssNames[0].slice(13, 21)) {
  throw new Error("new Glass visual stylesheet does not match its content fingerprint");
}
console.log(JSON.stringify({ eligible: true, apply: args.includes("--apply"), asset: chunkName }));
if (!args.includes("--apply")) process.exit(0);

const parent = dirname(root);
const backup = join(parent, `Glass.backup-${new Date().toISOString().replaceAll(":", "-")}-${randomUUID().slice(0, 8)}`);
const stage = mkdtempSync(join(parent, ".Glass-stage-"));
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
  cpSync(root, stage, { recursive: true });
  for (const relative of ["komari-theme.json", "dist/index.html", chunkPath, cssPath]) {
    copyFileSync(join(source, relative), join(stage, relative));
  }
  preserveOwnershipAndMode(root, stage);
  const originalChunk = statSync(join(root, installedChunk));
  chownSync(join(stage, chunkPath), originalChunk.uid, originalChunk.gid);
  chmodSync(join(stage, chunkPath), originalChunk.mode & 0o7777);
  const originalCss = statSync(join(root, "dist/_next/static/chunks/1j6ltxvh3a3zn.css"));
  chownSync(join(stage, cssPath), originalCss.uid, originalCss.gid);
  chmodSync(join(stage, cssPath), originalCss.mode & 0o7777);
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

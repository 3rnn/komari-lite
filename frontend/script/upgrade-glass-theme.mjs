// Explicit migration for a Glass theme with pinned core manifest, HTML and chunk.
// Dry-run by default; never deletes the existing theme or its backup.
// Other files are copied as-is: inspect custom CSS/JS before applying.
import { createHash, randomUUID } from "node:crypto";
import { chownSync, chmodSync, cpSync, existsSync, lstatSync, mkdtempSync, readFileSync, readdirSync, renameSync, rmSync, statSync, writeFileSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const knownVersions = [
  // Exact deployed 49914cf mobile summary before the desktop-only redesign.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "6661f600004f91ef1ddb624c26451a1b98fccd7b2bf0539467d1160d513f25dd"],
    ["dist/_next/static/chunks/3859ru-30af502c.js", "30af502c54497f844beb5c4b8d4429c8b981da0d875241c73a6ec2d73adfbd58"],
  ]),
  // Exact deployed 58d6998 bounded ranking before the mobile-only summary.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "7923290bb879eb775df7769d1b193652a6587a079a4dd373e2f4bd5607c6d18d"],
    ["dist/_next/static/chunks/3859ru-fa8e62f6.js", "fa8e62f6e5e44480f37a2de0389d5bc6ae93cc56cfa835c03292c7250114b06a"],
  ]),
  // Exact installed c74f01c compact annotation before the bounded ranking rail.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "b815ccf7e617f9c983f5c3ce0fd1c787a366657d9336e55aae83b60f16e94443"],
    ["dist/_next/static/chunks/3859ru-2f2505a3.js", "2f2505a35064a721b0b32313b4d8d0658f7f83fe845ac3c8aa5a65b778ce202d"],
  ]),
  // Exact installed compact card/list theme before the throughput annotation.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "d4302d7b01b74526a3b3607a94e9229c6628006364356c39119b0e645a4e4a98"],
    ["dist/_next/static/chunks/3859ru-43ce8b19.js", "43ce8b1948eb027a51e5b2e4cd5158eedb84e2ba031975a1f76bccea1756007c"],
  ]),
  // Initial compact-row preview before small-width metric alignment.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "9baf6144d80ace7b0ae268e94cc8b627f16752d7d3dc1507b68401a0a1b9b18d"],
    ["dist/_next/static/chunks/3859ru-43ce8b19.js", "43ce8b1948eb027a51e5b2e4cd5158eedb84e2ba031975a1f76bccea1756007c"],
  ]),
  // Installed development list preview before compact horizontal rows.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "e8a7ae0d62870f9c1e63fe2db66cfcf2a35168ab3ba5c793490261faa60b4e35"],
    ["dist/_next/static/chunks/3859ru-2eea8e89.js", "2eea8e8994fdcd265a7dc60b4bed299a8a178391e84ecc5bb7e595b92d4c07af"],
  ]),
  // Initial development list preview, before long network-value wrapping.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "765f38099c12baf5a0f82e6bcb60cfe98b33c4c9a6a6c96cc1cda3300ed82b93"],
    ["dist/_next/static/chunks/3859ru-2eea8e89.js", "2eea8e8994fdcd265a7dc60b4bed299a8a178391e84ecc5bb7e595b92d4c07af"],
  ]),
  // Development v1.0.60 immediately before the optional card/list view.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "83fd4f5950b2406567f9cdf5aca2d171bde44007c141254c788c30d1b2ef3ca4"],
    ["dist/_next/static/chunks/3859ru-5c958c91.js", "5c958c916943bc516d33188524b579291be850cd544c750c16c432f348dcbcf5"],
  ]),
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
  // Installed latency-only legend fix, before removing packet-loss legend dots.
  new Map([
    ["komari-theme.json", "022143c717a12cf44e5af525bd61cf596ace4ef1415914d2ee71a65e42a2d284"],
    ["dist/index.html", "c02a01067ce13058ce59c626707262a9c7e34e69909f2b096e87cff3fc43c247"],
    ["dist/_next/static/chunks/3859ru-d2e6b2f6.js", "d2e6b2f6a2693cc5562104e2b35c0dee75f065a8113f36c5fa24bf09f481e164"],
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
  ["glass-visual-557fe8f7.css", "557fe8f73e245e930be75f4367502d84b158328b989dd7cc2c05cfbc84cedb39"],
  ["glass-visual-8af54d66.css", "8af54d66f5eb4a5bd76e839f2ef8ae627744471e495401a1290f953b9b158115"],
  ["glass-visual-ecf8ea54.css", "ecf8ea54d3d36f6beb72fe404a670af5a2869ae845836223720bd95ac139eb02"],
  ["glass-visual-15bcafd2.css", "15bcafd269e32f626d5777b1b454991ff424011d2268503dd5bbdffc632f94cb"],
  ["glass-visual-38210cb5.css", "38210cb587c5d334536d1e0bbf5ebf3027f84989bf7ff93c6e112777834a72e2"],
  ["glass-visual-fd073f36.css", "fd073f367cc08099f96c64314e569233a7d9cbd6d16e773e4e0e94e6fea865ca"],
  ["glass-visual-9786cb16.css", "9786cb164dfb950f4ddadd0661294ef1cf04b71d297ad87b3b87e3e963929b84"],
  ["glass-visual-fc179bf5.css", "fc179bf571b818b324c1d86465b8a6ee4bc590d551c90f5d50463e9937a3b3fd"],
  ["glass-visual-5dee188d.css", "5dee188db03783d3db47dbe65c148f1a07fa1747fd12983607dee5c6948b9169"],
  ["glass-visual-b3511731.css", "b35117319b1ad40b5515250a018c04f93301646be082192514270573ffe281ff"],
  ["glass-visual-ebb062ca.css", "ebb062cabd84536e3ec061eb91a819c09cd493f396b2724d1a1c1f80ac4000f3"],
  ["glass-visual-b543e07d.css", "b543e07d6f5973d834ea75fbda4a950a59847628cb2ab99ce5d577fa949f5aa7"],
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

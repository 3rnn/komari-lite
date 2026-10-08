import { cp, readFile, rm } from "node:fs/promises";
import { resolve } from "node:path";

process.env.VITE_SYSTEM_UI_BUILD = "1";
process.env.VITE_BASE_URL = "/system-assets/";

const { build } = await import("vite");
await build({ configLoader: "runner" });

const embeddedDistPath = resolve("../backend/web/public/systemUI/dist");
await rm(embeddedDistPath, { recursive: true, force: true });
await cp(resolve("dist"), embeddedDistPath, { recursive: true });

const indexPath = resolve("dist/index.html");
const index = await readFile(indexPath, "utf8");

if (!index.includes('/system-assets/assets/entry-')) {
  throw new Error("System UI build rejected: entry asset is not under /system-assets/.");
}

const forbidden = [
  /(?:src|href)=["']\/assets\//i,
  /registerSW/i,
  /manifest\.webmanifest/i,
];

for (const pattern of forbidden) {
  if (pattern.test(index)) {
    throw new Error(`System UI build rejected: dist/index.html matched ${pattern}.`);
  }
}

console.log("System UI build verified: all entry assets use /system-assets/.");

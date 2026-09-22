import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory()
      ? sourceFiles(path)
      : /\.(?:ts|tsx|js|jsx|css|scss|html)$/.test(entry.name)
        ? [path]
        : [];
  });
}

test("system UI assets follow the configured Vite base URL", () => {
  const applicationFiles = [...sourceFiles("src"), "index.html"];
  const offenders = applicationFiles
    .filter((path) => readFileSync(path, "utf8").includes("/assets/"));

  assert.deepEqual(offenders, []);

  const helper = readFileSync("src/utils/assetUrl.ts", "utf8");
  assert.match(helper, /import\.meta\.env\.BASE_URL/);
});

test("static manifest resources remain valid under a non-root base URL", () => {
  const manifest = JSON.parse(readFileSync("public/manifest.json", "utf8")) as {
    icons?: Array<{ src?: string }>;
  };
  const iconPaths = (manifest.icons ?? []).map((icon) => icon.src ?? "");

  assert.equal(iconPaths.some((path) => path.startsWith("/assets/")), false);
  assert.equal(iconPaths.includes("assets/pwa-icon.png"), true);
  assert.equal(existsSync("public/assets/pwa-icon.png"), true);
});

test("the default production build is guarded as an embedded system UI build", () => {
  const packageJson = JSON.parse(readFileSync("package.json", "utf8")) as {
    scripts?: Record<string, string>;
  };
  const buildScript = readFileSync("script/build-system-ui.mjs", "utf8");
  const viteConfig = readFileSync("vite.config.ts", "utf8");

  assert.equal(packageJson.scripts?.build, "tsc -b && node ./script/build-system-ui.mjs");
  assert.match(buildScript, /VITE_SYSTEM_UI_BUILD = "1"/);
  assert.match(buildScript, /VITE_BASE_URL = "\/system-assets\/"/);
  assert.match(buildScript, /system-assets\/assets\/entry-/);
  assert.match(viteConfig, /mode !== "development"/);
  assert.match(viteConfig, /process\.env\.VITE_SYSTEM_UI_BUILD !== "0"/);
});

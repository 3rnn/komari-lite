import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const frontendRoot = process.cwd();
const embeddedRoot = path.resolve(frontendRoot, "../backend/web/public/systemUI/dist");

function files(root: string, relative = ""): string[] {
  return readdirSync(path.join(root, relative), { withFileTypes: true }).flatMap((entry) => {
    const item = path.join(relative, entry.name);
    if (entry.isDirectory()) return files(root, item);
    return entry.isFile() ? [item] : [];
  }).sort();
}

test("production system UI build synchronizes the exact admin assets embedded by Go", () => {
  execFileSync(process.execPath, ["./script/build-system-ui.mjs"], {
    cwd: frontendRoot,
    stdio: "pipe",
  });

  const builtFiles = files(path.join(frontendRoot, "dist"));
  const embeddedFiles = files(embeddedRoot);
  assert.deepEqual(embeddedFiles, builtFiles, "embedded system UI file list must match the production frontend build");

  for (const file of builtFiles) {
    const built = path.join(frontendRoot, "dist", file);
    const embedded = path.join(embeddedRoot, file);
    assert.equal(statSync(embedded).size, statSync(built).size, `size mismatch for ${file}`);
    assert.deepEqual(readFileSync(embedded), readFileSync(built), `embedded asset differs from production build: ${file}`);
  }
});

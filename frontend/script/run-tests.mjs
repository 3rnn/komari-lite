import { readdirSync } from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";

const testDirectory = path.resolve("tests");
const testFiles = readdirSync(testDirectory)
  .filter((name) => name.endsWith(".test.ts"))
  .sort((left, right) => left.localeCompare(right))
  .map((name) => path.join(testDirectory, name));

if (testFiles.length === 0) {
  throw new Error("No tests/*.test.ts files were discovered.");
}

console.log("Discovered " + testFiles.length + " test files.");
const result = spawnSync(process.execPath, ["--test", ...testFiles], {
  stdio: "inherit",
  windowsHide: true,
});

if (result.error) {
  throw result.error;
}
process.exitCode = result.status ?? 1;

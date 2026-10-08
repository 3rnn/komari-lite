import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const version = "1.0.26";
const source = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");

test("panel, Agent, UI, and package metadata match the release tag", () => {
  assert.match(source("../../backend/utils/version.go"), /CurrentVersion = "1\.0\.26"/);
  assert.match(source("../../agent/utils/utils.go"), /const AgentVersion = "1\.0\.26"/);
  assert.match(source("../src/pages/admin/index.tsx"), /const agentReleaseVersion = "1\.0\.26";/);
  const packageJSON = JSON.parse(source("../package.json"));
  const lock = JSON.parse(source("../package-lock.json"));
  assert.equal(packageJSON.version, version);
  assert.equal(lock.version, version);
  assert.equal(lock.packages[""].version, version);
});

test("Agent installers receive a pinned version and never use latest", () => {
  const ui = source("../src/pages/admin/index.tsx");
  assert.equal((ui.match(/"--install-version", agentReleaseVersion/g) ?? []).length, 1);
  assert.equal((ui.match(/windowsInstallCommand\(scriptUrl, args\)/g) ?? []).length, 1);
  assert.doesNotMatch(ui, /powershell\.exe -NoProfile -ExecutionPolicy Bypass -Command/);
  for (const path of [
    "../../backend/web/api/public/agent_installers/install.sh",
    "../../backend/web/api/public/agent_installers/install.ps1",
  ]) {
    assert.doesNotMatch(source(path), /latest/i, `${path} must not advertise a moving version`);
  }
});

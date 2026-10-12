import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const version = "1.0.42";
const source = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");

test("panel and UI package metadata match the development candidate without changing Agent artifacts", () => {
	assert.match(source("../../backend/utils/version.go"), /CurrentVersion = "1\.0\.42"/);
	assert.match(source("../../agent/utils/utils.go"), /const AgentVersion = "1\.0\.37"/);
	assert.match(source("../src/pages/admin/index.tsx"), /const agentReleaseVersion = "1\.0\.37";/);
  const packageJSON = JSON.parse(source("../package.json"));
  const lock = JSON.parse(source("../package-lock.json"));
  assert.equal(packageJSON.version, version);
  assert.equal(lock.version, version);
  assert.equal(lock.packages[""].version, version);
});

test("panel-only release documentation keeps Agent installation pinned to its existing release", () => {
  const readme = source("../../README.md");
  const agentReadme = source("../../agent/readme.md");
  assert.match(readme, /Release `v1\.0\.42` publishes Linux panel binaries/);
  assert.match(readme, /releases\/download\/v1\.0\.42\/komari-oneclick\.py/);
  assert.match(readme, /releases\/download\/v1\.0\.37\/install\.sh/);
  assert.match(readme, /--install-version 1\.0\.37/);
  assert.match(agentReadme, /releases\/download\/v1\.0\.37\//);
  assert.match(source("../../scripts/build-release.sh"), /previous_tag="\$\{3:-v1\.0\.26\}"/);
  assert.match(source("../../RELEASE_NOTES.md"), /# Komari Lite v1\.0\.42/);
  assert.match(source("../../RELEASE_NOTES.md"), /Agent artifacts remain at `1\.0\.37`/);
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

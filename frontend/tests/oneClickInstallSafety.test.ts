import assert from "node:assert/strict";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { linuxInstallCommand } from "../src/utils/shellQuote.ts";

const page = readFileSync("src/pages/admin/index.tsx", "utf8");

test("Linux generated command propagates a failed download rather than reporting success", () => {
  assert.match(page, /linuxInstallCommand\(scriptUrl, args\)/);
  const command = linuxInstallCommand("https://example.test/install.sh", ["-t", "token"]);
  assert.match(command, /bash -o pipefail -c/);
  const dir = mkdtempSync(join(tmpdir(), "komari-download-"));
  try {
    writeFileSync(join(dir, "wget"), "#!/bin/sh\nprintf 'partial script'\nexit 7\n", { mode: 0o755 });
    writeFileSync(join(dir, "sudo"), `#!/bin/sh\nprintf invoked > '${join(dir, "sudo-was-invoked")}'\nexit 0\n`, { mode: 0o755 });
    const result = spawnSync("bash", ["-c", command], {
      encoding: "utf8",
      env: { ...process.env, PATH: `${dir}:/usr/bin:/bin` },
    });
    assert.equal(result.status, 7, result.stderr);
    assert.equal(existsSync(join(dir, "sudo-was-invoked")), false, "never execute even a partial failed download");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("Linux staging runs a completed script with exact quoted arguments and cleans up", () => {
  const dir = mkdtempSync(join(tmpdir(), "komari-success-"));
  try {
    writeFileSync(join(dir, "wget"), "#!/bin/sh\nprintf 'printf \"%%s\\\\n\" \"$@\"\\n' > \"$2\"\n", { mode: 0o755 });
    writeFileSync(join(dir, "sudo"), "#!/bin/sh\nexec \"$@\"\n", { mode: 0o755 });
    const command = linuxInstallCommand("https://example.test/install.sh", ["--token", "a ' b; $(echo hijack)"]);
    const result = spawnSync("bash", ["-c", command], {
      encoding: "utf8",
      env: { ...process.env, PATH: `${dir}:/usr/bin:/bin` },
    });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, "--token\na ' b; $(echo hijack)\n");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("unsupported macOS install option is absent, including saved legacy profile fallback", () => {
  assert.doesNotMatch(page, /value="macos"|case "macos"|zsh <\(curl|bash <\(curl/);
  assert.match(page, /profile\.platform === "windows" \? "windows" : "linux"/);
});
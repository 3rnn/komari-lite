import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import test from "node:test";

// Guard the lite UI: removed subsystems must have no remaining entry points.
// Otherwise a page may render while its API responds with 404.
const read = (path: string) => readFileSync(path, "utf8");

test("external SSO surfaces are gone from the admin UI", () => {
  assert.equal(
    existsSync("src/pages/admin/settings/sign-on.tsx"),
    false,
    "sign-on settings page should be deleted",
  );
  assert.doesNotMatch(read("src/components/Login.tsx"), /\/api\/oauth/);
  assert.doesNotMatch(read("src/pages/admin/account.tsx"), /oauth2\/(bind|unbind)/);
  assert.doesNotMatch(read("tests/../src/routes.ts"), /sign-on/);
  const accountSecurity = read("src/pages/admin/settings/account-security.tsx");
  assert.doesNotMatch(accountSecurity, /SignOnSettings|sign-on/);
});

test("terminal, clipboard and command-task surfaces stay deleted", () => {
  const files = [...readdirSync("src/pages/admin"), ...readdirSync("src/pages/admin/settings")];
  for (const gone of ["exec.tsx", "terminal.tsx", "xtermjs.tsx"]) {
    assert.equal(files.includes(gone), false, `${gone} should be deleted`);
  }
  const locales = readdirSync("src/i18n/locales").filter((f) => f.endsWith(".json"));
  for (const filename of locales) {
    const locale = JSON.parse(read(`src/i18n/locales/${filename}`));
    assert.equal(locale.command_clipboard, undefined, `${filename}: command_clipboard`);
    assert.equal(locale.settings?.xtermjs, undefined, `${filename}: settings.xtermjs`);
    assert.equal(locale.settings?.sign_on, undefined, `${filename}: settings.sign_on`);
    assert.equal(locale.settings?.sso, undefined, `${filename}: settings.sso`);
  }
});

test("one-click deployment pins installer and binaries to a GitHub Release", () => {
  const source = read("src/pages/admin/index.tsx");
  assert.match(source, /const agentReleaseVersion = "1\.0\.13";/);
  assert.match(source, /github\.com\/3rnn\/komari-lite\/releases\/download\/v\$\{agentReleaseVersion\}/);
  assert.doesNotMatch(source, /\/releases\/latest/);
  assert.doesNotMatch(source, /panelAgentDistribution\(host\)|\/agent\/download/);
});

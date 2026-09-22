import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const loginSource = readFileSync(
  new URL("../src/components/Login.tsx", import.meta.url),
  "utf8",
);
const restrictedLoginSource = readFileSync(
  new URL("../src/components/RestrictedLoginDialog.tsx", import.meta.url),
  "utf8",
);
const loginIdentitySource = readFileSync(
  new URL("../src/components/LoginIdentityHeader.tsx", import.meta.url),
  "utf8",
);
const mainSource = readFileSync(
  new URL("../src/main.tsx", import.meta.url),
  "utf8",
);
const legacyUpgradeSource = readFileSync(
  new URL("../src/pages/admin/update_1_2_7.tsx", import.meta.url),
  "utf8",
);

test("standalone login keeps page controls outside the card", () => {
  assert.match(loginSource, /fixed right-4 top-4 z-10/);
  assert.doesNotMatch(loginSource, /mb-3 flex justify-end gap-2/);
});

test("standalone login renders the complete favicon without a decorative frame", () => {
  assert.match(loginSource, /<LoginIdentityHeader \/>/);
  assert.match(
    loginIdentitySource,
    /<img src="\/favicon\.ico" alt="" className="size-12 shrink-0 object-contain" \/>/,
  );
  assert.doesNotMatch(loginIdentitySource, /bg-\[var\(--accent-a3\)\]/);
});

test("public and restricted login dialogs reuse the same identity header", () => {
  assert.equal((loginSource.match(/<LoginIdentityHeader dialog \/>/g) ?? []).length, 1);
  assert.equal((restrictedLoginSource.match(/<LoginIdentityHeader dialog \/>/g) ?? []).length, 1);
  assert.match(loginSource, /<AppDialogContent maxWidth="420px">/);
  assert.match(restrictedLoginSource, /<AppDialogContent[\s\S]{0,120}maxWidth="420px"/);
});

test("login fields use localized placeholders and matching unadorned inputs", () => {
  assert.match(loginSource, /placeholder=\{t\("login\.username_placeholder"\)\}/);
  assert.match(loginSource, /placeholder=\{t\("login\.password_placeholder"\)\}/);
  assert.doesNotMatch(loginSource, /LockKeyhole/);
  assert.equal((loginSource.match(/className="text-\[15px\]"/g) ?? []).length, 2);
});

test("restricted login uses the same unadorned localized fields", () => {
  assert.match(
    restrictedLoginSource,
    /placeholder=\{t\("login\.username_placeholder"\)\}/,
  );
  assert.match(
    restrictedLoginSource,
    /placeholder=\{t\("login\.password_placeholder"\)\}/,
  );
  assert.doesNotMatch(restrictedLoginSource, /LockKeyhole/);
  assert.equal(
    (restrictedLoginSource.match(/className="text-\[15px\]"/g) ?? []).length,
    2,
  );
  assert.match(loginIdentitySource, /publicInfo\?\.sitename \|\| "Komari Lite"/);
  assert.doesNotMatch(
    restrictedLoginSource,
    /<Dialog\.Title>\{t\("login\.title"\)\}<\/Dialog\.Title>/,
  );
});

test("restricted guide routes provide public site information to the login card", () => {
  assert.match(
    mainSource,
    /isRestrictedGuideRoute \? \(\s*<PublicInfoProvider>[\s\S]*?\{routing\}[\s\S]*?<\/PublicInfoProvider>/,
  );
});

test("both storage upgrade routes reuse the shared restricted login card", () => {
  assert.match(legacyUpgradeSource, /<RestrictedLoginDialog/);
  assert.doesNotMatch(legacyUpgradeSource, /const LoginDialog =/);
});

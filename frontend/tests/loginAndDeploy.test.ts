import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import test from "node:test";

// 本轮三项改动的守卫：
// 1) 后台「反向代理」入口彻底删除；
// 2) 一键部署指令的下载来源跟随域名（未设置脚本域名时用当前访问域名）；
// 3) 开启 2FA 后登录页有独立的两步验证步骤。
const read = (path: string) => readFileSync(path, "utf8");
const locales = readdirSync("src/i18n/locales").filter((f) => f.endsWith(".json"));

test("the reverse proxy section is gone from menu, routes and code", () => {
  const menu = read("src/config/menuConfig.json");
  assert.equal(menu.includes("/admin/settings/reverse-proxy"), false, "menu still links it");
  assert.equal(menu.includes("settings.reverse_proxy"), false, "menu still labels it");

  const routes = read("src/routes.ts");
  assert.equal(routes.includes("reverse-proxy"), false, "routes still register it");
  assert.equal(existsSync("src/pages/admin/settings/reverse-proxy.tsx"), false);
  assert.equal(existsSync("src/lib/https.ts"), false, "HTTPS client should go with the page");
  assert.equal(existsSync("tests/reverseProxy.test.ts"), false);

  for (const filename of locales) {
    const locale = JSON.parse(read(`src/i18n/locales/${filename}`));
    assert.equal(locale.settings.reverse_proxy, undefined, `${filename}: settings.reverse_proxy`);
  }
});

test("the deploy command always derives its download source from the live domain", () => {
  const source = read("src/pages/admin/index.tsx");
  assert.match(source, /function resolveAgentSource\(/);
  assert.match(source, /window\.location\.origin/);
  assert.match(source, /normalizeOptionalServiceUrl\(configured\)/);
  assert.match(source, /fromSetting: Boolean\(configured\)/);
  // 两处命令生成都走同一个解析函数，不再各自内联判断
  assert.equal(
    (source.match(/resolveAgentSource\(settings\?\.script_domain\)\.host/g) ?? []).length,
    2,
  );
  // 两处对话框都要提示来源
  assert.equal((source.match(/<AgentSourceHint /g) ?? []).length, 2);
  for (const key of [
    "installSource",
    "installSourceFromSetting",
    "installSourceFromOrigin",
    "installSourceWarning",
  ]) {
    for (const filename of locales) {
      const locale = JSON.parse(read(`src/i18n/locales/${filename}`));
      assert.equal(typeof locale.admin.nodeTable[key], "string", `${filename}: ${key}`);
    }
  }
});

test("the login page treats 2FA as a real second step", () => {
  const login = read("src/components/Login.tsx");
  assert.match(login, /const INVALID_2FA_MESSAGE = "Invalid 2FA code";/);
  assert.match(login, /twoFac\.replace\(\/\\D\/g, ""\)\.slice\(0, 6\)/);
  assert.match(login, /maxLength=\{6\}/);
  assert.match(login, /autoComplete="one-time-code"/);
  assert.match(login, /autoFocus/);
  assert.match(login, /require2FA && twoFactorDigits\.length !== 6/);
  assert.match(login, /login\.two_factor_verify/);
  assert.match(login, /login\.two_factor_back/);
  assert.match(login, /resetTwoFactorStep/);
  // 进入验证步骤后不再重复显示用户名/密码输入框
  assert.match(login, /passwordLoginEnabled && require2FA \?/);
  assert.match(login, /passwordLoginEnabled && !require2FA \?/);

  for (const key of [
    "two_factor_title",
    "two_factor_hint",
    "two_factor_trouble",
    "two_factor_invalid_hint",
    "two_factor_verify",
    "two_factor_back",
    "two_factor_account",
    "two_factor_code_label",
  ]) {
    for (const filename of locales) {
      const locale = JSON.parse(read(`src/i18n/locales/${filename}`));
      assert.equal(typeof locale.login[key], "string", `${filename}: ${key}`);
    }
  }
});

test("the 2FA code is sent whenever the user typed one", () => {
  const auth = read("src/utils/adminAuth.ts");
  assert.doesNotMatch(auth, /!twoFactorEnabled/, "the branch that dropped the code must be gone");
  assert.match(auth, /\.\.\.\(twoFactorCode \? \{ "2fa_code": twoFactorCode \} : \{\}\)/);
  assert.doesNotMatch(auth, /twoFactorEnabled/);
});

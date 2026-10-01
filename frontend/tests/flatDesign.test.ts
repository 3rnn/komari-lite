import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const css = readFileSync(new URL("../src/global.css", import.meta.url), "utf8");
const login = readFileSync(new URL("../src/components/Login.tsx", import.meta.url), "utf8");

function rule(selector: string): string {
  const start = css.indexOf(`${selector} {`);
  assert.notEqual(start, -1, `Missing rule: ${selector}`);
  return css.slice(start, css.indexOf("}", start) + 1);
}

test("admin surfaces use compact corners and flat dialog borders", () => {
  assert.match(css, /:root\s*\{\s*--radius:\s*0\.5rem/);
  const dialog = rule('html[data-admin-shell-active="true"] .rt-BaseDialogContent');
  assert.match(dialog, /border:\s*1px solid var\(--gray-a5\)/);
  assert.match(dialog, /box-shadow:\s*none/);
  assert.doesNotMatch(rule(".km-theme-config-scroll-button"), /box-shadow/);
});

test("placeholder surfaces avoid decorative gradients and continuous shimmer", () => {
  assert.match(rule(".km-theme-preview-skeleton"), /background:\s*var\(--gray-3\)/);
  assert.doesNotMatch(rule(".km-theme-preview-skeleton"), /gradient|animation:/);
  assert.doesNotMatch(css, /@keyframes km-theme-preview-skeleton/);
});

test("login card uses a flat bordered surface", () => {
  assert.match(login, /<section className="km-panel p-5 sm:p-7">/);
  assert.match(rule(".km-panel"), /border:\s*1px solid var\(--km-border\)/);
});

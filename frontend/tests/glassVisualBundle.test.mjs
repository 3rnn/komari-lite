import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";

const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const html = readFileSync(new URL("index.html", dist), "utf8");

test("Glass HTML loads one content-fingerprinted visual stylesheet after upstream CSS", () => {
  const matches = [...html.matchAll(/glass-visual-([a-f0-9]{8})\.css/g)];
  assert.ok(matches.length >= 1, "the public page must load its visual stylesheet");
  assert.ok(matches.every(([, hash]) => hash === matches[0][1]));
  const name = matches[0][0];
  const css = readFileSync(new URL(name, dist));
  assert.equal(createHash("sha256").update(css).digest("hex").slice(0, 8), matches[0][1]);
  assert.ok(html.indexOf("1j6ltxvh3a3zn.css") < html.indexOf(name), "overrides must follow the vendor CSS");
});

test("Glass starts dark by default while preserving a saved light preference", () => {
  const manifest = JSON.parse(readFileSync(new URL("../komari-theme.json", dist), "utf8"));
  const appearance = manifest.configuration.data.find((item) => item.key === "defaultAppearance");
  assert.equal(appearance.default, "dark");
  assert.equal(appearance.options, "system,light,dark");
  assert.match(html, /localStorage\.getItem\("appearance"\)\|\|"dark"/);
  assert.match(html, /a==="dark"\|\|\(a!=="light"/);
});

test("Glass dark surfaces use opaque deep teal and quiet rectangular outlines", () => {
  const [, hash] = [...html.matchAll(/glass-visual-([a-f0-9]{8})\.css/g)][0] ?? [];
  assert.ok(hash);
  const css = readFileSync(new URL(`glass-visual-${hash}.css`, dist), "utf8");
  assert.match(css, /\.dark\s*\{[^}]*--background:\s*#071f21\b/);
  assert.match(css, /\.dark\s*\{[^}]*--node-card-bg:\s*#0b292a\b/);
  assert.match(css, /\.node-card\s*\{[^}]*border-radius:\s*4px\b[^}]*box-shadow:\s*none/);
  assert.match(css, /\.glass-panel\s*\{[^}]*box-shadow:\s*none/);
  assert.match(css, /html\[data-blur=on\] \.glass-panel,\s*html\[data-blur=off\] \.glass-panel\s*\{[^}]*backdrop-filter:\s*none/);
  assert.match(css, /\.node-data-panel\s*\{[^}]*border:\s*1px solid var\(--border\)/);
  assert.match(css, /@media \(max-width:\s*640px\)/);
});

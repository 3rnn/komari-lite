import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";

const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const html = readFileSync(new URL("index.html", dist), "utf8");
const visualSource = readFileSync(new URL("../script/glass-visual.css", import.meta.url), "utf8");
const rule = (selector) => {
  const match = visualSource.match(new RegExp(`${selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}\\s*\\{([^}]*)\\}`));
  assert.ok(match, `missing ${selector} override`);
  return match[1];
};

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
  assert.match(css, /\.dark\s*\{[^}]*--background:\s*#072a28\b/);
  assert.match(css, /\.dark\s*\{[^}]*--node-card-bg:\s*#0a302d\b/);
  assert.match(css, /\.node-card\s*\{[^}]*border-radius:\s*4px\b[^}]*box-shadow:\s*none/);
  assert.match(css, /\.glass-panel\s*\{[^}]*box-shadow:\s*none/);
  assert.match(css, /html\[data-blur=on\] \.glass-panel,\s*html\[data-blur=off\] \.glass-panel\s*\{[^}]*backdrop-filter:\s*none/);
  assert.match(css, /\.node-data-panel\s*\{[^}]*border:\s*1px solid var\(--border\)/);
  assert.match(css, /@media \(max-width:\s*640px\)/);
});

test("public Glass palette aligns with admin canvas, panels and primary in both modes", () => {
  assert.match(rule(":root"), /--primary:\s*#146b58\b/);
  assert.match(rule(":root"), /--ring:\s*#146b58\b/);
  assert.match(rule(".dark"), /--background:\s*#072a28\b/);
  assert.match(rule(".dark"), /--card-solid:\s*#0a302d\b/);
  assert.match(rule(".dark"), /--node-card-bg:\s*#0a302d\b/);
  assert.match(rule(".dark"), /--primary:\s*#77dcb3\b/);
  assert.match(rule(".dark"), /--primary-foreground:\s*#072a28\b/);
  assert.match(rule(".dark"), /--ring:\s*#77dcb3\b/);
});

test("header icon and segmented controls have opaque hover, selected and keyboard focus states", () => {
  assert.match(rule(".icon-btn:hover"), /background:\s*var\(--glass-control\)/);
  assert.match(rule(".segment-btn:hover"), /background:\s*var\(--node-card-hover\)/);
  assert.match(rule(".segment-btn-active"), /background:\s*var\(--primary\)/);
  assert.match(rule(".segment-btn-active"), /color:\s*var\(--primary-foreground\)/);
  assert.match(rule(".segment-btn-active:hover"), /background:\s*var\(--primary\)/);
  assert.match(rule(".segment-btn:focus-visible"), /outline:\s*2px solid var\(--ring\)/);
  assert.match(rule(".icon-btn:focus-visible"), /outline:\s*2px solid var\(--ring\)/);
});

test("ping strips use a restrained five-step palette in light and dark modes", () => {
  for (const name of ["ping-good", "ping-calm", "ping-watch", "ping-high", "ping-critical"]) {
    assert.match(rule(":root"), new RegExp(`--${name}:\\s*#[0-9a-f]{6}\\b`));
    assert.match(rule(".dark"), new RegExp(`--${name}:\\s*#[0-9a-f]{6}\\b`));
  }
  const colors = ["good", "calm", "watch", "high", "critical"];
  for (const [index, name] of colors.entries()) {
    assert.match(visualSource, new RegExp(`\\.node-card \\.group\\\\/ping-panel \\.bg-signal-${index + 1}\\s*\\{[^}]*background-color:\\s*var\\(--ping-${name}\\)`));
  }
  assert.match(visualSource, /\.node-card \.group\\\/ping-panel \.ping-timeline\s*\{[^}]*max-height:\s*10px/);
});

test("latency sample width changes without changing strip height or packet-loss bars", () => {
  assert.match(visualSource, /\.node-card \.group\\\/ping-panel \.ping-timeline\.ping-latency \.group\\\/ping-bar > span:first-child\s*\{[^}]*width:\s*min\(68%,\s*4px\)/);
  assert.doesNotMatch(visualSource, /\.ping-timeline\.ping-latency[^}]*\b(?:height|max-height|min-height)\s*:/);
});

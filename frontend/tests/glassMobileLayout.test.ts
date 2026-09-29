import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const themeDir = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);

// The bundled Glass theme has no editable source in this repository. Keep the
// narrow-phone override separate from its fingerprinted generated chunks.
test("compact phones stack overview metrics instead of clipping their text", () => {
  const html = readFileSync(new URL("index.html", themeDir), "utf8");
  const stylesheet = "/mobile-layout-v1.css";
  assert.match(html, /<link rel="stylesheet" href="\/mobile-layout-v1\.css"\s*\/>/);

  const css = readFileSync(new URL(stylesheet.slice(1), themeDir), "utf8");
  assert.match(css, /@media\s*\(max-width:\s*389px\)/);
  assert.match(css, /\.overview-strip\s*\{\s*grid-template-columns:\s*minmax\(0,\s*1fr\)/);
  assert.match(css, /\.overview-strip\s*>\s*\.overview-stat:not\(:last-child\)\s*\{[^}]*border-bottom:/);
});

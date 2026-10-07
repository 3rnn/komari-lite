import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const page = readFileSync(new URL("../src/pages/admin/index.tsx", import.meta.url), "utf8");
const css = readFileSync(new URL("../src/global.css", import.meta.url), "utf8");

test("server workbench groups filters and accessible search without a forced wide table", () => {
  assert.match(page, /className="admin-server-toolbar"/);
  assert.match(page, /aria-label=\{t\("admin.nodeTable.searchByName"\)\}/);
  assert.doesNotMatch(page, /admin-node-table min-w-\[1136px\]/);
  assert.match(css, /\.admin-server-toolbar\s*\{[^}]*display: flex/s);
  assert.match(css, /\.admin-node-table tbody td:nth-child\(3\)::before\s*\{\s*display: none/s);
});

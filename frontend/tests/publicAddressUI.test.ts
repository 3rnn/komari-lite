import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const admin = readFileSync(new URL("../src/pages/admin/index.tsx", import.meta.url), "utf8");
const css = readFileSync(new URL("../src/global.css", import.meta.url), "utf8");

test("address layout overrides table nowrap with a separate fixed copy column", () => {
  assert.match(css, /\.admin-address-row\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\) 1\.25rem/s);
  assert.match(css, /\.admin-address-value\s*\{[^}]*white-space:\s*normal;[^}]*overflow-wrap:\s*anywhere/s);
  assert.match(css, /\.admin-node-row:has\(\.admin-additional-addresses\[open\]\)\s*>\s*td\s*\{[^}]*vertical-align:\s*top !important/s);
  assert.match(admin, /admin-address-value/);
  assert.doesNotMatch(admin, /!align-middle/);
});

const list = readFileSync(new URL("../src/components/admin/AdditionalPublicAddresses.tsx", import.meta.url), "utf8");

test("admin table and detail dialog keep primary addresses visible and provide a collapsed additional list", () => {
  assert.match(admin, /\["IPv4", node\.ipv4/);
  assert.match(admin, /\["IPv6", node\.ipv6/);
  assert.match(admin, /ReadOnlyDetailField label="IPv4" value=\{node\.ipv4\}/);
  assert.match(admin, /ReadOnlyDetailField label="IPv6" value=\{node\.ipv6\}/);
  assert.ok((admin.match(/<AdditionalPublicAddresses\b/g) ?? []).length >= 2);
  assert.match(list, /<details\b/);
  assert.match(list, /<summary\b/);
  assert.match(list, /onToggle=/);
  assert.match(list, /expanded &&/);
  assert.match(list, /admin-additional-address-list/);
  assert.match(list, /additionalPublicAddresses\(/);
  assert.match(list, /writeClipboardText\(/);
});

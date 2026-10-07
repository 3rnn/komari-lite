import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const admin = readFileSync(new URL("../src/pages/admin/index.tsx", import.meta.url), "utf8");
const css = readFileSync(new URL("../src/global.css", import.meta.url), "utf8");
const list = readFileSync(new URL("../src/components/admin/AdditionalPublicAddresses.tsx", import.meta.url), "utf8");

test("additional addresses use a lazy modal instead of expanding the server row", () => {
  assert.doesNotMatch(list, /<details\b|<summary\b/);
  assert.match(list, /<Dialog.Root open=\{expanded\} onOpenChange=\{setExpanded\}/);
  assert.match(list, /<Dialog.Trigger>/);
  assert.match(list, /expanded &&/);
  assert.match(list, /<AppDialogContent/);
  assert.match(list, /<Dialog.Close>/);
  assert.match(list, /additionalPublicAddresses\(/);
  assert.match(list, /writeClipboardText\(address\)/);
  assert.match(list, /\["ipv4", "ipv6"\]/);
});

test("compact primaries and modal addresses have independent responsive layouts", () => {
  assert.match(css, /\.admin-network-primary\s*\{[^}]*grid-template-columns:/s);
  assert.match(css, /\.admin-network-primary \.admin-address-value\s*\{[^}]*text-overflow:\s*ellipsis/s);
  assert.match(css, /\.admin-address-value\s*\{[^}]*white-space:\s*normal;[^}]*overflow-wrap:\s*anywhere/s);
  assert.doesNotMatch(css, /admin-node-row:has\(\.admin-additional-addresses/);
  assert.match(admin, /\["IPv4", node\.ipv4/);
  assert.match(admin, /\["IPv6", node\.ipv6/);
  assert.match(admin, /ReadOnlyDetailField label="IPv4" value=\{node\.ipv4\}/);
  assert.match(admin, /ReadOnlyDetailField label="IPv6" value=\{node\.ipv6\}/);
  assert.ok((admin.match(/<AdditionalPublicAddresses\b/g) ?? []).length >= 2);
});

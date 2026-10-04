import assert from "node:assert/strict";
import test from "node:test";

import { additionalPublicAddresses } from "../src/utils/publicAddresses.ts";

test("legacy nodes show only their existing primary addresses", () => {
  assert.deepEqual(additionalPublicAddresses(undefined, "8.8.8.8", ""), []);
});

test("multiple IPv4 and IPv6 identities exclude primaries and duplicates", () => {
  const addresses = [
    { address: "8.8.8.8", family: "ipv4" as const, primary: true },
    { address: "9.9.9.9", family: "ipv4" as const, interface: "eth1" },
    { address: "9.9.9.9", family: "ipv4" as const, interface: "eth2" },
    { address: "2001:4860::1", family: "ipv6" as const, primary: true },
    { address: "2001:4860::2", family: "ipv6" as const, interface: "eth2" },
    { address: "2001:4860::3", family: "ipv6" as const, interface: "eth3" },
  ];
  assert.deepEqual(additionalPublicAddresses(addresses, "8.8.8.8", "2001:4860::1"), [
    addresses[1], addresses[4], addresses[5],
  ]);
});

test("primary marker also excludes alternate IPv6 spellings", () => {
  const addresses = [
    { address: "2001:4860::1", family: "ipv6" as const, primary: true },
    { address: "2001:4860::2", family: "ipv6" as const },
  ];
  assert.deepEqual(additionalPublicAddresses(addresses, "", "2001:4860:0:0:0:0:0:1"), [addresses[1]]);
});

test("single-stack nodes can list additional identities without the other family", () => {
  const addresses = [
    { address: "2001:4860::1", family: "ipv6" as const, primary: true },
    { address: "2001:4860::2", family: "ipv6" as const },
  ];
  assert.deepEqual(additionalPublicAddresses(addresses, "", "2001:4860::1"), [addresses[1]]);
});

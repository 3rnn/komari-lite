import assert from "node:assert/strict";
import test from "node:test";

import { createLoadAlertBootstrapResource } from "../src/utils/loadAlertBootstrap.ts";

test("load alert bootstrap suspends once and reuses the account-scoped result", async () => {
  let resolveLoad!: (value: Array<{ id: number; name: string }>) => void;
  let requests = 0;
  const resource = createLoadAlertBootstrapResource(() => {
    requests += 1;
    return new Promise((resolve) => {
      resolveLoad = resolve;
    });
  });

  let pending: Promise<void> | undefined;
  try {
    resource.read("account-a");
  } catch (reason) {
    pending = reason as Promise<void>;
  }
  assert.ok(pending instanceof Promise);
  assert.equal(requests, 1);

  let duplicate: Promise<void> | undefined;
  try {
    resource.read("account-a");
  } catch (reason) {
    duplicate = reason as Promise<void>;
  }
  assert.equal(duplicate, pending);
  assert.equal(requests, 1);

  resolveLoad([{ id: 7, name: "Disk" }]);
  await pending;
  assert.deepEqual(resource.read("account-a").data, [{ id: 7, name: "Disk" }]);
  assert.equal(requests, 1);
});

test("load alert bootstrap does not leak results between accounts", async () => {
  const resource = createLoadAlertBootstrapResource(async () => []);
  resource.update("account-a", [{ id: 1, name: "A" }]);

  assert.deepEqual(resource.read("account-a").data, [{ id: 1, name: "A" }]);
  let pending: Promise<void> | undefined;
  try {
    resource.read("account-b");
  } catch (reason) {
    pending = reason as Promise<void>;
  }
  assert.ok(pending instanceof Promise);
  await pending;
  assert.deepEqual(resource.read("account-b").data, []);
});

test("load alert bootstrap expires after thirty seconds", () => {
  const resource = createLoadAlertBootstrapResource(async () => []);
  const snapshot = resource.update("account-a", []);

  assert.equal(resource.isStale(snapshot, snapshot.fetchedAt + 29_999), false);
  assert.equal(resource.isStale(snapshot, snapshot.fetchedAt + 30_000), true);
});

import assert from "node:assert/strict";
import test from "node:test";

import {
  ADMIN_IDLE_WARMUP_FALLBACK_MS,
  ADMIN_IDLE_WARMUP_SLICE_TIMEOUT_MS,
  expandAdminPreloadTargets,
  getIdleAdminWarmupTargets,
  scheduleIdleAdminWarmup,
  shouldPreloadAdminRoutes,
  type AdminIdleWarmupTimers,
} from "../src/utils/adminPreload.ts";

test("idle warmup skips the current page and throttles 3g", () => {
  assert.deepEqual(getIdleAdminWarmupTargets("/admin"), [
    "/admin/servers",
    "/admin/ping",
  ]);
  assert.deepEqual(getIdleAdminWarmupTargets("/admin/ping"), [
    "/admin/servers",
  ]);
  assert.deepEqual(
    getIdleAdminWarmupTargets("/admin", { effectiveType: "3g" }),
    ["/admin/servers"],
  );
  assert.deepEqual(
    getIdleAdminWarmupTargets("/admin", { effectiveType: "2g" }),
    [],
  );
  assert.equal(shouldPreloadAdminRoutes({ saveData: true }), false);
});

test("fixed Glass settings preload directly without the removed settings-theme route", () => {
  assert.deepEqual(expandAdminPreloadTargets("/admin/theme_managed?tab=1"), [
    "/admin/theme_managed",
  ]);
  assert.deepEqual(expandAdminPreloadTargets("/admin/ping"), ["/admin/ping"]);
});

test("idle warmup loads one route per idle slice and can be cancelled", async () => {
  const loaded: string[] = [];
  const idleQueue: Array<() => void> = [];
  let startDelay: (() => void) | undefined;
  const timers: AdminIdleWarmupTimers = {
    requestIdleCallback: (callback, options) => {
      assert.equal(options?.timeout, ADMIN_IDLE_WARMUP_SLICE_TIMEOUT_MS);
      idleQueue.push(() =>
        callback({ timeRemaining: () => 12, didTimeout: false }),
      );
      return idleQueue.length;
    },
    cancelIdleCallback: () => {
      idleQueue.length = 0;
    },
    setTimeout: (callback) => {
      startDelay = callback;
      return 1;
    },
    clearTimeout: () => {
      startDelay = undefined;
    },
  };

  const stop = scheduleIdleAdminWarmup({
    targets: ["/admin/servers", "/admin/ping"],
    preload: async (target) => {
      loaded.push(target);
    },
    startDelayMs: 1200,
    timers,
  });

  assert.equal(loaded.length, 0);
  assert.equal(idleQueue.length, 0);
  startDelay?.();
  assert.equal(idleQueue.length, 1);

  idleQueue.shift()?.();
  await Promise.resolve();
  assert.deepEqual(loaded, ["/admin/servers"]);
  assert.equal(idleQueue.length, 1);

  idleQueue.shift()?.();
  await Promise.resolve();
  assert.deepEqual(loaded, ["/admin/servers", "/admin/ping"]);

  stop();
  idleQueue.shift()?.();
  await Promise.resolve();
  assert.deepEqual(loaded, ["/admin/servers", "/admin/ping"]);
});

test("idle warmup falls back to a timer when requestIdleCallback is missing", async () => {
  const loaded: string[] = [];
  const timeouts: Array<{ callback: () => void; delay: number }> = [];
  const timers: AdminIdleWarmupTimers = {
    setTimeout: (callback, delay) => {
      timeouts.push({ callback, delay });
      return timeouts.length;
    },
    clearTimeout: () => {},
  };

  scheduleIdleAdminWarmup({
    targets: ["/admin/servers", "/admin/ping"],
    preload: async (target) => {
      loaded.push(target);
    },
    startDelayMs: 50,
    timers,
  });

  assert.equal(timeouts[0]?.delay, 50);
  timeouts.shift()?.callback();
  assert.equal(timeouts[0]?.delay, ADMIN_IDLE_WARMUP_FALLBACK_MS);
  timeouts.shift()?.callback();
  await Promise.resolve();
  assert.deepEqual(loaded, ["/admin/servers"]);
});

import assert from "node:assert/strict";
import test from "node:test";
import { createMemoryRouter } from "react-router-dom";

import {
  ADMIN_ROUTE_PROGRESS_MIN_VISIBLE_MS,
  getAdminRouteViewKey,
  getAdminRouteProgressHideDelay,
  isAdminRouteViewReady,
  promoteAdminRouteView,
  stageAdminRouteView,
  type RouteViewportState,
} from "../src/utils/adminRouteViewport.ts";

test("promotes only a non-empty route whose first content is ready", () => {
  assert.equal(
    isAdminRouteViewReady({
      hasPendingMarker: true,
      childElementCount: 1,
      textContent: "loading",
    }),
    false,
  );
  assert.equal(
    isAdminRouteViewReady({
      hasPendingMarker: false,
      childElementCount: 0,
      textContent: "",
    }),
    false,
  );
  assert.equal(
    isAdminRouteViewReady({
      hasPendingMarker: false,
      childElementCount: 1,
      textContent: "ready",
    }),
    true,
  );
});

const initialState = (): RouteViewportState => ({
  activeKey: "servers",
  pendingKey: null,
  views: [{ key: "servers", outlet: "server page" }],
});

test("keeps the current admin page while the next route is staged", () => {
  const staged = stageAdminRouteView(initialState(), "logs", "logs page");

  assert.equal(staged.activeKey, "servers");
  assert.equal(staged.pendingKey, "logs");
  assert.deepEqual(staged.views.map((view) => view.key), ["servers", "logs"]);
});

test("promotes the prepared route without remounting its stored outlet", () => {
  const outlet = { page: "logs" };
  const staged = stageAdminRouteView(initialState(), "logs", outlet);
  const promoted = promoteAdminRouteView(staged, "logs");

  assert.equal(promoted.activeKey, "logs");
  assert.equal(promoted.pendingKey, null);
  assert.equal(promoted.views.length, 1);
  assert.equal(promoted.views[0].outlet, outlet);
});

test("a newer navigation replaces an unfinished staged route", () => {
  const logsStaged = stageAdminRouteView(initialState(), "logs", "logs page");
  const settingsStaged = stageAdminRouteView(
    logsStaged,
    "settings",
    "settings page",
  );

  assert.equal(settingsStaged.activeKey, "servers");
  assert.equal(settingsStaged.pendingKey, "settings");
  assert.deepEqual(
    settingsStaged.views.map((view) => view.key),
    ["servers", "settings"],
  );
  assert.equal(promoteAdminRouteView(settingsStaged, "logs"), settingsStaged);
});

test("rapid navigation promotes only the final target after an old ready callback", () => {
  let state = stageAdminRouteView(initialState(), "ping", "ping page");
  state = stageAdminRouteView(state, "servers-settings", "settings page");

  const afterOldReady = promoteAdminRouteView(state, "ping");
  assert.equal(afterOldReady, state);

  const settled = promoteAdminRouteView(afterOldReady, "servers-settings");
  assert.equal(settled.activeKey, "servers-settings");
  assert.equal(settled.pendingKey, null);
  assert.deepEqual(settled.views, [
    { key: "servers-settings", outlet: "settings page" },
  ]);
});

test("an aborted transition and stale callbacks cannot override a later target", () => {
  const pingStaged = stageAdminRouteView(initialState(), "ping", "ping page");
  const aborted = stageAdminRouteView(pingStaged, "servers", "server page");

  assert.equal(promoteAdminRouteView(aborted, "ping"), aborted);

  const otherStaged = stageAdminRouteView(
    aborted,
    "servers-settings",
    "settings page",
  );
  assert.equal(
    promoteAdminRouteView(otherStaged, "ping"),
    otherStaged,
  );
  assert.equal(
    promoteAdminRouteView(otherStaged, "servers-settings").activeKey,
    "servers-settings",
  );
});

test("the final-target reducer remains stable across repeated rapid sequences", () => {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    let state = stageAdminRouteView(initialState(), "ping", "ping page");
    state = stageAdminRouteView(state, "servers-settings", "settings page");
    state = promoteAdminRouteView(state, "ping");
    state = promoteAdminRouteView(state, "servers-settings");

    assert.equal(state.activeKey, "servers-settings");
    assert.equal(state.pendingKey, null);
  }
});

test("returning to the active route cancels an unfinished transition", () => {
  const staged = stageAdminRouteView(initialState(), "logs", "logs page");
  const cancelled = stageAdminRouteView(staged, "servers", "server page");

  assert.equal(cancelled.activeKey, "servers");
  assert.equal(cancelled.pendingKey, null);
  assert.deepEqual(cancelled.views.map((view) => view.key), ["servers"]);
});

test("real Router keys do not remount the active route when a pending navigation is cancelled", async (t) => {
  const router = createMemoryRouter([{ path: "*", element: null }], {
    initialEntries: ["/admin/servers"],
  });
  t.after(() => router.dispose());

  const activeOutlet = { search: "Tokyo" };
  const firstLocation = router.state.location;
  const firstKey = getAdminRouteViewKey(firstLocation);
  let state: RouteViewportState<typeof activeOutlet> = {
    activeKey: firstKey,
    pendingKey: null,
    views: [{ key: firstKey, outlet: activeOutlet }],
  };

  await router.navigate("/admin/notification/load");
  const pendingLocation = router.state.location;
  state = stageAdminRouteView(
    state,
    getAdminRouteViewKey(pendingLocation),
    { search: "load page" },
  );

  await router.navigate("/admin/servers");
  const returnedLocation = router.state.location;
  assert.notEqual(returnedLocation.key, firstLocation.key);
  assert.equal(getAdminRouteViewKey(returnedLocation), firstKey);

  const cancelled = stageAdminRouteView(
    state,
    getAdminRouteViewKey(returnedLocation),
    { search: "" },
  );
  assert.equal(cancelled.pendingKey, null);
  assert.equal(cancelled.views.length, 1);
  assert.equal(cancelled.views[0].outlet, activeOutlet);
});

test("visible route progress stays long enough to avoid a flash", () => {
  assert.equal(
    getAdminRouteProgressHideDelay({
      becameVisibleAt: 1000,
      now: 1000 + ADMIN_ROUTE_PROGRESS_MIN_VISIBLE_MS - 40,
    }),
    40,
  );
  assert.equal(
    getAdminRouteProgressHideDelay({
      becameVisibleAt: 1000,
      now: 1000 + ADMIN_ROUTE_PROGRESS_MIN_VISIBLE_MS + 10,
    }),
    0,
  );
});

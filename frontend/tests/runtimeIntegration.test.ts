import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { shouldPreloadAdminRoutes } from "../src/utils/adminPreload.ts";
import { isKomariThemeCacheEntry } from "../src/utils/themeCache.ts";

test("admin warmup respects reduced-data connections", () => {
  assert.equal(shouldPreloadAdminRoutes(), true);
  assert.equal(shouldPreloadAdminRoutes({ effectiveType: "4g" }), true);
  assert.equal(shouldPreloadAdminRoutes({ effectiveType: "2g" }), false);
  assert.equal(shouldPreloadAdminRoutes({ effectiveType: "slow-2g" }), false);
  assert.equal(shouldPreloadAdminRoutes({ saveData: true, effectiveType: "4g" }), false);

  const preload = readFileSync("src/utils/adminPreload.ts", "utf8");
  const routes = readFileSync("src/routes.ts", "utf8");
  const likelyRoutesSource = preload.match(
    /export const LIKELY_ADMIN_ROUTES = \[([\s\S]*?)\] as const/,
  )?.[1];
  assert.ok(likelyRoutesSource);
  assert.deepEqual(
    Array.from(likelyRoutesSource.matchAll(/"([^"]+)"/g), (match) => match[1]),
    [
      "/admin/servers",
      "/admin/ping",
    ],
  );
  assert.doesNotMatch(likelyRoutesSource, /\/admin\/notification\/load/);
  assert.doesNotMatch(likelyRoutesSource, /\/admin\/settings\/site/);
  assert.doesNotMatch(routes, /Object\.values\(adminRoutePreloaders\)/);
  assert.doesNotMatch(routes, /Promise\.allSettled/);

  const layout = readFileSync("src/pages/admin/_layout.tsx", "utf8");
  const viewport = readFileSync(
    "src/components/admin/AdminRouteViewport.tsx",
    "utf8",
  );
  const loading = readFileSync("src/components/loading.tsx", "utf8");
  const settingsSkeleton = readFileSync(
    "src/components/admin/SettingsPageSkeleton.tsx",
    "utf8",
  );
  assert.doesNotMatch(layout, /useDeferredValue/);
  assert.match(
    layout,
    /<AdminRouteViewport\s+fallback=\{<AdminRouteLoading \/>\}\s+outlet=\{outlet\}/,
  );
  assert.match(viewport, /data-admin-route-active=\{active \? "true" : "false"\}/);
  assert.match(
    viewport,
    /<React\.Suspense fallback=\{fallback\}>\{view\.outlet\}<\/React\.Suspense>/,
  );
  assert.match(viewport, /isRouteViewReady\(element\)/);
  assert.match(viewport, /readyFrames >= 2/);
  assert.match(loading, /data-admin-route-pending="true"/);
  assert.match(settingsSkeleton, /data-admin-route-pending="true"/);

  const managedTheme = readFileSync(
    "src/pages/admin/theme_managed.tsx",
    "utf8",
  );
  assert.match(managedTheme, /const \[loading, setLoading\] = useState\(true\)/);
  assert.match(managedTheme, /publicInfoLoading \|\| \(!publicInfo && !publicInfoError\)/);
  assert.match(managedTheme, /if \(!theme\) \{[\s\S]*setLoading\(false\);[\s\S]*setFirstLoading\(false\)/);
});

test("theme switching removes only Komari navigation and theme cache entries", () => {
  const origin = "https://monitor.example";
  assert.equal(isKomariThemeCacheEntry(`${origin}/admin/theme_managed`, origin, "navigate"), true);
  assert.equal(isKomariThemeCacheEntry(`${origin}/themes/emerald/assets/app.js`, origin), true);
  assert.equal(isKomariThemeCacheEntry(`${origin}/system-assets/assets/app.js`, origin), true);
  assert.equal(isKomariThemeCacheEntry(`${origin}/api/clients`, origin), false);
  assert.equal(isKomariThemeCacheEntry("https://other.example/themes/app.js", origin), false);

  const source = readFileSync("src/utils/themeCache.ts", "utf8");
  assert.doesNotMatch(source, /getRegistrations\(\)/);
  assert.doesNotMatch(source, /caches\.delete\(cacheName\)/);
  assert.match(source, /cache\.delete\(request\)/);
});

test("load alert configuration and current-alert requests own separate states", () => {
  const source = readFileSync("src/contexts/LoadAlertContext.tsx", "utf8");
  const page = readFileSync("src/pages/admin/notification/load.tsx", "utf8");
  assert.match(source, /const \[currentError, setCurrentError\]/);
  assert.match(source, /loadAlertBootstrapResource\.read\(accountKey\)/);
  assert.match(source, /const refresh = React\.useCallback\(async \(\) =>/);
  assert.match(source, /finally \{\s*setIsLoading\(false\)/);
  assert.match(page, /isLoading && loadAlerts === null/);
  assert.doesNotMatch(source, /void refresh\(\);\s*\}, \[refresh\]\)/);
});

test("deep routes use an origin-root favicon before backend rewriting", () => {
  const html = readFileSync("index.html", "utf8");
  assert.match(html, /rel="shortcut icon" href="\/favicon\.ico"/);
  assert.doesNotMatch(html, /href="favicon\.ico"/);
});

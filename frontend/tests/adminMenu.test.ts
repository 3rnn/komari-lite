import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  APPEARANCE_MENU_PATH,
  buildAdminMenuItems,
  isAdminMenuPathActive,
  syncSubMenuForLocation,
  toggleSingleSubMenu,
} from "../src/utils/adminMenu.ts";
import type { MenuItem } from "../src/types/menu.ts";

const menuConfig = JSON.parse(
  readFileSync(new URL("../src/config/menuConfig.json", import.meta.url), "utf8"),
) as { menu: MenuItem[]; footer: MenuItem[] };
const adminPanelSource = readFileSync(
  new URL("../src/components/admin/AdminPanelBar.tsx", import.meta.url),
  "utf8",
);
const routesSource = readFileSync(new URL("../src/routes.ts", import.meta.url), "utf8");
const mainSource = readFileSync(new URL("../src/main.tsx", import.meta.url), "utf8");
const adminLayoutSource = readFileSync(new URL("../src/pages/admin/_layout.tsx", import.meta.url), "utf8");
const pingTaskContextSource = readFileSync(new URL("../src/contexts/PingTaskContext.tsx", import.meta.url), "utf8");
const pingTaskPageSource = readFileSync(new URL("../src/pages/admin/pingTask.tsx", import.meta.url), "utf8");
const globalCssSource = readFileSync(new URL("../src/global.css", import.meta.url), "utf8");
const serverPageSource = readFileSync(new URL("../src/pages/admin/index.tsx", import.meta.url), "utf8");
const dashboardPanelsSource = readFileSync(new URL("../src/components/admin/DashboardPanels.tsx", import.meta.url), "utf8");
const selectorSource = readFileSync(new URL("../src/components/Selector.tsx", import.meta.url), "utf8");
const nodeSelectorSource = readFileSync(new URL("../src/components/NodeSelector.tsx", import.meta.url), "utf8");
const checkboxSource = readFileSync(new URL("../src/components/ui/checkbox.tsx", import.meta.url), "utf8");
const selectOrInputSource = readFileSync(new URL("../src/components/ui/select-or-input.tsx", import.meta.url), "utf8");
function allPaths(items: MenuItem[]): string[] {
  return items.flatMap((item) => [item.path, ...allPaths(item.children ?? [])]);
}

test("keeps the admin navigation in the intended groups", () => {
  assert.deepEqual(
    menuConfig.menu.map((item) => item.path),
    [
      "/admin",
      "/admin/servers",
      "/admin/monitoring",
      "/admin/notifications",
      "/admin/appearance",
      "/admin/settings",
    ],
  );

  const systemSettings = menuConfig.menu.find(
    (item) => item.path === "/admin/settings",
  );
  assert.deepEqual(
    systemSettings?.children?.map((item) => item.path),
    [
      "/admin/settings/site",
      "/admin/settings/dashboard",
      "/admin/settings/metrics",
      "/admin/settings/account-security",
      "/admin/settings/general",
    ],
  );

  const paths = allPaths(menuConfig.menu);
  assert.equal(paths.includes("/"), false);
  assert.equal(
    paths.filter((path) => path === "/admin/settings/metrics").length,
    1,
  );
  assert.equal(paths.includes("/admin/account-security"), false);

  const notifications = menuConfig.menu.find(
    (item) => item.path === "/admin/notifications",
  );
  assert.ok(
    notifications?.children?.some(
      (item) => item.path === "/admin/settings/notification",
    ),
  );

  // The lite build removes remote management and its execution, terminal, and terminal settings entries.
  assert.equal(
    menuConfig.menu.some((item) => item.path === "/admin/remote-management"),
    false,
  );
  for (const removed of ["/admin/exec", "/admin/terminal", "/admin/settings/xtermjs"]) {
    assert.equal(paths.includes(removed), false, `${removed} should not appear in the sidebar`);
  }

  assert.deepEqual(
    menuConfig.footer.map((item) => item.path),
    ["/admin/logs"],
  );
  assert.equal(allPaths(menuConfig.footer).includes("/admin/about"), false);
});

test("uses concise sidebar labels without shortening page titles", () => {
  const expected = new Map([
    ["/admin/notifications", "Notifications"],
    ["/admin/settings/notification", "Channels"],
    ["/admin/notification/ping-loss", "Latency Alerts"],
    ["/admin/appearance", "Appearance"],
    ["/admin/theme_managed", "Theme"],
    ["/admin/settings", "Settings"],
    ["/admin/settings/dashboard", "Dashboard"],
    ["/admin/settings/metrics", "Storage"],
    ["/admin/settings/account-security", "Account"],
  ]);

  const items = new Map<string, MenuItem>();
  const collect = (menu: MenuItem[]) => {
    for (const item of menu) {
      items.set(item.path, item);
      collect(item.children ?? []);
    }
  };
  collect([...menuConfig.menu, ...menuConfig.footer]);

  for (const locale of ["en", "zh_CN"]) {
    const messages = JSON.parse(
      readFileSync(new URL(`../src/i18n/locales/${locale}.json`, import.meta.url), "utf8"),
    ) as Record<string, unknown>;
    const translate = (key: string) =>
      key.split(".").reduce<unknown>(
        (value, part) => (value as Record<string, unknown>)?.[part],
        messages,
      );

    for (const [path, item] of items) {
      const label = item.rawLabel ?? translate(item.labelKey);
      assert.equal(typeof label, "string", `${locale}: ${path} is missing its label`);
      if (typeof label !== "string") continue;
      assert.ok(label.length <= 14, `${locale}: ${path} wraps with ${label}`);
      if (expected.has(path)) assert.equal(label, expected.get(path));
    }
    assert.equal(translate("notification.ping_loss.title"), "Latency Monitoring");
    assert.equal(translate("settings.dashboard.title"), "Dashboard configuration");
    assert.equal(translate("theme.title"), "Theme Management");
    assert.equal(translate("navigation.account_security"), "Account & Security");
  }
});

test("gives sidebar labels room while keeping the mobile page visible", () => {
  assert.match(adminPanelSource, /const DESKTOP_SIDEBAR_WIDTH = 252;/);
  assert.match(
    adminPanelSource,
    /const MOBILE_SIDEBAR_WIDTH = "min\(280px, calc\(100vw - 56px\)\)";/,
  );
});

test("places dynamic theme configuration inside the appearance group", () => {
  const dynamicTheme: MenuItem = {
    labelKey: "Current theme settings",
    rawLabel: "Current theme settings",
    path: "/admin/theme-settings",
    icon: "Palette",
  };
  const result = buildAdminMenuItems(menuConfig.menu, [dynamicTheme]);
  const appearance = result.find((item) => item.path === APPEARANCE_MENU_PATH);

  assert.equal(result.some((item) => item.path === dynamicTheme.path), false);
  assert.equal(
    appearance?.children?.at(-1)?.path,
    dynamicTheme.path,
  );
});

test("keeps the fixed Glass settings as the only theme sidebar entry", () => {
  assert.match(adminPanelSource, /const menuItems = baseMenuItems/);
  assert.doesNotMatch(adminPanelSource, /theme_raw/);
  assert.doesNotMatch(adminPanelSource, /\/themes\/\$\{encodeURIComponent\(currentTheme\)\}/);
  assert.doesNotMatch(adminPanelSource, /buildAdminMenuItems\(baseMenuItems/);
});

test("keeps only one sidebar group expanded", () => {
  assert.deepEqual(
    toggleSingleSubMenu({ "/admin/monitoring": true }, "/admin/settings"),
    { "/admin/settings": true },
  );
  assert.deepEqual(
    toggleSingleSubMenu({ "/admin/settings": true }, "/admin/settings"),
    {},
  );
});

test("keeps the monitoring group open across rapid standalone navigation", () => {
  let openSubMenus = { "/admin/monitoring": true };

  for (const pathname of [
    "/admin/servers",
    "/admin/ping",
  ]) {
    const next = syncSubMenuForLocation(
      openSubMenus,
      menuConfig.menu,
      pathname,
    );
    assert.equal(next, openSubMenus);
    openSubMenus = next;
  }
});

test("navigation highlights the exact route or a nested route, not a sibling prefix", () => {
  assert.equal(isAdminMenuPathActive("/admin/ping", "/admin/ping"), true);
  assert.equal(isAdminMenuPathActive("/admin/ping/history", "/admin/ping"), true);
  assert.equal(isAdminMenuPathActive("/admin/ping-history", "/admin/ping"), false);
  assert.equal(isAdminMenuPathActive("/admin/settings", "/admin"), false);
  assert.equal(isAdminMenuPathActive("/admin", "/admin"), true);
});

test("sidebar groups and links share the same focusable navigation surface", () => {
  assert.match(adminPanelSource, /<button\s+type="button"\s+className="km-admin-nav-item km-admin-nav-row/);
  assert.match(adminPanelSource, /aria-expanded=\{Boolean\(isOpen\)\}/);
  assert.match(adminPanelSource, /data-active=\{groupActive \? "true" : undefined\}/);
  assert.match(adminPanelSource, /className="km-admin-nav-item group"/);
  assert.match(globalCssSource, /\.km-admin-nav-item:is\(:hover, :focus-visible\)/);
  assert.match(globalCssSource, /\.km-admin-nav-row\s*\{[^}]*border-left: 2px solid transparent/);
  assert.match(globalCssSource, /\.km-admin-nav-item\[data-active="true"\] \.km-admin-nav-row,[^}]*border-color: var\(--km-nav-accent\)/);
  assert.match(globalCssSource, /--km-nav-accent: var\(--accent-11\)/);
  assert.doesNotMatch(adminPanelSource, /hover:bg-accent-3/);
  assert.doesNotMatch(adminPanelSource, /var\(--gray11\)/);
});

test("admin floating menus use one quiet surface and legible selection states", () => {
  assert.match(globalCssSource, /:is\(\.rt-SelectContent, \.rt-DropdownMenuContent, \.rt-PopoverContent, \.admin-select-or-input-content\)\s*\{[^}]*background: var\(--km-panel\)/);
  assert.match(globalCssSource, /\.admin-select-or-input-content \[role="option"\]\[aria-selected="true"\]\s*\{[^}]*background: var\(--accent-a4\)/);
  assert.doesNotMatch(selectOrInputSource, /bg-accent-9 text-\[var\(--accent-contrast\)\]/);
});

test("does not match a sibling route that only shares a child prefix", () => {
  const monitoringOpen = { "/admin/monitoring": true };

  assert.equal(
    syncSubMenuForLocation(
      monitoringOpen,
      menuConfig.menu,
      "/admin/ping-history",
    ),
    monitoringOpen,
  );
});

test("switches to the submenu containing the current nested route", () => {
  assert.deepEqual(
    syncSubMenuForLocation(
      { "/admin/monitoring": true },
      menuConfig.menu,
      "/admin/settings/site",
    ),
    { "/admin/settings": true },
  );
});

test("does not create an implicit second grid column on mobile", () => {
  assert.match(adminPanelSource, /className="km-admin-topbar md:col-span-2"/);
  assert.doesNotMatch(adminPanelSource, /className="col-span-2"/);
  assert.match(adminPanelSource, /open: \{\s+x: 0,\s+opacity: 1,/);
  assert.match(adminPanelSource, /closed: \{\s+x: 0,\s+opacity: 1,/);
  assert.match(adminPanelSource, /sidebarOpen\s+\? `\$\{DESKTOP_SIDEBAR_WIDTH\}px`\s+: "0px"/);
});

test("mobile and desktop submenus share the original motion collapse", () => {
  assert.doesNotMatch(adminPanelSource, /km-admin-mobile-submenu/);
  assert.doesNotMatch(globalCssSource, /\.km-admin-mobile-submenu/);
  assert.match(
    adminPanelSource,
    /<motion\.div\s+inert=\{!isOpen\}\s+aria-hidden=\{!isOpen\}\s+initial=\{\{ height: 0, opacity: 0 \}\}[\s\S]*height: "auto", opacity: 1/,
  );
  assert.match(
    adminPanelSource,
    /transition=\{reduceMotion \? \{ duration: 0 \} : \{ duration: 0\.14 \}\}/,
  );
  assert.match(adminPanelSource, /<motion\.div\s+inert=\{!isOpen\}\s+aria-hidden=\{!isOpen\}/);
});

test("system UI routes do not embed the legacy public dashboard", () => {
  assert.doesNotMatch(routesSource, /pages\/Index|pages\/_layout|pages\/instance/);
  assert.doesNotMatch(routesSource, /path:\s*["']\/["']/);
  assert.match(routesSource, /path:\s*["']\/admin["']/);
  assert.match(routesSource, /path:\s*["']\/install["']/);
  // The lite build also removes the remote terminal route.
  assert.doesNotMatch(routesSource, /path:\s*["']\/terminal["']/);
  assert.doesNotMatch(routesSource, /pages\/admin\/exec|pages\/admin\/terminal|pages\/admin\/settings\/xtermjs/);
  assert.match(routesSource, /path:\s*["']\/manage\/\*["']/);
});

test("admin route changes keep the main content out of a composited animation layer", () => {
  assert.doesNotMatch(adminPanelSource, /<AnimatePresence mode="wait"/);
  assert.match(
    adminPanelSource,
    /<div data-admin-page-content style=\{\{ minHeight: "100%" \}\}>/,
  );
  assert.doesNotMatch(adminPanelSource, /<motion\.div[^>]*data-admin-page-content/);
  assert.doesNotMatch(adminPanelSource, /ADMIN_PAGE_TRANSITION/);
  assert.doesNotMatch(adminPanelSource, /key=\{location\.pathname\}/);
  assert.doesNotMatch(adminPanelSource, /willChange:[^\n]*"opacity, transform"/);
  assert.doesNotMatch(adminPanelSource, /useReducedMotion/);
  assert.match(adminPanelSource, /Boolean\(settings\.reduce_motion\)/);
  assert.match(adminPanelSource, /preloadAdminRoute\(to\)/);
  assert.match(adminPanelSource, /onPointerOverCapture=\{\(event\) => preloadAdminLink/);
  assert.doesNotMatch(adminPanelSource, /onClickCapture=/);
  assert.match(adminPanelSource, /url\.pathname !== "\/admin"/);
  assert.match(adminPanelSource, /anchor\.dataset\.adminReloadDocument/);
});

test("admin layout never prompts for legal notice acceptance", () => {
  assert.doesNotMatch(adminLayoutSource, /Legal Notice and Compliance Guidelines/);
  assert.doesNotMatch(adminLayoutSource, /eula_accepted|acceptEula|\bEula\b/);
  assert.doesNotMatch(adminLayoutSource, /<Dialog\.Root open=\{open\}>/);
  assert.match(adminLayoutSource, /<AdminPanelBar/);
});

test("admin tabs and dialogs share the saved motion preference", () => {
  assert.match(adminPanelSource, /dataset\.adminShellActive = "true"/);
  assert.match(adminPanelSource, /data-admin-tab-motion-ready/);
  assert.doesNotMatch(globalCssSource, /admin-tab-content-enter/);
  assert.doesNotMatch(
    globalCssSource,
    /\.rt-TabsContent\[data-state="active"\][\s\S]*?(?:animation|backface-visibility|will-change)/,
  );
  assert.match(globalCssSource, /admin-dialog-content-enter 240ms[^;]*backwards/);
  assert.match(globalCssSource, /height: 2px;[\s\S]*background: var\(--accent-9\)/);
  assert.match(globalCssSource, /\.rt-TabsTriggerInner[\s\S]*background-color: transparent !important/);
  assert.match(globalCssSource, /\.rt-TabsTrigger:hover \.rt-TabsTriggerInner[\s\S]*background-color: var\(--gray-a3\) !important/);
  assert.doesNotMatch(globalCssSource, /\.rt-TabsTrigger\[data-state="active"\] \.rt-TabsTriggerInner[\s\S]*background-color: var\(--accent-a3\) !important/);
  assert.match(globalCssSource, /\.rt-TabsTrigger:focus-visible[\s\S]*outline-offset: 1px/);
  assert.match(globalCssSource, /admin-dialog-content-exit 220ms[^;]*forwards/);
  assert.doesNotMatch(
    globalCssSource,
    /\.rt-BaseDialogContent\[data-state="(?:open|closed)"\][^}]*will-change/,
  );
  assert.match(globalCssSource, /data-reduce-motion="true"[\s\S]*\.rt-BaseDialogContent/);
});

test("admin checkboxes share the active accent palette", () => {
  assert.match(globalCssSource, /\.rt-CheckboxRoot::before[\s\S]*background-color 150ms ease/);
  assert.doesNotMatch(globalCssSource, /\.rt-CheckboxRoot\[data-state="checked"\]::before[\s\S]*background-color: var\(--accent-8\)/);
  assert.match(checkboxSource, /data-\[state=checked\]:border-\[var\(--accent-9\)\]/);
  assert.match(checkboxSource, /data-\[state=checked\]:bg-\[var\(--accent-9\)\]/);
  assert.match(checkboxSource, /data-\[state=checked\]:text-\[var\(--accent-contrast\)\]/);
  assert.match(checkboxSource, /data-\[state=indeterminate\]:bg-\[var\(--accent-9\)\]/);
  assert.match(checkboxSource, /data-\[state=indeterminate\]:text-\[var\(--accent-contrast\)\]/);
  assert.match(checkboxSource, /border shadow-xs transition-shadow/);
  assert.match(globalCssSource, /data-reduce-motion="true"[\s\S]*\.rt-CheckboxRoot::before/);
  assert.match(selectorSource, /import \{ Checkbox \} from "\.\/ui\/checkbox"/);
  assert.doesNotMatch(selectorSource, /import \{ Checkbox, TextField \} from "@radix-ui\/themes"/);
  assert.match(selectorSource, /checked=\{checkAllState\}/);
  assert.match(checkboxSource, /MinusIcon[\s\S]*group-data-\[state=indeterminate\]:block/);
});

test("node selector dialogs keep a single select-all control", () => {
  assert.match(selectorSource, /showHeaderSelectAll = true/);
  assert.match(selectorSource, /showHeaderSelectAll \? \([\s\S]*aria-label=\{t\("common\.select_all"\)\}/);
  assert.match(nodeSelectorSource, /showHeaderSelectAll=\{false\}/);
});

test("admin floating controls and switches animate consistently", () => {
  assert.match(globalCssSource, /@keyframes admin-floating-content-enter/);
  assert.match(globalCssSource, /:is\(\.rt-SelectContent, \.rt-DropdownMenuContent, \.rt-PopoverContent, \.admin-select-or-input-content\)\[data-state="open"\]/);
  assert.match(globalCssSource, /admin-floating-content-enter 180ms[^;]*backwards/);
  assert.match(globalCssSource, /admin-floating-content-exit 140ms[^;]*forwards/);
  assert.doesNotMatch(
    globalCssSource,
    /:is\(\.rt-SelectContent,[^}]*\[data-state="(?:open|closed)"\][^}]*will-change/,
  );
  assert.doesNotMatch(globalCssSource, /\.rt-SelectItem\[data-highlighted\][\s\S]*background-color: var\(--accent-a3\)/);
  assert.doesNotMatch(globalCssSource, /\.rt-SelectItem\[data-state="checked"\][\s\S]*background-color: var\(--accent-a4\)/);
  assert.match(selectOrInputSource, /admin-select-or-input-content/);
  assert.match(selectOrInputSource, /data-state=\{open \? "open" : "closed"\}/);
  assert.match(selectOrInputSource, /FLOATING_CONTENT_EXIT_MS = 140/);
  assert.match(selectOrInputSource, /bg-\[var\(--accent-a4\)\] text-foreground/);
  assert.match(selectOrInputSource, /hover:bg-\[var\(--accent-a3\)\] hover:text-foreground/);
  assert.match(selectOrInputSource, /text-sm font-normal outline-hidden/);
  assert.doesNotMatch(selectOrInputSource, /text-sm font-semibold outline-hidden/);
  assert.doesNotMatch(selectOrInputSource, /rounded-md border bg-accent-1[\s\S]*shadow-md/);
  assert.match(globalCssSource, /\.rt-SwitchThumb[\s\S]*transform 180ms/);
  assert.match(globalCssSource, /\.rt-SwitchThumb\[data-state="checked"\][\s\S]*scale\(0\.92\)/);
  assert.match(globalCssSource, /data-reduce-motion="true"[\s\S]*\.rt-SelectContent/);
  assert.match(globalCssSource, /data-reduce-motion="true"[\s\S]*\.rt-SwitchThumb/);
});

test("admin command buttons use motion instead of abrupt active flashes", () => {
  assert.match(globalCssSource, /\.rt-Button,[\s\S]*\.rt-IconButton[\s\S]*background-color 160ms/);
  assert.match(globalCssSource, /\.rt-Button:active:not\(\[data-disabled\],[\s\S]*filter: none;[\s\S]*scale\(0\.985\)/);
  assert.match(globalCssSource, /data-reduce-motion="true"[\s\S]*\.rt-Button/);
  assert.match(globalCssSource, /data-reduce-motion="true"[\s\S]*\.rt-IconButton/);
  assert.match(
    globalCssSource,
    /@media \(pointer: coarse\)[\s\S]*\.rt-Button:active[\s\S]*outline: none/,
  );
});

test("prewarms admin routes and reuses shared monitoring data", () => {
  assert.match(routesSource, /export const preloadAdminRoutes/);
  assert.match(mainSource, /scheduleIdleAdminWarmup/);
  assert.match(mainSource, /getIdleAdminWarmupTargets/);
  assert.match(
    adminLayoutSource,
    /<NodeDetailsProvider>\s*<PingTaskProvider>\s*<AdminAuthenticatedContent \/>/,
  );
  assert.match(pingTaskContextSource, /React\.useState<boolean>\(true\)/);
  assert.match(pingTaskContextSource, /const inherited = React\.useContext\(PingTaskContext\)/);
  assert.doesNotMatch(pingTaskContextSource, /refresh\(\);\s*setIsLoading\(false\)/);
  assert.doesNotMatch(pingTaskPageSource, /<PingTaskProvider>/);
  assert.doesNotMatch(pingTaskPageSource, /<NodeDetailsProvider>/);
});

test("dashboard alert navigation prepares filtered server data before switching", () => {
  assert.match(dashboardPanelsSource, /prefetchDashboardAlertItems\(kind, accountKey\)/);
  assert.match(dashboardPanelsSource, /event\.preventDefault\(\)/);
  assert.match(serverPageSource, /getDashboardAlertItemsSnapshot\(routeAlert, accountKey\)/);
  assert.match(serverPageSource, /if \(isLoading\) return <Loading text="" \/>/);
  assert.doesNotMatch(serverPageSource, /isLoading \|\| alertFilterLoading/);
});

test("registers the dashboard settings route", () => {
  assert.match(routesSource, /path:\s*["']dashboard["']/);
  assert.match(routesSource, /pages\/admin\/settings\/dashboard/);
});

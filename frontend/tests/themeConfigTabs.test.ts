import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  groupThemeConfigFields,
  resolveActiveThemeConfigGroup,
} from "../src/utils/themeConfigTabs.ts";

const tabsSource = readFileSync(
  new URL("../src/components/admin/ThemeConfigTabs.tsx", import.meta.url),
  "utf8",
);
const panelSource = readFileSync(
  new URL("../src/components/admin/AdminPanelBar.tsx", import.meta.url),
  "utf8",
);
const globalStyles = readFileSync(
  new URL("../src/global.css", import.meta.url),
  "utf8",
);

test("groups theme fields without exposing empty category tabs", () => {
  assert.deepEqual(
    groupThemeConfigFields([
      { type: "string", key: "before" },
      { type: "title", name: "Empty" },
      { type: "title", name: "Display" },
      { type: "switch", key: "showLogo" },
      { type: "title", name: "Network" },
      { type: "select", key: "mode" },
      { type: "string" },
    ]),
    [
      { items: [{ type: "string", key: "before" }] },
      {
        title: "Display",
        items: [{ type: "switch", key: "showLogo" }],
      },
      {
        title: "Network",
        items: [{ type: "select", key: "mode" }],
      },
    ],
  );
});

test("resolves the active category from the shared admin scroll position", () => {
  assert.equal(resolveActiveThemeConfigGroup([100, 400, 800], 399), 0);
  assert.equal(resolveActiveThemeConfigGroup([100, 400, 800], 401), 1);
  assert.equal(resolveActiveThemeConfigGroup([100, 400, 800], 401, true), 2);
});

test("theme tabs switch only the content below the stable page heading", () => {
  assert.match(panelSource, /data-admin-scroll-container/);
  assert.match(tabsSource, /const activeGroup = groups\[currentTab\]/);
  assert.match(tabsSource, /activeGroup\.items\.map\(renderField\)/);
  assert.doesNotMatch(tabsSource, /container\.scrollTo/);
  assert.doesNotMatch(tabsSource, /scrollIntoView/);
  assert.match(globalStyles, /overscroll-behavior-x:\s*contain/);
  assert.match(globalStyles, /min-height:\s*2\.75rem/);
  assert.doesNotMatch(globalStyles, /width:\s*max-content/);
  assert.match(globalStyles, /\.km-page-admin-theme-managed,[\s\S]*min-width:\s*0/);
  assert.match(tabsSource, /list\.scrollWidth - list\.clientWidth/);
  assert.match(tabsSource, /new ResizeObserver\(updateScrollEdges\)/);
  assert.match(tabsSource, /km-theme-config-scroll-button/);
  assert.match(tabsSource, /reduceMotion \? false/);
  assert.doesNotMatch(globalStyles, /admin-tab-content-enter/);
  assert.doesNotMatch(
    globalStyles,
    /\.rt-TabsContent\[data-state="active"\][^}]*?(?:animation|backface-visibility|will-change)/,
  );
  assert.match(globalStyles, /data-reduce-motion="true"/);
  assert.match(globalStyles, /\.rt-SegmentedControlItem/);
  assert.match(globalStyles, /a\[href\^="\/admin"\]/);
  assert.doesNotMatch(tabsSource, /overflow-y-auto/);
  assert.doesNotMatch(tabsSource, /plugin/i);
  assert.match(tabsSource, /case "nodes"/);
  assert.match(tabsSource, /NodeSelectorDialog/);
  assert.match(tabsSource, /case "pingtasks"/);
  assert.match(tabsSource, /PingTaskSelectorDialog/);
});

test("the fixed Glass configuration owns localized manifest text", () => {
  const themeSettingsSource = readFileSync(
    new URL("../src/pages/admin/theme_managed.tsx", import.meta.url),
    "utf8",
  );
  assert.match(themeSettingsSource, /resolveI18nText/);
  assert.match(themeSettingsSource, /themeDisplayName/);
  assert.doesNotMatch(themeSettingsSource, /importPreview/);
});

test("mobile admin drawer stays above sticky theme category tabs", () => {
  assert.match(globalStyles, /\.km-theme-config-tabs\s*\{[\s\S]*?z-index:\s*10/);
  assert.match(panelSource, /z-\[49\]/);
  assert.match(panelSource, /zIndex:\s*isMobile\s*\?\s*50\s*:\s*1/);
});


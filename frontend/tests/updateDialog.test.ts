import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { AVAILABLE_LOCALES } from "./localeFixtures.ts";

const source = readFileSync(
  "src/components/admin/AdminPanelBar.tsx",
  "utf8",
);
const brandSource = readFileSync(
  "src/components/KomariLiteBrand.tsx",
  "utf8",
);
const globalStyles = readFileSync("src/global.css", "utf8");
const appSource = readFileSync("src/main.tsx", "utf8");

test("admin branding keeps Lite smaller and green on desktop and mobile", () => {
  assert.match(source, /<KomariLiteBrand size=\{isMobile \? "sm" : "md"\} \/>/);
  assert.match(brandSource, /text-\[var\(--green-9\)\]/);
  assert.doesNotMatch(brandSource, /bg-\[var\(--green-a3\)\]/);
  assert.match(brandSource, /lite: "text-\[13px\]"/);
  assert.match(brandSource, /lite: "text-base"/);
});

test("mobile navigation uses a partial overlay without hiding the page", () => {
  assert.match(source, /const MOBILE_SIDEBAR_WIDTH = "clamp\(184px, 42vw, 244px\)"/);
  assert.match(source, /open:\s*\{\s*x: 0,/);
  assert.match(source, /closed:\s*\{\s*x: "-100%",/);
  assert.match(
    source,
    /width: isMobile\s*\? MOBILE_SIDEBAR_WIDTH\s*:\s*sidebarOpen\s*\? `\$\{DESKTOP_SIDEBAR_WIDTH\}px`\s*:\s*"0px"/,
  );
  assert.match(source, /willChange: isMobile \? "transform" : undefined/);
  assert.doesNotMatch(source, /open:\s*\{\s*width: isMobile/);
  assert.match(source, /data-testid="mobile-sidebar-trigger"/);
  assert.match(source, /data-testid="mobile-sidebar-close"/);
  assert.match(source, /key="mobile-sidebar-backdrop"/);
  assert.match(source, /onClick=\{\(\) => setSidebarOpen\(false\)\}/);
  assert.match(source, /backgroundColor: "var\(--accent-3\)"[\s\S]{0,100}display: "block"/);
  assert.doesNotMatch(
    source,
    /backgroundColor: "var\(--accent-3\)"[\s\S]{0,100}display: isMobile && sidebarOpen \? "none"/,
  );
});

test("admin shell owns the viewport while the mobile drawer locks only main content", () => {
  assert.match(source, /initial: "auto minmax\(0, 1fr\)"/);
  assert.match(source, /md: "auto minmax\(0, 1fr\)"/);
  assert.match(source, /height: "var\(--app-viewport-height, 100vh\)"/);
  assert.match(source, /width: "100%",\s+overflow: "hidden",\s+overscrollBehavior: "none"/);
  assert.match(source, /key="mobile-sidebar-backdrop"[\s\S]*className="[^"]*touch-none/);
  assert.match(source, /data-admin-scroll-container[\s\S]*overflowY: isMobile && sidebarOpen \? "hidden" : "auto"/);
  assert.match(source, /overflowY: "auto",\s+overflowX: "hidden",\s+overscrollBehaviorY: "contain"/);
  assert.match(globalStyles, /--app-viewport-height: 100vh/);
  assert.match(globalStyles, /@supports \(height: 100dvh\)[\s\S]*--app-viewport-height: 100dvh/);
  assert.match(appSource, /minHeight: "var\(--app-viewport-height, 100vh\)"/);
});

test("mobile navigation label is localized in every admin language", () => {
  for (const locale of AVAILABLE_LOCALES) {
    const messages = JSON.parse(
      readFileSync(`src/i18n/locales/${locale}.json`, "utf8"),
    );
    assert.equal(typeof messages.navigation.open, "string");
    assert.notEqual(messages.navigation.open.trim(), "");
  }
});


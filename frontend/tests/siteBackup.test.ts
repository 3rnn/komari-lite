import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { AVAILABLE_LOCALES, readLocale } from "./localeFixtures.ts";
import test from "node:test";
import {
  ADMIN_LIST_PAGE_SIZE,
  adminPageSizeOptions,
  isValidAdminPageSize,
  normalizeAdminPageSize,
} from "../src/utils/adminPagination.ts";

const source = readFileSync("src/pages/admin/settings/site.tsx", "utf8");
const generalSource = readFileSync("src/pages/admin/settings/general.tsx", "utf8");
const zhCN = JSON.parse(readFileSync("src/i18n/locales/zh_CN.json", "utf8"));

test("normalizes the global admin page size within its supported range", () => {
  assert.equal(ADMIN_LIST_PAGE_SIZE, 10);
  assert.equal(normalizeAdminPageSize(30), 30);
  assert.equal(normalizeAdminPageSize("40"), 40);
  assert.equal(normalizeAdminPageSize(4), 10);
  assert.equal(normalizeAdminPageSize(101), 10);
  assert.equal(normalizeAdminPageSize(10.5), 10);
  assert.equal(normalizeAdminPageSize("invalid"), 10);
  assert.equal(isValidAdminPageSize(5), true);
  assert.equal(isValidAdminPageSize(100), true);
  assert.deepEqual(adminPageSizeOptions(), [10, 50, 100]);
});

test("global list pagination is configured under general settings", () => {
  assert.match(generalSource, /settings\.general\.admin_default_page_size/);
  assert.match(generalSource, /admin_default_page_size: pageSize/);
  assert.match(generalSource, /<SettingCardLabel>[\s\S]*settings\.general\.admin_default_page_size[\s\S]*<SettingCardShortTextInput/);
  assert.doesNotMatch(generalSource, /<SettingCardShortTextInput\s+title=\{t\("settings\.general\.admin_default_page_size"\)\}/);
  assert.doesNotMatch(source, /settings\.site\.admin_default_page_size/);
  assert.equal(zhCN.settings.general.admin_default_page_size, "列表默认分页");
});

test("general settings exposes the Komari motion preference", () => {
  assert.match(generalSource, /settings\.general\.reduce_motion/);
  assert.match(generalSource, /reduce_motion: checked/);
  assert.equal(zhCN.settings.general.reduce_motion, "减少动态效果");
});

test("auto discovery help opens the dedicated agent guide", () => {
  assert.match(
    generalSource,
    /https:\/\/nuomiiiii\.github\.io\/komari-document\/install\/agent-ad["']/,
  );
  assert.doesNotMatch(generalSource, /agent-ad\.html/);
});

test("global pagination is wired into shared and server-backed admin lists", () => {
  const shared = readFileSync("src/components/admin/AdminPagination.tsx", "utf8");
  const globalHook = readFileSync("src/hooks/useAdminDefaultPageSize.ts", "utf8");
  const servers = readFileSync("src/pages/admin/index.tsx", "utf8");
  const logs = readFileSync("src/pages/admin/log.tsx", "utf8");

  assert.match(globalHook, /settings\.admin_default_page_size/);
  assert.match(shared, /useAdminDefaultPageSize\(\)/);
  assert.match(servers, /useAdminDefaultPageSize\(\)/);
  assert.match(logs, /useState<number>\(defaultPageSize\)/);
  assert.match(logs, /showSummary=\{false\}/);
});

test("full and configuration exports download through authenticated requests", () => {
  assert.match(source, /downloadBackup\("full"\)/);
  assert.match(source, /downloadBackup\("config"\)/);
  assert.match(source, /response\.blob\(\)/);
  assert.match(source, /anchor\.download = filename/);
  assert.doesNotMatch(source, /window\.open\("\/api\/admin\/download\/backup/);
});

test("both backup rows use the same text action as restore", () => {
  assert.equal(source.match(/\{t\("common\.export"\)\}/g)?.length, 2);
  assert.match(source, /\{t\("common\.select"\)\}/);
  assert.doesNotMatch(source, /SettingCardIconButton/);
});

test("account backup hint points to the actual site settings page", () => {
  for (const locale of AVAILABLE_LOCALES) {
    const hint = readLocale(locale).account_settings.looking_for_backup;
    assert.equal(typeof hint, "string");
    assert.ok(hint.length > 0);
  }
  assert.match(String(zhCN.account_settings.looking_for_backup), /设置/);
});

test("backup restore dialog uses staged progress instead of a fake 95 percent finish", () => {
  assert.match(source, /phase_uploading/);
  assert.match(source, /phase_processing/);
  assert.match(source, /phase_completed/);
  assert.match(source, /uploadState={restoreState}/);
  assert.match(
    source,
    /setRestoreOpen\(false\);\s*await delay\(UPLOAD_DIALOG_EXIT_MS\);\s*setTrackedRestoreState\(null\)/,
  );
  assert.doesNotMatch(source, /setRestoreProgress/);
});

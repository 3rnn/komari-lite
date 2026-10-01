import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import * as themeTabs from "../src/utils/themeConfigTabs.ts";

test("missing task falls back only after a successful task list loads", () => {
  const normalize = themeTabs.normalizePreferredPingTaskId;
  assert.equal(normalize(7, null), 7);
  assert.equal(normalize(7, [{ id: 7 }]), 7);
  assert.equal(normalize(7, [{ id: 8 }]), 0);
  assert.equal(normalize(7, []), 0);
  assert.equal(normalize(0, []), 0);
});

test("deleted ping-task selections are normalized before display and save", () => {
  const tabs = readFileSync(new URL("../src/components/admin/ThemeConfigTabs.tsx", import.meta.url), "utf8");
  const page = readFileSync(new URL("../src/pages/admin/theme_managed.tsx", import.meta.url), "utf8");
  assert.match(tabs, /normalizePreferredPingTaskId\(value, availableTasks\)/);
  assert.match(page, /normalizePreferredPingTaskId\(current, availableTasks\)/);
  assert.match(page, /const requiresPingTasks = fields\.some\(\(field\) => field\.type === "pingtask"\)/);
  assert.match(page, /if \(!theme \|\| \(requiresPingTasks && \(pingTasksLoading \|\| pingTaskError\)\)\) return/);
  assert.match(page, /disabled=\{saving \|\| \(requiresPingTasks && \(pingTasksLoading \|\| Boolean\(pingTaskError\)\)\)\}/);
  assert.doesNotMatch(page, /console\.log\("(Values|Payload) before saving:/);
});

const manifest = JSON.parse(readFileSync(new URL("../../backend/web/public/bundledThemes/Glass/komari-theme.json", import.meta.url), "utf8"));
const fields = manifest.configuration.data;

test("admin exposes a toggle between a single selected task and separate assigned tasks", () => {
  const field = fields.find((entry) => entry.key === "showCarrierPing");
  assert.equal(field.type, "switch");
  assert.match(field.help.en, /each assigned task/i);
  assert.match(field.help.en, /one selected task/i);
  assert.doesNotMatch(field.help.en, /aggregate/i);
});

test("admin can select one ping task by ID or automatic per-node selection", () => {
  const field = fields.find((entry) => entry.key === "preferredPingTaskId");
  assert.equal(field.type, "pingtask");
  assert.equal(field.default, 0);
  assert.match(readFileSync(new URL("../src/components/admin/ThemeConfigTabs.tsx", import.meta.url), "utf8"), /case "pingtask"/);
});

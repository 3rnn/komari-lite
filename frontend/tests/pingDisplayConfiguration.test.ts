import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { assignedDisplayTasks, resolveDisplayPingTaskId } from "../src/utils/pingDisplaySelection.ts";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const serverView = fs.readFileSync(path.join(root, "src/pages/admin/pingTask_Server.tsx"), "utf8");
const latencyPage = fs.readFileSync(path.join(root, "src/pages/admin/pingTask.tsx"), "utf8");
const locales = ["en.json", "zh_CN.json"].map((name) => JSON.parse(fs.readFileSync(path.join(root, "src/i18n/locales", name), "utf8")));

const tasks = [
  { id: 1, name: "CT", clients: ["node-a", "node-b"] },
  { id: 2, name: "CU", clients: ["node-a"] },
  { id: 3, name: "CM", clients: ["node-b"] },
];

test("display selection offers only tasks actually assigned to this server", () => {
  assert.deepEqual(assignedDisplayTasks("node-a", tasks).map((task) => task.id), [1, 2]);
  assert.deepEqual(assignedDisplayTasks("node-b", tasks).map((task) => task.id), [1, 3]);
  assert.deepEqual(assignedDisplayTasks("node-c", tasks), []);
});

test("deleted or unassigned display choice resets to automatic without changing probe assignments", () => {
  assert.equal(resolveDisplayPingTaskId("node-a", tasks, 2), 2);
  assert.equal(resolveDisplayPingTaskId("node-b", tasks, 2), 0);
  assert.equal(resolveDisplayPingTaskId("node-a", tasks, 999), 0);
  assert.equal(resolveDisplayPingTaskId("node-a", tasks, 0), 0);
  assert.deepEqual(tasks[0].clients, ["node-a", "node-b"]);
});

test("probe tab owns assignments; public-display tab saves only the selected task", () => {
  assert.match(latencyPage, /<TaskView[\s\S]*pingTasks=\{filteredTasks\}/);
  assert.match(latencyPage, /t\("ping\.monitor_config_tab"\)/);
  assert.match(latencyPage, /t\("ping\.public_display_tab"\)/);
  assert.match(latencyPage, /t\("ping\.monitor_config_hint"\)/);
  assert.match(latencyPage, /t\("ping\.public_display_hint"\)/);
  assert.match(serverView, /\/api\/admin\/client\/\$\{nodeUuid\}\/display-ping-task/);
  assert.match(serverView, /body: JSON\.stringify\(\{ task_id: selectedTaskId \}\)/);
  assert.doesNotMatch(serverView, /\/api\/admin\/ping\/edit|task\.clients\s*=/);
  for (const locale of locales) {
    assert.equal(locale.ping.monitor_config_tab, "Probe Setup");
    assert.equal(locale.ping.public_display_tab, "Public Display");
    assert.equal(locale.ping.display_task, "Displayed probe");
    assert.match(locale.ping.monitor_config_hint, /server runs/);
  }
});

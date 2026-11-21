import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { extname, join } from "node:path";
import test from "node:test";

const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const node = { uuid: "greencloud-id", name: "greencloud", weight: 0, mem_total: 8192,
  disk_total: 8192, region: "", os: "Linux", tags: "", price: 0, display_ping_task_ids: [3],
  traffic_limit: 128 * 1024 ** 2, traffic_limit_type: "sum" };
const mime = { ".html": "text/html", ".css": "text/css", ".js": "text/javascript", ".png": "image/png" };
const mimeFor = file => mime[extname(file)] ?? "application/octet-stream";
const latencyTasks = [{ id: 2, name: "CT" }, { id: 3, name: "CU" }, { id: 4, name: "CM" }];

test("the shipped homepage renders two views, filters the same nodes, and shares the configured latency probe", {
  skip: !process.env.GLASS_PLAYWRIGHT_MODULE,
}, async t => {
  const { chromium } = await import(process.env.GLASS_PLAYWRIGHT_MODULE);
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  t.after(() => browser.close());
  const page = await browser.newPage({ viewport: { width: 1440, height: 980 } });
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));

  let socket;
  await page.routeWebSocket("**/api/clients", connection => {
    socket = connection;
    connection.onMessage(message => {
      if (message !== "get") return;
      connection.send(JSON.stringify({ status: "success", data: { online: [node.uuid], data: {
        [node.uuid]: { updated_at: new Date().toISOString(), cpu: { usage: 0.1 }, ram: { used: 1081 },
          disk: { used: 294 }, load: { load1: 0.1, load5: 0.2, load15: 0.3 },
          network: { down: 4096, up: 2048, totalUp: 12 * 1024 ** 2, totalDown: 30 * 1024 ** 2 },
          connections: { tcp: 1, udp: 2 }, process: 7, uptime: 1994400 },
      } } }));
    });
  });
  await page.route("**/*", async route => {
    const url = new URL(route.request().url());
    if (url.hostname !== "glass.test") return route.abort();
    if (url.pathname === "/api/rpc2") {
      const { id, method } = route.request().postDataJSON();
      const minute = Date.now() - 60_000;
      const result = {
        "public:getPublicSettings": { sitename: "Komari Lite", theme_settings: {} },
        "public:getNodesInformation": [node, { ...node, uuid: "offline-id", name: "offline-edge" }],
        "public:listMetricDefinitions": [{ name: "ping.latency_ms", retention_days: 1 }],
        "public:getPingRecords": { tasks: latencyTasks, records: [[2, 25], [3, 300], [3, 1470], [4, 70]]
          .map(([task_id, value], index) => ({ task_id, value, time: new Date(minute + index * 1000).toISOString() })) },
      };
      return route.fulfill({ json: { jsonrpc: "2.0", id, result: result[method] ?? [] } });
    }
    const path = url.pathname === "/" || url.pathname.startsWith("/instance/") ? "index.html" : url.pathname.slice(1);
    try {
      return await route.fulfill({ body: await readFile(new URL(path, dist)), contentType: mimeFor(path) });
    } catch {
      return route.fulfill({ status: 404 });
    }
  });

  await page.goto("http://glass.test/");
  await page.getByRole("heading", { name: "Infrastructure" }).waitFor();
  await page.getByRole("status").getByText("Live", { exact: true }).waitFor();
  await page.getByRole("table", { name: "Nodes" }).waitFor();
  await page.locator(".glass-throughput-plot .glass-trace-upload").first().waitFor({ state: "attached" });
  await page.locator(".glass-throughput-plot .glass-trace-download").first().waitFor({ state: "attached" });
  await page.locator(".glass-site-mark[title=Undertaker]").waitFor();
  const row = page.getByRole("row").filter({ hasText: "greencloud" });
  const traffic = row.locator(".node-cell-traffic");
  await page.getByRole("columnheader", { name: "Traffic", exact: true }).waitFor();
  await traffic.getByText("42.00 MB", { exact: true }).waitFor();
  assert.match(await traffic.innerText(), /32\.8%/);
  assert.match(await traffic.innerText(), /\/ 128\.00 MB/);
  const offlineTraffic = page.getByRole("row").filter({ hasText: "offline-edge" }).locator(".node-cell-traffic");
  await offlineTraffic.getByText("—", { exact: true }).waitFor();
  await row.getByText("885 ms", { exact: true }).waitFor();
  assert.match(await row.innerText(), /CU\s+885 ms/);
  assert.doesNotMatch(await row.innerText(), /\b(?:CT|CM)\b/);
  const screenshotDir = process.env.GLASS_SCREENSHOT_DIR;
  if (screenshotDir) await page.screenshot({ path: join(screenshotDir, "home86-list.png"), fullPage: true });

  const search = page.getByRole("searchbox", { name: "Find a node" });
  await search.fill("missing");
  await page.getByText("No matching nodes", { exact: true }).waitFor();
  await search.fill("green");
  await row.waitFor();
  assert.equal(await page.getByRole("link", { name: "View offline-edge details" }).count(), 0);
  await page.getByRole("button", { name: "Cards", exact: true }).click();
  const card = page.getByRole("link", { name: "View greencloud details" });
  await card.getByText("885 ms", { exact: true }).waitFor();
  assert.match(await card.innerText(), /Latency · CU/);
  await card.getByText("42.00 MB / 128.00 MB", { exact: true }).waitFor();
  assert.equal(await page.getByRole("link", { name: "View offline-edge details" }).count(), 0);
  await search.fill("");
  if (screenshotDir) await page.screenshot({ path: join(screenshotDir, "home86-cards.png"), fullPage: true });
  await page.reload();
  await page.locator('[data-node-view="cards"]').waitFor();

  for (const width of [320, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const view of ["List", "Cards"]) {
      await page.getByRole("button", { name: view, exact: true }).click();
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
        `${width}px ${view} should not overflow`);
      if (view === "List") {
        await traffic.getByText("42.00 MB", { exact: true }).waitFor();
        assert.ok(await traffic.evaluate(cell => cell.scrollWidth <= cell.clientWidth + 1),
          `${width}px traffic should fit inside its column`);
      }
      if (screenshotDir && width === 390) await page.screenshot({ path: join(screenshotDir, `home86-mobile-${view}.png`), fullPage: true });
    }
  }

  await page.getByRole("button", { name: "List", exact: true }).click();
  await page.waitForFunction(() => Number(document.querySelector(".glass-throughput-plot")?.dataset.sampleCount) >= 2);
  const countBefore = Number(await page.locator(".glass-throughput-plot").getAttribute("data-sample-count"));
  await page.getByRole("link", { name: "View greencloud details" }).click();
  await page.getByText("Latency", { exact: true }).click();
  await page.getByRole("button", { name: /CU latency curve/ }).getByText("885 ms", { exact: true }).waitFor();
  await page.getByRole("button", { name: "Komari Lite home" }).click();
  await page.getByRole("heading", { name: "Infrastructure" }).waitFor();
  assert.ok(Number(await page.locator(".glass-throughput-plot").getAttribute("data-sample-count")) >= countBefore);
  socket.close();
  await page.getByRole("status").getByText("Disconnected", { exact: true }).waitFor();
  assert.deepEqual(errors, []);
});

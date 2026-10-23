// Optional real-browser regression: GLASS_PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { renderSummary, summaryContext } from "./fixtures/glass-summary-render.mjs";
import { renderGlassHome } from "./fixtures/glass-home-render.mjs";
const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const index = readFileSync(new URL("index.html", dist), "utf8");
const code = readFileSync(new URL(`_next/static/chunks/${index.match(/3859ru-[a-f0-9]{8}\.js/)[0]}`, dist), "utf8");
const original = readFileSync(new URL("_next/static/chunks/3859ru-aa5bf30a.js", dist), "utf8");
const prior = readFileSync(new URL("_next/static/chunks/3859ru-2f2505a3.js", dist), "utf8");
const styles = [...index.matchAll(/<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"/g)].map(([, name]) => readFileSync(new URL(name.replace(/^\//, ""), dist), "utf8")).join("\n");
const priorStyles = styles.replace(readFileSync(new URL(`../script/glass-visual.css`, import.meta.url), "utf8"), readFileSync(new URL("glass-visual-ecf8ea54.css", dist), "utf8"));

test("real Glass timing tower preserves all cells, sizing rows and left metrics with bounded keyboard scrolling", { skip: !process.env.GLASS_PLAYWRIGHT_MODULE }, async t => {
  const { chromium } = await import(process.env.GLASS_PLAYWRIGHT_MODULE);
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  t.after(() => browser.close());
  const page = await browser.newPage();
  const context = summaryContext([]);
  const render = async (source, width, container, css = styles) => {
    await page.setViewportSize({ width, height: 700 });
    await page.setContent(`<!doctype html><html class="dark"><meta charset="utf-8"><style>${css}</style><body><main style="width:${container ? `${container}px` : "auto"};margin:16px"><div class="space-y-3.5 sm:space-y-4 glass-node-home" data-node-view="cards">${renderSummary(source, context)}</div></main></body></html>`);
    await page.evaluate(() => document.fonts.ready);
    return page.evaluate(() => ({
      pageOverflow: document.documentElement.scrollWidth > innerWidth,
      cells: [...document.querySelectorAll(".overview-stat")].map(el => {
        const box = el.getBoundingClientRect();
        return { x: box.x, y: box.y, width: box.width, height: box.height, html: el.innerHTML,
          rows: [...el.children].map(row => { const r = row.getBoundingClientRect(); return { y: r.y, height: r.height }; }) };
      }),
      total: (() => { const e = document.querySelector(".overview-network-total"); if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.x, y: r.y, height: r.height, font: getComputedStyle(e).fontSize }; })(),
    }));
  };
  for (const [width, container] of [[320, 0], [390, 0], [768, 0], [1100, 0], [1280, 0], [1280, 720]]) {
    for (const host of [null, "akko", "edge-" + "very-long-hostname-".repeat(30), "high-rate"]) {
      const high = host === "high-rate";
      context.overview.speedUp = (high ? 600 : 16) * 1024 ** 2;
      context.overview.speedDown = (high ? 423.99 : 8) * 1024 ** 2;
      context.nodes = host ? Array.from({ length: 12 }, (_, i) => ({ uuid: String(i), name: i ? `edge-${i}-long-hostname` : host, online: true, net_in: (high ? 423.99 : 1) * 1024 ** 2 / (i + 1), net_out: (high ? 600 : 2) * 1024 ** 2 / (i + 1), updated_at_live: new Date().toISOString() })) : [];
      const baseline = await render(original, width, container);
      const installed = await render(prior, width, container, priorStyles);
      const current = await render(code, width, container);
      assert.equal(current.pageOverflow, false, `page overflow at ${width}/${container}`);
      assert.equal(current.cells.length, 4);
      for (let i = 0; i < 4; i++) {
        const { html: beforeHtml, ...before } = baseline.cells[i];
        const { html: afterHtml, ...after } = current.cells[i];
        assert.deepEqual(after, before, `cell ${i} geometry at ${width}/${container}`);
        if (i < 3) assert.equal(afterHtml, beforeHtml, `untouched cell ${i}`);
      }
      assert.deepEqual(current.total, installed.total, "preserve deployed left total font and position");
      const geometry = await page.evaluate(() => {
        const box = el => { const r = el.getBoundingClientRect(); return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, width: r.width, height: r.height, scrollWidth: el.scrollWidth, clientWidth: el.clientWidth, overflow: getComputedStyle(el).textOverflow, text: el.textContent }; };
        const cell = document.querySelector(".overview-network-stat"), rail = cell.querySelector(".overview-network-ranking");
        return { cell: box(cell), label: box(cell.querySelector(".overview-network-label")), total: box(cell.querySelector(".overview-network-total")), rail: box(rail), footer: box(cell.lastElementChild), footerHtml: cell.lastElementChild.outerHTML, rows: cell.children.length,
          entries: [...rail.querySelectorAll("li.overview-network-rank-row")].map(row => ({ row: box(row), host: box(row.querySelector(".overview-network-host")), speed: box(row.querySelector(".overview-network-host-speed")) })) };
      });
      assert.equal(geometry.rows, 3, "never add a sizing row");
      assert.equal(await page.locator(".overview-network-ranking").count(), 1);
      assert.ok(geometry.label.right <= geometry.rail.left + 1, "title/rail overlap");
      assert.ok(geometry.total.right <= geometry.rail.left + 1, `total/rail overlap at ${width}: ${JSON.stringify(geometry)}`);
      assert.ok(geometry.total.scrollWidth <= geometry.total.clientWidth + 1, `left total clipped ${width}/${host}: ${JSON.stringify(geometry)}`);
      assert.ok(geometry.rail.top >= geometry.cell.top && geometry.rail.bottom <= geometry.footer.top, "rail stays above unchanged footer and within cell");
      assert.ok(geometry.rail.right <= geometry.cell.right, "rail stays inside the cell");
      assert.ok(baseline.cells[3].html.endsWith(geometry.footerHtml), "original upload/download footer unchanged");
      if (host) {
        assert.equal(geometry.entries.length, 12, "do not discard lower ranks");
        const visible = geometry.entries.filter(e => e.row.bottom <= geometry.rail.bottom + 1).length;
        assert.equal(visible, width <= 640 ? 4 : 5, "four mobile or five desktop rows without growing the card");
        for (const e of geometry.entries) {
          assert.ok(e.host.width > 0, "hostname has an ellipsis column");
          assert.equal(e.host.overflow, "ellipsis");
          assert.ok(e.host.right <= e.speed.left + 1);
          assert.ok(e.speed.scrollWidth <= e.speed.clientWidth + 1 && e.speed.right <= geometry.rail.right + 1, "speed must be fully readable inside rail");
        }
        await page.locator(".overview-network-ranking").focus();
        await page.keyboard.press("End");
        await page.waitForFunction(() => document.querySelector(".overview-network-ranking").scrollTop > 0);
        assert.equal(await page.locator(".overview-network-ranking").evaluate(el => document.activeElement === el), true);
      }
      if (process.env.GLASS_SCREENSHOT_DIR && host === "akko") {
        await page.locator(".overview-network-ranking").evaluate(el => { el.scrollTop = 0; el.blur(); });
        await page.screenshot({ path: `${process.env.GLASS_SCREENSHOT_DIR}/glass-summary-${width}-${container}.png` });
      }
    }
  }
});

test("actual shipped card/list fixtures retain every node and metric bound beside the ranking", { skip: !process.env.GLASS_PLAYWRIGHT_MODULE }, async t => {
  const { chromium } = await import(process.env.GLASS_PLAYWRIGHT_MODULE);
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  t.after(() => browser.close());
  const page = await browser.newPage();
  // Local deterministic public-context fixtures; not production monitoring data.
  const nodes = ["berohost", "akko", "edge-singapore", "edge-tokyo", "edge-frankfurt", "edge-6", "edge-7", "edge-8"].map((name, i) => ({
    uuid: String(i), name, online: true, net_out: (8 - i) * 2048, net_in: 1024,
    updated_at_live: new Date().toISOString(), cpu: 7.1 + i, ram: 2 * 1024 ** 3, mem_total: 8 * 1024 ** 3, disk: 15 * 1024 ** 3, disk_total: 80 * 1024 ** 3,
    load: .12, load5: .08, load15: .05, uptime: 864000, net_total_up: 12 * 1024 ** 3, net_total_down: 9 * 1024 ** 3,
    traffic_limit: 0, price: 0, os: "Debian", region: "", tags: "", expired_at: "", display_ping_task_ids: [],
  }));
  const context = summaryContext(nodes);
  context.overview.speedUp = nodes.reduce((sum, n) => sum + n.net_out, 0);
  context.overview.speedDown = nodes.reduce((sum, n) => sum + n.net_in, 0);
  for (const width of [320, 390, 1280]) for (const view of ["cards", "list"]) {
    await page.setViewportSize({ width, height: 950 });
    const render = async (source, css) => {
      await page.setContent(`<!doctype html><html class="dark"><meta charset="utf-8"><style>${css}</style><body><main style="margin:16px">${renderGlassHome(source, context, view)}</main></body></html>`);
      await page.evaluate(async () => { await document.fonts.ready; for (const animation of document.getAnimations()) { animation.pause(); animation.currentTime = 0; } });
      return page.evaluate(() => [...document.querySelectorAll(".node-card, .node-card *")].map(el => { const r = el.getBoundingClientRect(); return [el.tagName, el.className.baseVal ?? el.className, r.x, r.y, r.width, r.height]; }));
    };
    const before = await render(prior, priorStyles);
    const after = await render(code, styles);
    assert.deepEqual(after, before, `${view}/${width}: all actual card and nested metric bounds unchanged`);
    assert.equal(await page.locator(".node-card").count(), nodes.length);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    const visible = await page.locator(".overview-network-ranking").evaluate(el => [...el.children].filter(row => row.getBoundingClientRect().bottom <= el.getBoundingClientRect().bottom + 1).length);
    assert.equal(visible, width <= 640 ? 4 : 5);
    if (process.env.GLASS_SCREENSHOT_DIR) await page.screenshot({ path: `${process.env.GLASS_SCREENSHOT_DIR}/glass-home-${view}-${width}.png`, fullPage: true });
  }
});

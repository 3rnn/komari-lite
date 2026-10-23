// Optional real-browser regression: GLASS_PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs
// node --test tests/glassNetworkSummary.browser.test.mjs
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { renderSummary, summaryContext } from "./fixtures/glass-summary-render.mjs";
const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const index = readFileSync(new URL("index.html", dist), "utf8");
const code = readFileSync(new URL(`_next/static/chunks/${index.match(/3859ru-[a-f0-9]{8}\.js/)[0]}`, dist), "utf8");
const original = readFileSync(new URL("_next/static/chunks/3859ru-aa5bf30a.js", dist), "utf8");
const styles = [...index.matchAll(/<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"/g)].map(([, name]) => readFileSync(new URL(name.replace(/^\//, ""), dist), "utf8")).join("\n");

test("real Glass summary keeps all cell geometry and three rows at mobile and narrow desktop widths", { skip: !process.env.GLASS_PLAYWRIGHT_MODULE }, async t => {
  const { chromium } = await import(process.env.GLASS_PLAYWRIGHT_MODULE);
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  t.after(() => browser.close());
  const page = await browser.newPage();
  const context = summaryContext([]);
  const render = async (source, width, container) => {
    await page.setViewportSize({ width, height: 700 });
    await page.setContent(`<!doctype html><html class="dark"><meta charset="utf-8"><style>${styles}</style><body><main style="width:${container ? `${container}px` : "auto"};margin:16px">${renderSummary(source, context)}</main></body></html>`);
    await page.evaluate(() => document.fonts.ready);
    return page.evaluate(() => ({
      pageOverflow: document.documentElement.scrollWidth > innerWidth,
      cells: [...document.querySelectorAll(".overview-stat")].map(el => {
        const box = el.getBoundingClientRect();
        return { x: box.x, y: box.y, width: box.width, height: box.height, html: el.innerHTML,
          rows: [...el.children].map(row => { const r = row.getBoundingClientRect(); return { y: r.y, height: r.height }; }) };
      }),
    }));
  };
  for (const [width, container] of [[320, 0], [390, 0], [768, 0], [1100, 0], [1280, 0], [1280, 720]]) {
    for (const host of [null, "akko", "edge-" + "very-long-hostname-".repeat(30), "high-rate"]) {
      const high = host === "high-rate";
      context.overview.speedUp = (high ? 600 : 16) * 1024 ** 2;
      context.overview.speedDown = (high ? 423.99 : 8) * 1024 ** 2;
      context.nodes = host ? [{ uuid: "a", name: host, online: true, net_in: (high ? 423.99 : 1) * 1024 ** 2, net_out: (high ? 600 : 2) * 1024 ** 2, updated_at_live: new Date().toISOString() }] : [];
      const baseline = await render(original, width, container);
      const current = await render(code, width, container);
      assert.equal(current.pageOverflow, false, `page overflow at ${width}/${container}`);
      assert.equal(current.cells.length, 4);
      for (let i = 0; i < 4; i++) {
        const { html: beforeHtml, ...before } = baseline.cells[i];
        const { html: afterHtml, ...after } = current.cells[i];
        assert.deepEqual(after, before, `cell ${i} geometry at ${width}/${container}`);
        if (i < 3) assert.equal(afterHtml, beforeHtml, `untouched cell ${i}`);
      }
      const geometry = await page.evaluate(() => {
        const box = selector => { const el = document.querySelector(selector); const r = el.getBoundingClientRect(); return { left: r.left, right: r.right, width: r.width, height: r.height, scrollWidth: el.scrollWidth, clientWidth: el.clientWidth, overflow: getComputedStyle(el).textOverflow, text: el.textContent }; };
        return { label: box(".overview-network-label"), host: box(".overview-network-host"), total: box(".overview-network-total"), speed: box(".overview-network-host-speed"), footer: document.querySelector(".overview-network-stat").lastElementChild.outerHTML, rows: document.querySelector(".overview-network-stat").children.length };
      });
      assert.equal(geometry.rows, 3, "never add a host row");
      assert.ok(geometry.label.right <= geometry.host.left + 1, "title/hostname overlap");
      assert.ok(geometry.total.right <= geometry.speed.left + 1, "total/host-speed overlap");
      assert.ok(geometry.total.scrollWidth <= geometry.total.clientWidth + 1, `total clipped at ${width}: ${JSON.stringify(geometry)}`);
      assert.ok(geometry.speed.scrollWidth <= geometry.speed.clientWidth + 1, "host speed clipped");
      assert.ok(geometry.host.width > 0);
      assert.equal(geometry.host.text, host ?? "—");
      assert.equal(geometry.host.overflow, "ellipsis");
      assert.ok(baseline.cells[3].html.endsWith(geometry.footer), "original total upload/download footer is unchanged");
      if (process.env.GLASS_SCREENSHOT_DIR && host === "akko") await page.screenshot({ path: `${process.env.GLASS_SCREENSHOT_DIR}/glass-summary-${width}-${container}.png` });
    }
  }
});

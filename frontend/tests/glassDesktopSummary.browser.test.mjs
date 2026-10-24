import assert from "node:assert/strict";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import test from "node:test";
import { renderGlassHome } from "./fixtures/glass-home-render.mjs";
import { summaryContext, renderSummary } from "./fixtures/glass-summary-render.mjs";
const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const index = readFileSync(new URL("index.html", dist), "utf8");
const code = readFileSync(new URL(`_next/static/chunks/${index.match(/3859ru-[a-f0-9]{8}\.js/)[0]}`, dist), "utf8");
const prior = readFileSync(new URL("_next/static/chunks/3859ru-30af502c.js", dist), "utf8");
const css = [...index.matchAll(/<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"/g)].map(([, name]) => readFileSync(new URL(name.replace(/^\//, ""), dist), "utf8")).join("\n");
const priorCss = css.replace(readFileSync(new URL("../script/glass-visual.css", import.meta.url), "utf8"), readFileSync(new URL("glass-visual-557fe8f7.css", dist), "utf8"));
test("desktop shorthand preserves shipped binary-byte formatter precision and full tooltip", () => {
  for (const [bytes, compact, full] of [[0, "0 B/s", "0 B/s"], [12.5, "12.50 B/s", "12.50 B/s"], [1024 * 42.72, "42.72 K/s", "42.72 KB/s"], [1024 ** 2 * 2.5, "2.50 M/s", "2.50 MB/s"], [1024 ** 3, "1.00 G/s", "1.00 GB/s"]]) {
    const html = renderSummary(code, summaryContext([{ uuid: "unit", name: "unit-host", online: true, net_out: bytes, net_in: 0, updated_at_live: new Date().toISOString() }]));
    assert.ok(html.includes(`data-short-speed="${compact}"`));
    assert.ok(html.includes(`title="1. unit-host: ${full}"`));
    assert.ok(html.includes(`>${full}</span>`), "mobile retains original formatter text");
  }
});
test("desktop compact five-row summary preserves exact 49914cf shared geometry and mobile DOM bounds", { skip: !process.env.GLASS_PLAYWRIGHT_MODULE }, async t => {
  const { chromium } = await import(process.env.GLASS_PLAYWRIGHT_MODULE);
  const browser = await chromium.launch({ args: ["--no-sandbox"] }); t.after(() => browser.close());
  const page = await browser.newPage({ reducedMotion: "reduce" });
  const nodes = Array.from({ length: 12 }, (_, i) => ({ uuid: String(i), name: i ? `edge-${i}-long-hostname` : "edge-very-long-hostname-".repeat(20), online: true, net_out: 600 * 1024 ** 2 / (i + 1), net_in: 423.99 * 1024 ** 2 / (i + 1), updated_at_live: new Date().toISOString(), cpu: 7.1, ram: 2 * 1024 ** 3, mem_total: 8 * 1024 ** 3, disk: 15 * 1024 ** 3, disk_total: 80 * 1024 ** 3, load: .12, load5: .08, load15: .05, uptime: 864000, net_total_up: 12 * 1024 ** 3, net_total_down: 9 * 1024 ** 3, traffic_limit: 0, price: 0, os: "Debian", region: "", tags: "", expired_at: "", display_ping_task_ids: [] }));
  const context = summaryContext(nodes);
  context.overview.speedUp = 600 * 1024 ** 2; context.overview.speedDown = 423.99 * 1024 ** 2;
  const measurements = [];
  for (const width of [320, 390, 640, 641, 768, 1024, 1280, 1440]) for (const theme of ["dark", "light"]) {
    await page.setViewportSize({ width, height: 1000 });
    const render = async (source, styles) => {
      await page.setContent(`<!doctype html><html class="${theme}"><meta charset="utf-8"><style>${styles}</style><body><main style="width:calc(100% - 64px);max-width:1364px;margin:32px auto">${renderGlassHome(source, context, "cards")}</main></body></html>`);
      await page.evaluate(async () => { await document.fonts.ready; for (const a of document.getAnimations()) { a.pause(); a.currentTime = 0; } });
      return page.evaluate(() => {
        const box = e => { const r = e.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; };
        return { cells: [...document.querySelectorAll(".overview-stat")].map(box), neighbors: [...document.querySelectorAll(".overview-stat:not(.overview-network-stat), .overview-stat:not(.overview-network-stat) *")].map(e => [e.tagName, e.className.baseVal ?? e.className, ...box(e)]), mobile: [...document.querySelectorAll(".overview-strip, .overview-strip *")].filter(e => !e.closest(".overview-network-desktop")).map(e => [e.tagName, e.className.baseVal ?? e.className, ...box(e)]) };
      });
    };
    const before = await render(prior, priorCss), after = await render(code, css);
    measurements.push({ width, theme, before: before.cells, after: after.cells });
    assert.deepEqual(after.cells, before.cells, `${width}/${theme}: ALL four summary cells unchanged`);
    assert.deepEqual(after.neighbors, before.neighbors, `${width}/${theme}: every neighboring-cell DOM bound unchanged`);
    if (width <= 640) { assert.deepEqual(after.mobile, before.mobile, `${width}/${theme}: every existing mobile DOM bound unchanged`); continue; }
    assert.equal(await page.locator(".overview-network-desktop-header").isVisible(), true, "desktop TOP 5 LIVE header visible");
    assert.equal(await page.locator(".overview-network-desktop-header").innerText(), "TOP 5\nLIVE");
    const geometry = await page.evaluate(() => {
      const rect = e => { const r = e.getBoundingClientRect(); return { x: r.x, y: r.y, right: r.right, bottom: r.bottom, width: r.width, height: r.height }; };
      const rail = document.querySelector(".overview-network-ranking");
      return { cell: rect(document.querySelector(".overview-network-stat")), rail: rect(rail), total: rect(document.querySelector(".overview-network-total")), rates: [...document.querySelectorAll(".overview-network-desktop-rates > span")].map(e => ({ ...rect(e), clipped: e.scrollWidth > e.clientWidth + 1 })), rows: [...rail.children].map(e => ({ ...rect(e), short: e.querySelector(".overview-network-host-speed").dataset.shortSpeed, title: e.title, clipped: e.querySelector(".overview-network-host-speed").scrollWidth > e.querySelector(".overview-network-host-speed").clientWidth + 1 })) };
    });
    assert.equal(geometry.rows.filter(r => r.bottom <= geometry.rail.bottom + .1).length, 5);
    assert.ok(geometry.total.right <= geometry.rail.x - 4, "main total never overlaps ranking");
    assert.equal(geometry.rates.length, 2);
    assert.ok(geometry.rates[1].y >= geometry.rates[0].bottom, "upload/download are two stacked lines");
    assert.ok(geometry.rates.every(r => !r.clipped && r.right <= geometry.rail.x - 4 && r.bottom <= geometry.cell.bottom));
    assert.ok(geometry.rows.every(r => !r.clipped && /(?:B|K|M|G|T|P)\/s$/.test(r.short) && /(?:KB|MB|GB)\/s/.test(r.title)), "shorthand retains exact formatter unit tooltip");
    if (process.env.GLASS_SCREENSHOT_DIR && theme === "dark" && [1280, 1440].includes(width)) {
      mkdirSync(process.env.GLASS_SCREENSHOT_DIR, { recursive: true });
      await page.locator(".overview-strip").screenshot({ path: `${process.env.GLASS_SCREENSHOT_DIR}/desktop-summary-${width}.png` });
    }
  }
  if (process.env.GLASS_SCREENSHOT_DIR) writeFileSync(`${process.env.GLASS_SCREENSHOT_DIR}/geometry-49914cf.json`, JSON.stringify(measurements, null, 2));
});

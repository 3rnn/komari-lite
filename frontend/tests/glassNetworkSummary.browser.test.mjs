// Execute the shipped renderers and styles, with exact deployed 58d6998 baselines.
import assert from "node:assert/strict";
import { readFileSync, mkdirSync } from "node:fs";
import test from "node:test";
import { build } from "esbuild";
import { renderSummary, summaryContext } from "./fixtures/glass-summary-render.mjs";
import { renderGlassHome } from "./fixtures/glass-home-render.mjs";
const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const index = readFileSync(new URL("index.html", dist), "utf8");
const code = readFileSync(new URL(`_next/static/chunks/${index.match(/3859ru-[a-f0-9]{8}\.js/)[0]}`, dist), "utf8");
const prior = readFileSync(new URL("_next/static/chunks/3859ru-fa8e62f6.js", dist), "utf8");
const styles = [...index.matchAll(/<link[^>]+rel="stylesheet"[^>]+href="([^"]+)"/g)].map(([, name]) => readFileSync(new URL(name.replace(/^\//, ""), dist), "utf8")).join("\n");
const priorStyles = styles.replace(readFileSync(new URL("../script/glass-visual.css", import.meta.url), "utf8"), readFileSync(new URL("glass-visual-8af54d66.css", dist), "utf8"));
const nodes = () => Array.from({ length: 12 }, (_, i) => ({ uuid: String(i), name: i ? `edge-${i}-long-hostname` : "edge-very-long-hostname-".repeat(20), online: true,
  net_out: 600 * 1024 ** 2 / (i + 1), net_in: 423.99 * 1024 ** 2 / (i + 1), updated_at_live: new Date().toISOString(),
  cpu: 7.1 + i, ram: 2 * 1024 ** 3, mem_total: 8 * 1024 ** 3, disk: 15 * 1024 ** 3, disk_total: 80 * 1024 ** 3, load: .12, load5: .08, load15: .05, uptime: 864000,
  net_total_up: 12 * 1024 ** 3, net_total_down: 9 * 1024 ** 3, traffic_limit: 0, price: 0, os: "Debian", region: "", tags: "", expired_at: "", display_ping_task_ids: [] }));

const enabled = !!process.env.GLASS_PLAYWRIGHT_MODULE;
test("mobile full-width overview is readable; desktop and all card/list metrics retain exact deployed geometry", { skip: !enabled }, async t => {
  const { chromium } = await import(process.env.GLASS_PLAYWRIGHT_MODULE);
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] });
  t.after(() => browser.close());
  const page = await browser.newPage({ reducedMotion: "reduce" });
  const context = summaryContext(nodes());
  context.overview.speedUp = 600 * 1024 ** 2; context.overview.speedDown = 423.99 * 1024 ** 2;
  const shots = process.env.GLASS_SCREENSHOT_DIR;
  if (shots) mkdirSync(shots, { recursive: true });
  for (const width of [320, 390, 640, 768, 1280]) for (const theme of ["light", "dark"]) for (const view of ["cards", "list"]) {
    await page.setViewportSize({ width, height: 1000 });
    const render = async (source, css) => {
      await page.setContent(`<!doctype html><html class="${theme === "dark" ? "dark" : ""}"><meta charset="utf-8"><style>${css}</style><body><main style="margin:16px">${renderGlassHome(source, context, view)}</main></body></html>`);
      await page.evaluate(async () => { await document.fonts.ready; for (const a of document.getAnimations()) { a.pause(); a.currentTime = 0; } });
      return page.evaluate(() => {
        const box = e => { const r = e.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; };
        const first = document.querySelector(".node-card").getBoundingClientRect();
        return { cells: [...document.querySelectorAll(".overview-stat")].map(box), rows: [...document.querySelectorAll(".overview-stat > div:nth-child(-n+3)")].map(e => getComputedStyle(e).display === "none" ? null : box(e)),
          cards: [...document.querySelectorAll(".node-card, .node-card *")].map(e => { const r = box(e); return [e.tagName, e.className.baseVal ?? e.className, r[0], r[2] || r[3] ? r[1] - first.y : r[1], r[2], r[3]]; }),
          total: box(document.querySelector(".overview-network-total")) };
      });
    };
    const before = await render(prior, priorStyles);
    const after = await render(code, styles);
    assert.deepEqual(after.cards, before.cards, `${view}/${width}/${theme}: nodecard relative metric geometry unchanged`);
    assert.equal(await page.locator(".node-card").count(), 12);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    if (width > 640) {
      assert.deepEqual(after.cells, before.cells, "desktop cells EXACT unchanged");
      // The newly added chart is display:none; the original three rows are untouched.
      assert.deepEqual(after.rows, before.rows, "desktop original sizing rows EXACT unchanged");
      assert.ok(after.total[2] > 0 && after.total[3] > 0, "desktop left total remains visible; redesign readability checked separately");
    }
    const geometry = await page.evaluate(() => {
      const box = e => { const r = e.getBoundingClientRect(); return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, width: r.width, height: r.height, scrollWidth: e.scrollWidth, clientWidth: e.clientWidth }; };
      const cells = [...document.querySelectorAll(".overview-stat")].map(box);
      const rail = document.querySelector(".overview-network-ranking");
      return { cells, rail: box(rail), total: box(document.querySelector(".overview-network-total")), chart: box(document.querySelector(".overview-network-chart")),
        footer: box(document.querySelector(".overview-network-stat > div:nth-child(3)")),
        rows: [...rail.children].map(e => ({ box: box(e), host: box(e.querySelector(".overview-network-host")), speed: box(e.querySelector(".overview-network-host-speed")) })) };
    });
    const visible = geometry.rows.filter(row => row.box.bottom <= geometry.rail.bottom + 1).length;
    assert.equal(visible, width <= 640 ? 4 : 5);
    if (width <= 640) {
      const topValues = await page.locator('.overview-stat:not(.overview-traffic-stat):not(.overview-network-stat) > div:nth-child(2) > span:first-child').evaluateAll(elements => elements.map(e => ({ height: e.getBoundingClientRect().height, clipped: e.scrollWidth > e.clientWidth + 1 })));
      assert.ok(topValues.every(value => value.height <= 28 && !value.clipped), "top two principal values stay readable on one line");
      assert.equal(geometry.cells[2].left, geometry.cells[0].left);
      assert.equal(geometry.cells[2].right, geometry.cells[1].right, "traffic full width");
      assert.equal(geometry.cells[3].width, geometry.cells[2].width, "network full width");
      assert.ok(geometry.cells[3].top >= geometry.cells[2].bottom, "network below traffic");
      assert.ok(geometry.chart.left >= geometry.total.right - 1, "sparkline and total never overlap");
      assert.ok(geometry.rail.top >= geometry.footer.bottom, "ranking below rates");
      assert.ok(geometry.rail.width > geometry.cells[3].width - 40, "full width readable ranking");
      assert.equal(geometry.rows[0].box.height, 28);
    }
    assert.ok(geometry.total.scrollWidth <= geometry.total.clientWidth + 1, "total speed not clipped");
    for (const row of geometry.rows) {
      assert.ok(row.host.width > 0 && row.host.right <= row.speed.left + 1);
      assert.ok(row.speed.scrollWidth <= row.speed.clientWidth + 1 && row.speed.right <= geometry.rail.right + 1, "rank speed fully readable");
    }
    await page.locator(".overview-network-ranking").focus(); await page.keyboard.press("End");
    await page.waitForFunction(() => document.querySelector(".overview-network-ranking").scrollTop > 0);
    if (shots && view === "cards") {
      // Chromium's native End-key scroll can still be in flight after scrollTop
      // first changes; settle it before resetting evidence to ranks 1–4.
      await page.waitForTimeout(300);
      await page.locator(".overview-network-ranking").evaluate(e => { e.style.scrollBehavior = "auto"; e.scrollTop = 0; e.blur(); });
      await page.waitForFunction(() => document.querySelector(".overview-network-ranking").scrollTop === 0);
      await page.locator(".overview-strip").screenshot({ path: `${shots}/glass-summary-${theme}-${width}.png` });
      await page.screenshot({ path: `${shots}/glass-home-${theme}-${width}.png`, fullPage: true });
    }
  }
});

test("browser mounts actual shipped streaming hook: observed SVG, gap, empty, reduced motion", { skip: !enabled }, async t => {
  const { chromium } = await import(process.env.GLASS_PLAYWRIGHT_MODULE);
  const browser = await chromium.launch({ headless: true, args: ["--no-sandbox"] }); t.after(() => browser.close());
  const start = code.indexOf("function rK("), end = code.indexOf("function rW()", start);
  const unitsStart = code.indexOf("let ek=["), unitsEnd = code.indexOf("function eA(", unitsStart);
  const result = await build({ stdin: { contents: `import * as R from 'react';import {jsx,jsxs} from 'react/jsx-runtime';import {createRoot} from 'react-dom/client';const L={jsx,jsxs};const eB=()=>window.glassContext;const icon=props=>jsx('svg',props);const rF=icon,rU=icon,rB=icon,rL=icon,r$=icon,rR=icon;const ex=v=>'¥'+v;const t9=(...v)=>v.filter(Boolean).join(' ');${code.slice(unitsStart, unitsEnd)};${code.slice(start,end)};const root=createRoot(document.getElementById('fixture'));window.renderFeed=()=>root.render(jsx(rH,{}));window.renderFeed();`, resolveDir: new URL("..", import.meta.url).pathname, loader: "js" }, bundle: true, write: false, format: "iife" });
  const page = await browser.newPage({ viewport: { width: 390, height: 700 }, reducedMotion: "reduce" });
  await page.setContent(`<!doctype html><html class="dark"><style>${styles}</style><main style="margin:16px" class="glass-node-home"><div id="fixture"></div></main></html>`);
  const context = summaryContext([]);
  await page.evaluate(c => { window.glassContext = c; window.clock = Date.now(); const NativeDate = Date; window.Date = class extends NativeDate { static now() { return window.clock; } }; }, context);
  await page.addScriptTag({ content: result.outputFiles[0].text });
  await page.waitForSelector(".overview-network-chart-empty");
  assert.equal(await page.locator(".overview-network-chart path").count(), 0);
  for (let i = 0; i < 20; i++) {
    await page.evaluate(i => { window.clock += i === 10 ? 5000 : 1000; window.glassContext.nodes = [{ uuid: "observed", name: "observed-edge", online: true, net_out: (i % 5) * 1024 ** 2, net_in: 1024 ** 2, updated_at_live: new Date(window.clock).toISOString() }]; window.glassContext.overview.speedUp = (i % 5) * 1024 ** 2; window.glassContext.overview.speedDown = 1024 ** 2; window.renderFeed(); }, i);
    await page.waitForFunction(n => Number(document.querySelector(".overview-network-chart").dataset.sampleCount) === n, i + 1);
  }
  assert.equal(await page.locator(".overview-network-chart path").count(), 2, "real silence splits SVG");
  assert.equal(await page.locator(".overview-network-chart").getAttribute("data-sample-count"), "20");
  for (const width of [320, 390, 640, 768, 1280]) for (const theme of ["light", "dark"]) {
    await page.setViewportSize({ width, height: 700 });
    await page.evaluate(theme => document.documentElement.className = theme === "dark" ? "dark" : "", theme);
    assert.equal(await page.locator('.overview-network-chart').isVisible(), width <= 640);
    if (width <= 640) {
      assert.equal(await page.locator('.overview-network-chart').evaluate(e => getComputedStyle(e).animationName), 'none');
      assert.ok(await page.locator('.overview-network-chart').evaluate(e => parseFloat(getComputedStyle(e).transitionDuration) <= .00001), 'no perceptible reduced-motion transition');
      const rates = await page.locator('.overview-network-stat > div:nth-child(3) .truncate').evaluateAll(elements => elements.map(e => e.scrollWidth <= e.clientWidth + 1));
      assert.ok(rates.every(Boolean), 'actual mounted rates are not clipped');
      if (process.env.GLASS_SCREENSHOT_DIR) await page.screenshot({ path: `${process.env.GLASS_SCREENSHOT_DIR}/glass-streaming-${theme}-${width}.png` });
    }
  }
  await page.evaluate(() => { window.glassContext.nodes = []; window.renderFeed(); });
  await page.waitForSelector('[data-network-empty="true"]');
  assert.equal(await page.locator(".overview-network-rank-row").count(), 0);
});

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import test from "node:test";
import { renderSummary, summaryContext, summaryRenderer } from "./fixtures/glass-summary-render.mjs";
import { jsx } from "react/jsx-runtime";
import { act, create } from "react-test-renderer";
globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const html = readFileSync(new URL("index.html", dist), "utf8");
const code = readFileSync(new URL(`_next/static/chunks/${html.match(/3859ru-[a-f0-9]{8}\.js/)[0]}`, dist), "utf8");
const now = Date.parse("2026-10-23T12:00:00Z");
const node = (uuid, up, down, extra = {}) => ({ uuid, name: uuid, online: true, net_out: up, net_in: down, updated_at_live: new Date(now).toISOString(), ...extra });
function selector() {
  const start = code.indexOf("function selectGlassTopThroughput(");
  const end = code.indexOf("function glassNetworkStat(", start);
  assert.ok(start > 0 && end > start, "the shipped chunk must include the tested throughput model");
  return runInNewContext(`${code.slice(start, end)};selectGlassTopThroughput`);
}
test("summary ranks existing public live nodes by upload plus download bytes per second", () => {
  const select = selector();
  const nodes = [node("upload", 50, 0), node("combined", 30, 40), node("download", 0, 60)];
  assert.equal(select(nodes, now).uuid, "combined");
  assert.equal(select(nodes, now).speed, 70);
  assert.equal(nodes[0].uuid, "upload", "do not sort the shared node feed");
});

test("only fresh online valid public samples qualify, with stable UUID ties and honest empty state", () => {
  const select = selector();
  const valid = node("valid", 3, 4);
  const invalid = [
    node("offline", 100, 100, { online: false }),
    node("stale", 100, 100, { updated_at_live: new Date(now - 60_001).toISOString() }),
    node("future", 100, 100, { updated_at_live: new Date(now + 1).toISOString() }),
    node("missing-time", 100, 100, { updated_at_live: undefined }),
    node("bad-time", 100, 100, { updated_at_live: "invalid" }),
    node("negative", -1, 100), node("nan", NaN, 100), node("infinite", Infinity, 100),
    node("string", "100", 100), node("overflow", Number.MAX_VALUE, Number.MAX_VALUE),
    node("hidden", 100, 100, { hidden: true }),
    node("blank-name", 100, 100, { name: " " }),
  ];
  assert.equal(select([...invalid, valid], now).uuid, "valid");
  assert.equal(select(invalid, now), null);
  assert.equal(select([], now), null);
  assert.equal(select(undefined, now), null);
  assert.equal(select([node("edge", 0, 0, { updated_at_live: new Date(now - 60_000).toISOString() })], now).speed, 0);
  for (const nodes of [[node("z", 3, 4), node("a", 4, 3)], [node("a", 4, 3), node("z", 3, 4)]]) {
    assert.equal(select(nodes, now).uuid, "a", "feed ordering must not flicker tied winners");
  }
});

test("the shipped summary renders compact host and speed in its original three rows, escaping host text", () => {
  const host = '<img src=x onerror="alert(1)"> & edge';
  const context = summaryContext([node("winner", 2 * 1024 ** 2, 1024 ** 2, { name: host, updated_at_live: new Date().toISOString() })]);
  const rendered = renderSummary(code, context);
  assert.match(rendered, /class="overview-network-host"/);
  assert.match(rendered, /&lt;img src=x onerror=&quot;alert\(1\)&quot;&gt; &amp; edge/);
  assert.doesNotMatch(rendered, /<img|dangerouslySetInnerHTML/);
  assert.match(rendered, /class="overview-network-host-speed"[^>]*>3.00 MB\/s</);
  assert.match(rendered, /24.00 MB\/s/);
  assert.match(rendered, /16.00 MB\/s/);
  assert.match(rendered, /8.00 MB\/s/);
  assert.equal((rendered.match(/class="overview-stat /g) ?? []).length, 4);
  const original = readFileSync(new URL("_next/static/chunks/3859ru-aa5bf30a.js", dist), "utf8");
  const firstThree = text => text.slice(0, text.indexOf("Live network speed")).slice(0, text.lastIndexOf('<div class="overview-stat', text.indexOf("Live network speed")));
  assert.equal(firstThree(rendered).replace(" overview-traffic-stat", ""), firstThree(renderSummary(original, context)), "other three cells retain original markup except the mobile traffic layout class");
  const empty = renderSummary(code, summaryContext([]));
  assert.match(empty, /class="overview-network-empty"[^>]*>No fresh online sample</);
  assert.doesNotMatch(empty, /class="overview-network-rank-row"/);
  assert.match(empty, /data-network-empty="true"/);
  context.settings.showSpeed = false;
  assert.doesNotMatch(renderSummary(code, context), /overview-network-host/);
});

test("ranking is one keyboard-focusable rail and expires a lower-ranked sample first", async () => {
  let clock = now;
  const timers = new Map();
  let id = 0;
  class Clock extends Date { static now() { return clock; } }
  const nodes = Array.from({ length: 9 }, (_, i) => node(`host-${i}`, 100 - i, 0));
  nodes[8].updated_at_live = new Date(now - 59_000).toISOString();
  const context = summaryContext(nodes);
  const Summary = summaryRenderer(code, context, { Date: Clock, setTimeout: (fn, delay) => { timers.set(++id, { fn, delay }); return id; }, clearTimeout: id => timers.delete(id) });
  let tree;
  await act(() => { tree = create(jsx(Summary, {})); });
  const rail = () => tree.root.findByProps({ className: "overview-network-ranking" });
  assert.equal(rail().props.tabIndex, 0);
  assert.match(rail().props["aria-label"], /Live throughput ranking/);
  assert.equal(tree.root.findAllByProps({ className: "overview-network-rank-row" }).length, 9);
  assert.equal([...timers.values()][0].delay, 1001);
  clock += 1001;
  const [key, timer] = [...timers.entries()][0];
  timers.delete(key);
  await act(() => timer.fn());
  assert.equal(tree.root.findAllByProps({ className: "overview-network-rank-row" }).length, 8);
  assert.equal(timers.size, 1);
  await act(() => tree.unmount());
  assert.equal(timers.size, 0);
});

test("the mounted summary follows feed updates and expires a frozen winner without fetching or polling", async () => {
  let clock = now;
  let nextId = 0;
  const timers = new Map();
  class Clock extends Date { static now() { return clock; } }
  const context = summaryContext([node("a", 50, 20), node("b", 10, 0)]);
  const Summary = summaryRenderer(code, context, {
    Date: Clock,
    setTimeout: (fn, delay) => { timers.set(++nextId, { fn, delay }); return nextId; },
    clearTimeout: id => timers.delete(id),
  });
  let tree;
  await act(() => { tree = create(jsx(Summary, {})); });
  const host = () => tree.root.findAllByProps({ className: "overview-network-host" })[0]?.children.join("") ?? "—";
  assert.equal(host(), "a");
  assert.equal(timers.size, 1, "only one local expiry timer, not a network poll");
  assert.equal([...timers.values()][0].delay, 60_001);
  context.nodes = [node("a", 0, 1), node("b", 200, 0)];
  await act(() => tree.update(jsx(Summary, {})));
  assert.equal(host(), "b");
  assert.equal(timers.size, 1);
  clock += 60_001;
  const [id, timer] = [...timers.entries()][0];
  timers.delete(id);
  await act(() => timer.fn());
  assert.equal(host(), "—", "no frozen live annotation after disconnect");
  assert.equal(timers.size, 0, "do not keep polling empty state");
  context.nodes = [node("a", 0, 0, { updated_at_live: new Date(clock).toISOString() })];
  await act(() => tree.update(jsx(Summary, {})));
  assert.equal(host(), "a", "real zero speed remains a valid live sample");
  await act(() => tree.unmount());
  assert.equal(timers.size, 0, "clean up expiry on unmount");
});

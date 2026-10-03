import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";
import { runInNewContext } from "node:vm";

const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const html = readFileSync(new URL("index.html", dist), "utf8");
const matches = [...html.matchAll(/3859ru-([a-f0-9]{8})\.js/g)];
const fingerprint = matches[0]?.[1];
const asset = fingerprint ? readFileSync(new URL(`_next/static/chunks/3859ru-${fingerprint}.js`, dist)) : Buffer.alloc(0);
const code = asset.toString("utf8");

test("Glass serves the content-hashed node-card bundle referenced by its HTML", () => {
  assert.ok(fingerprint);
  assert.equal(createHash("sha256").update(asset).digest("hex").slice(0, 8), fingerprint);
  assert.ok(matches.length >= 1);
  assert.ok(matches.every((match) => match[1] === fingerprint));
});

test("node cards use the tested per-task model instead of averaging different tasks", () => {
  assert.match(code, /function selectPingTaskData\(/);
  assert.match(code, /selectPingTaskData\(i\.history\.tasks,i\.history\.records/);
  assert.doesNotMatch(code, /avgLatency:rc\(r\.map\(e=>e\.avgLatency\)\)/);
  assert.match(code, /preferredId:r\.preferredPingTaskId/);
  assert.match(code, /v\.selectedName\?`Latency/);
});

test("node cards apply the server display IDs before choosing multi-task or single-task panels", () => {
  assert.match(code, /function resolveNodePingSelection\(/);
  assert.match(code, /resolveNodePingSelection\(r,n,e\.display_ping_task_ids,e\.display_ping_task_id\)/);
  assert.match(code, /r=selection\.showAll;n=selection/);
  assert.match(code, /selectPingTaskData\(i\.history\.tasks,i\.history\.records,Boolean\(n\?\.showAll\),n\?\.preferredId,rd,n\?\.displayIds,n\?\.fallbackShowAll,n\?\.fallbackPreferredId\)/);
  assert.match(code, /r=v\.showAll\?\?r;let g=/);
  assert.match(code, /r\?\(0,L\.jsx\)\(rI,\{rows:v\.tasks/);
  assert.match(code, /showAll:t\.showAll/);
});

test("per-task panels and cards use content-driven height and compact rows", () => {
  assert.match(code, /rows:t,loading:r,kind:n/);
  assert.ok(code.includes('className:"node-data-panel group/ping-panel gap-1.5 !overflow-visible p-2",style:{minHeight:0,justifyContent:"flex-start"}'));
  assert.ok(code.includes('className:"flex min-w-0 flex-col gap-2",children:t.map('));
  assert.ok(code.includes('style:{minHeight:r?0:void 0}'), "per-task cards must not inherit a fixed minimum height");
  assert.ok(code.includes('style:{justifyContent:r?"flex-start":void 0}'), "per-task sections must not stretch apart to fill taller cards");
  assert.ok(code.includes('className:"min-w-0 flex-1 truncate",children:e.label'), "long task labels must not push metrics outside narrow panels");
});

test("latency and loss timelines use up to 32 real history buckets with matching placeholders", () => {
  assert.match(code, /i=Math\.min\(32,t\.length\),a=Math\.max\(1,\(n-r\)\/i\)/);
  assert.match(code, /function rw\(e\)\{return Array\.from\(\{length:32\}/);
  assert.match(code, /length:32\},\(t,r\)=>\(\{key:`empty-\$\{r\}`/);
});

test("timeline buckets preserve sparse samples and aggregate dense real samples", () => {
  const from = code.indexOf("function rc(e){");
  const until = code.indexOf("function rf(e,t=!1){", from);
  assert.ok(from > 0 && until > from, "extract the pinned aggregation routine");
  const bucket = runInNewContext(`${code.slice(from, until)};rd`, Object.create(null));
  const record = (minutes, value) => ({ time: new Date(Date.UTC(2026, 0, 1, 0, minutes)).toISOString(), value });
  const sparse = bucket([record(0, 15), record(60, -1)]);
  assert.equal(sparse.length, 2, "do not invent samples to fill the strip");
  assert.equal(sparse[0].latency, 15);
  assert.equal(sparse[0].loss, 0);
  assert.equal(sparse[1].latency, null);
  assert.equal(sparse[1].loss, 100);
  const dense = bucket(Array.from({ length: 64 }, (_, i) => record(i, i + 1)));
  assert.equal(dense.length, 32);
  assert.ok(dense.every(({ latency, loss }) => latency > 0 && loss === 0));
  assert.equal(dense[0].latency, 1.5);
  assert.equal(dense.at(-1).latency, 63.5);
});

test("the 32-column strip has its own compact class and a subtle hover", () => {
  assert.match(code, /className:t9\("ping-timeline grid items-end gap-px/);
  assert.match(code, /group-hover\/ping-bar:scale-y-\[1\.15\]/);
});

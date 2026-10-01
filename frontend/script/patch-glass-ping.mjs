// Glass is shipped only as a generated chunk. Rebuild the small, reviewed
// node-card behavior from its pinned original without rewriting other modules.
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { Script } from "node:vm";

const dist = new URL("../../backend/web/public/bundledThemes/Glass/dist/", import.meta.url);
const chunks = new URL("_next/static/chunks/", dist);
const originalName = "3859ru-aa5bf30a.js";
const original = readFileSync(new URL(originalName, chunks), "utf8");
if (createHash("sha256").update(original).digest("hex").slice(0, 8) !== "aa5bf30a") {
  throw new Error("the pinned Glass vendor chunk has changed; review the patch again");
}
const vendorCss = readFileSync(new URL("1j6ltxvh3a3zn.css", chunks));
if (createHash("sha256").update(vendorCss).digest("hex") !== "14ffd5dfcde1d1b256c8c56750a2346e5bb92427dcd2ba14f480cc0a17c741f9") {
  throw new Error("the pinned Glass vendor CSS has changed; review the visual overrides again");
}
let patched = original;
function replaceOnce(before, after) {
  if (patched.split(before).length !== 2) throw new Error(`expected exactly one Glass anchor: ${before.slice(0, 75)}`);
  patched = patched.replace(before, after);
}

const model = readFileSync(new URL("glass-ping-view.mjs", import.meta.url), "utf8")
  .replace(/^export function /gm, "function ");
replaceOnce("function rP({node:e", `${model}\nfunction rP({node:e`);
replaceOnce(
  "function rP({node:e,pingEnabled:t,showCarrierPing:r,pingTaskSelection:n,onClick:i}){var a;let o=",
  "function rP({node:e,pingEnabled:t,showCarrierPing:r,pingTaskSelection:n,onClick:i}){var a;const selection=resolveNodePingSelection(r,n,e.display_ping_task_ids,e.display_ping_task_id);r=selection.showAll;n=selection;let o=",
);
const hookStart = "let t=function(e,t=!1){";
const hookEnd = "},[i,o,n?.telecom,n?.mobile,n?.unicom,e])}";
const start = patched.indexOf(hookStart, patched.indexOf("function rP({node:e"));
const end = patched.indexOf(hookEnd, start) + hookEnd.length;
if (start < 0 || end < start || patched.indexOf(hookStart, end) !== -1) {
  throw new Error("Glass node-card ping hook no longer matches the pinned build");
}
const replacement = [
  'let t=selectPingTaskData(i.history.tasks,i.history.records,Boolean(n?.showAll),n?.preferredId,rd,n?.displayIds,n?.fallbackShowAll,n?.fallbackPreferredId),a=t.selected,',
  'f=t.rows.map((row,index)=>({id:row.id,label:row.name,name:row.name,',
  'color:["#fb7185","#60a5fa","#34d399","#fbbf24"][index%4],',
  'latencyDisplay:row.avgLatency===null?"-":`${Math.round(row.avgLatency)} ms`,',
  'lossDisplay:row.loss===null?"-":`${row.loss.toFixed(1)}%`,',
  'latencyBars:row.history.length?row.history.map((point,index)=>({key:`latency-${row.id}-${point.time}-${index}`,',
  'className:point.latency===null?"bg-muted-foreground/15":rg(point.latency),',
  'tooltip:point.latency===null?`${rx(point.time)}\\nNo sample data`:`${rx(point.time)}\\n${Math.round(point.latency)} ms`})):rw("No sample data"),',
  'lossBars:row.history.length?row.history.map((point,index)=>({key:`loss-${row.id}-${point.time}-${index}`,',
  'className:point.loss===null?"bg-muted-foreground/15":rb(point.loss),',
  'tooltip:point.loss===null?`${rx(point.time)}\\nNo sample data`:`${rx(point.time)}\\n${point.loss.toFixed(1)}%`})):rw("No sample data")}));',
  'const p=a?.history??[];return{tasks:f,showAll:t.showAll,selectedName:a?.name||null,',
  'latencyDisplay:a?.avgLatency==null?"-":`${Math.round(a.avgLatency)} ms`,',
  'lossDisplay:a?.loss==null?"-":`${a.loss.toFixed(1)}%`,',
  'latencyBars:p.length?p.map((point,index)=>({key:`latency-${point.time}-${index}`,',
  'className:point.latency===null?"bg-muted-foreground/15":rg(point.latency),',
  'tooltip:point.latency===null?`${rx(point.time)}\\nNo sample data`:`${rx(point.time)}\\n${Math.round(point.latency)} ms`})):rw("No sample data"),',
  'lossBars:p.length?p.map((point,index)=>({key:`loss-${point.time}-${index}`,',
  'className:point.loss===null?"bg-muted-foreground/15":rb(point.loss),',
  'tooltip:point.loss===null?`${rx(point.time)}\\nNo sample data`:`${rx(point.time)}\\n${point.loss.toFixed(1)}%`})):rw("No sample data"),',
  'loading:o}},[i,o,n?.showAll,n?.preferredId,n?.displayIds,n?.fallbackShowAll,n?.fallbackPreferredId,e])}',
].join("");
patched = patched.slice(0, start) + replacement + patched.slice(end);
replaceOnce('}(e.uuid,t,1,r?n:null)', '}(e.uuid,t,1,n)');
replaceOnce('}(e.uuid,t,1,n),g=function(e)', '}(e.uuid,t,1,n);r=v.showAll??r;let g=function(e)');
replaceOnce('label:"Latency",value:v.latencyDisplay', 'label:v.selectedName?`Latency · ${v.selectedName}`:"Latency",value:v.latencyDisplay');
replaceOnce('className:"text-muted-foreground",children:e}),(0,L.jsx)("span",{className:"font-medium tabular-nums",children:t})]}),(0,L.jsx)(rT', 'className:"min-w-0 truncate text-muted-foreground",title:e,children:e}),(0,L.jsx)("span",{className:"font-medium tabular-nums",children:t})]}),(0,L.jsx)(rT');
replaceOnce('!e.online&&"node-card-offline"),children:', '!e.online&&"node-card-offline"),style:{minHeight:r?0:void 0},children:');
replaceOnce('className:"relative flex flex-1 flex-col justify-between gap-3.5 px-4 pb-4",children:', 'className:"relative flex flex-1 flex-col justify-between gap-3.5 px-4 pb-4",style:{justifyContent:r?"flex-start":void 0},children:');
replaceOnce('className:"node-data-panel group/ping-panel h-[7.75rem] gap-1.5 !overflow-visible p-2",children:', 'className:"node-data-panel group/ping-panel gap-1.5 !overflow-visible p-2",style:{minHeight:0,justifyContent:"flex-start"},children:');
replaceOnce('className:"flex min-h-0 flex-1 flex-col justify-between gap-1",children:t.map(', 'className:"flex min-w-0 flex-col gap-2",children:t.map(');
replaceOnce('className:"shrink-0 whitespace-nowrap",children:e.label', 'className:"min-w-0 flex-1 truncate",children:e.label');
replaceOnce('showCarrierPing:eI(n.showCarrierPing,!1),telecomPingTaskName:', 'showCarrierPing:eI(n.showCarrierPing,!0),preferredPingTaskId:Number.isSafeInteger(n.preferredPingTaskId)&&n.preferredPingTaskId>0?n.preferredPingTaskId:0,telecomPingTaskName:');
replaceOnce('pingTaskSelection:{telecom:r.telecomPingTaskName,mobile:r.mobilePingTaskName,unicom:r.unicomPingTaskName}', 'pingTaskSelection:{showAll:r.showCarrierPing,preferredId:r.preferredPingTaskId}');
new Script(patched, { filename: "Glass node-card chunk" });
const hash = createHash("sha256").update(patched).digest("hex").slice(0, 8);
const filename = `3859ru-${hash}.js`;
writeFileSync(new URL(filename, chunks), patched);
const visualCss = readFileSync(new URL("glass-visual.css", import.meta.url));
const cssHash = createHash("sha256").update(visualCss).digest("hex").slice(0, 8);
const cssName = `glass-visual-${cssHash}.css`;
writeFileSync(new URL(cssName, dist), visualCss);
const indexUrl = new URL("index.html", dist);
const index = readFileSync(indexUrl, "utf8");
const previous = [...new Set([...index.matchAll(/3859ru-[a-f0-9]{8}\.js/g)].map(([name]) => name))];
if (previous.length !== 1) throw new Error("Glass HTML must reference exactly one node-card chunk name");
const oldCss = [...new Set([...index.matchAll(/glass-visual-[a-f0-9]{8}\.css/g)].map(([name]) => name))];
if (oldCss.length > 1) throw new Error("Glass HTML references multiple visual stylesheets");
let updated = index.replaceAll(previous[0], filename);
if (oldCss.length) {
  updated = updated.replaceAll(oldCss[0], cssName);
} else {
  const link = '<link rel="stylesheet" href="/mobile-layout-v1.css"/>';
  if (updated.split(link).length !== 2) throw new Error("Glass HTML stylesheet anchor has changed");
  updated = updated.replace(link, `${link}<link rel="stylesheet" href="/${cssName}"/>`);
}
if (updated.includes('||"system"')) {
  const fallback = /\|\|(\\*)"system\1"/g;
  if ([...updated.matchAll(fallback)].length !== 2) throw new Error("Glass appearance bootstrap has changed");
  updated = updated.replace(fallback, (_match, escapes) => `||${escapes}"dark${escapes}"`);
}
if (updated !== index) writeFileSync(indexUrl, updated);
console.log(`Glass node-card asset ${filename}; visual asset ${cssName}; HTML references updated=${updated !== index}`);

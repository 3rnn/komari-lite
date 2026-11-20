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

// Overlap the live status handshake and first snapshot with the initial RPCs.
// The provider still waits for its initial data before showing the dashboard.
replaceOnce(
  '()=>{if(t)return;let e=(ez||(ez=new eT),ez),r=e.subscribe(c);return e.connect(),()=>{r(),e.disconnect()}},[t]',
  '()=>{let e=(ez||(ez=new eT),ez),r=e.subscribe((snapshot,connected)=>{if(connected)c(snapshot);setFeedLive(connected)});return e.connect(),()=>{r(),e.disconnect()}},[]',
);
replaceOnce('function eU({children:e}){let[t,r]', 'function eU({children:e}){const [feedLive,setFeedLive]=R.useState(null);let[t,r]');
replaceOnce('let N={loading:t,error:n,publicInfo:a', 'let N={feedLive,loading:t,error:n,publicInfo:a');
replaceOnce('this.listeners.forEach(e=>e(r))', 'this.listeners.forEach(listener=>listener(r,true))');
replaceOnce('this.ws=null,this.intentionalClose||this.scheduleReconnect())', 'this.ws=null,this.listeners.forEach(listener=>listener(null,false)),this.intentionalClose||this.scheduleReconnect())');
replaceOnce('console.error("[ws] connect failed:",e),this.scheduleReconnect()', 'console.error("[ws] connect failed:",e),this.listeners.forEach(listener=>listener(null,false)),this.scheduleReconnect()');
replaceOnce('"system"===t?t:"system",assetCurrency:', '"system"===t?t:"dark",assetCurrency:');

const model = readFileSync(new URL("glass-ping-view.mjs", import.meta.url), "utf8")
  .replace(/^export function /gm, "function ");
replaceOnce("function rP({node:e", `${model}\nfunction rP({node:e`);
// Use one raw history snapshot and one per-task summary on both surfaces.
replaceOnce('async function Y(e,t=1,r=6e3){let n=await F("public:getPingMetricStats",{entity_id:e,hours:t,max_points:r});return{stats:Array.isArray(n?.stats)?n.stats:[]}}', "");
const legacyStatsStart = patched.indexOf("function rf(e,t=!1){");
const legacyStatsEnd = patched.indexOf("let rp=new Map", legacyStatsStart);
if (legacyStatsStart < 0 || legacyStatsEnd <= legacyStatsStart) throw new Error("Glass legacy ping statistics have changed");
replaceOnce(patched.slice(legacyStatsStart, legacyStatsEnd), "");
replaceOnce(
  'async function ry(e,t){let[r,n]=await Promise.all([q(e,t),Y(e,t,6e3).catch(()=>null)]);return{history:r,metricStats:n?.stats??[]}}',
  'async function ry(e,t){return{history:await q(e,t)}}',
);
// Denser strips represent more *actual* samples, never interpolated values.
replaceOnce('i=Math.min(20,t.length),a=Math.max(1,(n-r)/i)', 'i=Math.min(32,t.length),a=Math.max(1,(n-r)/i)');
replaceOnce('function rw(e){return Array.from({length:20}', 'function rw(e){return Array.from({length:32}');
replaceOnce('"grid items-end gap-px opacity-85 transition-opacity duration-150 group-hover/ping-panel:opacity-100"', '"ping-timeline grid items-end gap-px opacity-85 transition-opacity duration-150 group-hover/ping-panel:opacity-100"');
replaceOnce('group-hover/ping-bar:scale-y-[1.6]', 'group-hover/ping-bar:scale-y-[1.15]');
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
  'f=t.rows.map((row,index)=>({id:row.id,label:row.name,name:row.name,avgLatency:row.avgLatency,',
  'color:["#fb7185","#60a5fa","#34d399","#fbbf24"][index%4],',
  'latencyDisplay:formatPingLatency(row.avgLatency),',
  'lossDisplay:row.loss===null?"-":`${row.loss.toFixed(1)}%`,',
  'latencyBars:row.history.length?row.history.map((point,index)=>({key:`latency-${row.id}-${point.time}-${index}`,',
  'className:point.latency===null?"bg-muted-foreground/15":rg(point.latency),',
  'tooltip:point.latency===null?`${rx(point.time)}\\nNo sample data`:`${rx(point.time)}\\n${Math.round(point.latency)} ms`})):rw("No sample data"),',
  'lossBars:row.history.length?row.history.map((point,index)=>({key:`loss-${row.id}-${point.time}-${index}`,',
  'className:point.loss===null?"bg-muted-foreground/15":rb(point.loss),',
  'tooltip:point.loss===null?`${rx(point.time)}\\nNo sample data`:`${rx(point.time)}\\n${point.loss.toFixed(1)}%`})):rw("No sample data")}));',
  'const p=a?.history??[];return{tasks:f,showAll:t.showAll,selectedName:a?.name||null,avgLatency:a?.avgLatency,',
  'latencyDisplay:formatPingLatency(a?.avgLatency),',
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
// The chart still receives individual samples. Only its legend is summarized.
replaceOnce('function jc({uuid:e,hours:t}){let[r,n]=(0,R.useState)(null),[i,a]=(0,R.useState)([]),', 'function jc({uuid:e,hours:t}){let[r,n]=(0,R.useState)(null),');
replaceOnce(
  'let[i,o]=await Promise.all([q(e,t),Y(e,t).catch(()=>null)]);r||(n(i),a(o?.stats??[]))}catch(e){r||(u(e instanceof Error?e.message:"Failed to load"),a([]))}',
  'let result=await rv(e,t);r||n(result.history)}catch(e){r||u(e instanceof Error?e.message:"Failed to load")}',
);
replaceOnce(
  '()=>{let t=new Map(rf(r?.records??[]).map(e=>[e.taskId,e])),n=rh(e,i);return h.flatMap((e,r)=>{let i=t.get(e.id);if(!i)return[];let a=i.latestLatency,o=n.get(e.id)??i.loss;return Number.isFinite(a)&&Number.isFinite(o)?[{id:e.id,name:e.name,latency:Number(a),loss:Number(o),color:ju[r%ju.length]}]:[]})},[r,i,h,e]',
  '()=>selectPingTaskData(h,r?.records,!0,0).rows.flatMap((row,index)=>row.loss===null?[]:[{...row,color:ju[index%ju.length]}]),[r,h]',
);
replaceOnce('Latency ${e.latency.toFixed(1)} ms', 'Average latency ${formatPingLatency(e.avgLatency)}');
replaceOnce('children:[e.latency.toFixed(1)," ms"]', 'children:formatPingLatency(e.avgLatency)');
replaceOnce('children:"Latency monitoring"}),y.length?', 'children:"Latency monitoring"}),y.length?(0,L.jsx)("p",{className:"text-xs text-muted-foreground",children:"Average latency and packet loss for the selected time range"}):null,y.length?');
replaceOnce('}(e.uuid,t,1,r?n:null)', '}(e.uuid,t,1,n)');
const cardHookStart = patched.indexOf("v=function(e,t=!0,r=1,n=null){", patched.indexOf("function rP("));
const cardHookEnd = patched.indexOf("}(e.uuid,t,1,n),g=function(e)", cardHookStart);
if (cardHookStart < 0 || cardHookEnd <= cardHookStart) throw new Error("Glass node ping consumer has changed");
const cardHook = patched.slice(cardHookStart + 2, cardHookEnd + 1);
replaceOnce(`v=${cardHook}(e.uuid,t,1,n),g=function(e)`, 'v=useGlassNodePing(e.uuid,t,1,n);r=v.showAll??r;let g=function(e)');
replaceOnce("function rP(", `${cardHook.replace("function(", "function useGlassNodePing(")}\nfunction rP(`);
replaceOnce('label:"Latency",value:v.latencyDisplay', 'label:v.selectedName?`Latency · ${v.selectedName}`:"Latency",value:v.latencyDisplay');
replaceOnce('bars:v.latencyBars,dimmed:!e.online', 'bars:v.latencyBars,latency:!0,dimmed:!e.online');
replaceOnce('function rC({label:e,value:t,bars:r,dimmed:n})', 'function rC({label:e,value:t,bars:r,dimmed:n,latency:i})');
replaceOnce('(0,L.jsx)(rT,{bars:r,fill:!0})', '(0,L.jsx)(rT,{bars:r,fill:!0,latency:i})');
replaceOnce('function rT({bars:e,fill:t=!1})', 'function rT({bars:e,fill:t=!1,latency:n=!1})');
replaceOnce('t?"min-h-0 flex-1":"h-2"),style:{gridTemplateColumns:', 't?"min-h-0 flex-1":"h-2",n&&"ping-latency"),style:{gridTemplateColumns:');
replaceOnce('className:"text-muted-foreground",children:e}),(0,L.jsx)("span",{className:"font-medium tabular-nums",children:t})]}),(0,L.jsx)(rT', 'className:"min-w-0 truncate text-muted-foreground",title:e,children:e}),(0,L.jsx)("span",{className:"font-medium tabular-nums",children:t})]}),(0,L.jsx)(rT');
replaceOnce('title:e,children:e}),(0,L.jsx)("span",{className:"font-medium tabular-nums",children:t})]}),(0,L.jsx)(rT', 'title:i?`${e} (1-hour average)`:e,children:e}),(0,L.jsx)("span",{className:"font-medium tabular-nums",children:t})]}),(0,L.jsx)(rT');
replaceOnce('!e.online&&"node-card-offline"),children:', '!e.online&&"node-card-offline"),style:{minHeight:r?0:void 0},children:');
replaceOnce('className:"relative flex flex-1 flex-col justify-between gap-3.5 px-4 pb-4",children:', 'className:"relative flex flex-1 flex-col justify-between gap-3.5 px-4 pb-4",style:{justifyContent:r?"flex-start":void 0},children:');
replaceOnce('className:"node-data-panel group/ping-panel h-[7.75rem] gap-1.5 !overflow-visible p-2",children:', 'className:"node-data-panel group/ping-panel gap-1.5 !overflow-visible p-2",style:{minHeight:0,justifyContent:"flex-start"},children:');
replaceOnce('className:"flex min-h-0 flex-1 flex-col justify-between gap-1",children:t.map(', 'className:"flex min-w-0 flex-col gap-2",children:t.map(');
replaceOnce('className:"shrink-0 whitespace-nowrap",children:e.label', 'className:"min-w-0 flex-1 truncate",children:e.label');
replaceOnce('(0,L.jsx)("span",{className:"size-2 shrink-0 rounded-full",style:{backgroundColor:e.color}}),(0,L.jsx)("span",{className:"min-w-0 flex-1 truncate"', '(0,L.jsx)("span",{className:"min-w-0 flex-1 truncate"');
replaceOnce('(0,L.jsx)(rT,{bars:"latency"===n?e.latencyBars:e.lossBars})', '(0,L.jsx)(rT,{bars:"latency"===n?e.latencyBars:e.lossBars,latency:"latency"===n})');
replaceOnce('showCarrierPing:eI(n.showCarrierPing,!1),telecomPingTaskName:', 'showCarrierPing:eI(n.showCarrierPing,!0),preferredPingTaskId:Number.isSafeInteger(n.preferredPingTaskId)&&n.preferredPingTaskId>0?n.preferredPingTaskId:0,telecomPingTaskName:');
replaceOnce('pingTaskSelection:{telecom:r.telecomPingTaskName,mobile:r.mobilePingTaskName,unicom:r.unicomPingTaskName}', 'pingTaskSelection:{showAll:r.showCarrierPing,preferredId:r.preferredPingTaskId}');
// Keep the existing cards; the list uses the same ping hook and route.
replaceOnce('function rz(){let{nodes:e,metricRetention:t,settings:r,goInstance:n,error:i,refresh:a}=eB();', 'function rz({visibleNodes,view,pingEnabled}){let{nodes:e,settings:r,goInstance:n,error:i,refresh:a}=eB();');
replaceOnce('className:"grid grid-cols-1 items-start gap-3 sm:grid-cols-[repeat(auto-fill,minmax(320px,1fr))]",children:e.map(e=>(0,L.jsx)(rP,{node:e,pingEnabled:t.pingHours>=1', 'className:"node-view-grid",children:visibleNodes.map(e=>(0,L.jsx)(rP,{node:e,pingEnabled');
replaceOnce('children:"Open admin"})]}):(0,L.jsx)("div",{className:"node-view-grid"', 'children:"Open admin"})]}):visibleNodes.length===0?L.jsx("p",{className:"glass-no-matches",role:"status",children:"No matching nodes"}):view==="list"?L.jsx(glassNodeTable,{nodes:visibleNodes,settings:r,pingEnabled,goInstance:n}):(0,L.jsx)("div",{className:"node-view-grid"');
replaceOnce('(0,L.jsx)(rl,{region:e.region,size:19})]})]}),(0,L.jsxs)("div"', '(0,L.jsx)(rl,{region:e.region,size:19}),L.jsx("span",{className:"glass-spearhead","aria-hidden":true})]})]}),(0,L.jsxs)("div"');
replaceOnce('(0,L.jsx)("img",{src:"/favicon.ico",alt:"",className:"size-9 shrink-0 object-contain"})', 'L.jsx("span",{className:"glass-site-mark","aria-hidden":true,children:"86"})');
replaceOnce('title:"Open admin",children:(0,L.jsx)(eJ,{className:"size-[18px]"})', 'title:"Open admin",children:"Admin"');
const nodeView = readFileSync(new URL("glass-node-view.mjs", import.meta.url), "utf8");
const networkSummary = readFileSync(new URL("glass-network-summary.mjs", import.meta.url), "utf8")
  .replace(/^export function /gm, "function ");
const overviewStart = patched.indexOf("function rK(");
const overviewEnd = patched.indexOf("let rV=", overviewStart);
if (overviewStart < 0 || overviewEnd <= overviewStart) throw new Error("Glass homepage has changed");
replaceOnce(patched.slice(overviewStart, overviewEnd), `${networkSummary}\n${nodeView}\n`);
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
// Keep the static Next export in sync with the hydrated navigation.
const oldBrand = '<img src="/favicon.ico" alt="" class="size-9 shrink-0 object-contain"/>';
const newBrand = '<span class="glass-site-mark" aria-hidden="true">86</span>';
if (!updated.includes(newBrand)) {
  if (updated.split(oldBrand).length !== 2) throw new Error("Glass static brand has changed");
  updated = updated.replace(oldBrand, newBrand);
}
const adminLink = updated.match(/<a href="\/admin"[^>]*title="Open admin">[^]*?<\/a>/g);
if (adminLink?.length !== 1) throw new Error("Glass static admin link has changed");
updated = updated.replace(adminLink[0], adminLink[0].replace(/<svg[^]*?<\/svg>/, "Admin"));
if (updated !== index) writeFileSync(indexUrl, updated);
console.log(`Glass node-card asset ${filename}; visual asset ${cssName}; HTML references updated=${updated !== index}`);

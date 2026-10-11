import { useEffect, useRef, useState } from "react";
import { Button, TextField } from "@radix-ui/themes";
import AdminPageTitle from "../../components/admin/AdminPageTitle.tsx";
import { AIAdvanced, AIMonitoringView, AISelect, type AIOverview } from "../../components/admin/AIMonitoring.tsx";
import { taskLabel } from "../../utils/aiMonitoring.ts";
import { aiRequest, type AISettings } from "../../utils/aiMonitoringApi.ts";
import { subscribeAdminRevocation, getAdminRevocationGeneration, adminRequestInvalidated } from "../../utils/adminRevocation.ts";

const defaults:AISettings={timezone:"Asia/Shanghai",retention_days:30,detailed_retention_hours:24};
function Field({label,value,onChange}:{label:string;value:string;onChange:(value:string)=>void}) {
 return <label className="flex min-w-0 flex-col gap-1 text-xs text-muted-foreground">{label}<TextField.Root aria-label={label} value={value} onChange={e=>onChange(e.target.value)} className="min-w-0"/></label>;
}
function AutomaticPricing() {
 return <section className="km-panel min-w-0 space-y-2 p-3" aria-label="Automatic pricing"><h2 className="km-kicker">Automatic pricing</h2><p className="text-sm font-medium">gpt-5.6-terra · OpenAI Codex</p><p className="text-xs text-muted-foreground">API-equivalent estimate only, not a ChatGPT subscription charge. Standard / short context: input $2 · cache read $0.20 · cache write $2.50 · output $12 per 1M tokens. Other models and tiers remain Unpriced. Historical price snapshots are unchanged.</p></section>;
}
export default function AIMonitoringPage() {
 const [data,setData]=useState<AIOverview|null>(null),[,setSettings]=useState<AISettings>(defaults);
 const [period,setPeriod]=useState("24H"),[timezone,setTimezone]=useState("Asia/Shanghai"),[node,setNode]=useState(""),[model,setModel]=useState(""),[task,setTask]=useState("");
 const [error,setError]=useState(""),[loading,setLoading]=useState(true),[settingsReady,setSettingsReady]=useState(false),[revoked,setRevoked]=useState(false),[reload,setReload]=useState(0);
 const controllers=useRef(new Set<AbortController>());
 const [choices,setChoices]=useState<{nodes:{value:string;label:string}[];models:{value:string;label:string}[];tasks:{value:string;label:string}[]}>({nodes:[],models:[],tasks:[]});
 useEffect(()=>{
  const activeControllers=controllers.current;
  const unsubscribe=subscribeAdminRevocation(()=>{for(const controller of activeControllers)controller.abort();activeControllers.clear();setData(null);setChoices({nodes:[],models:[],tasks:[]});setRevoked(true);setLoading(false);setError("");});
  return()=>{unsubscribe();for(const controller of activeControllers)controller.abort();activeControllers.clear();};
 },[]);
 useEffect(()=>{
  const activeControllers=controllers.current;if(revoked)return;
  const controller=new AbortController();activeControllers.add(controller);const generation=getAdminRevocationGeneration();
  aiRequest<AISettings>("settings","",controller.signal).then(saved=>{adminRequestInvalidated(generation);if(!controller.signal.aborted){setSettings(saved);setTimezone(saved.timezone||defaults.timezone);setSettingsReady(true);}}).catch(err=>{if(!controller.signal.aborted){setError(err instanceof Error?err.message:"Configuration unavailable");setSettingsReady(true);}});
  return()=>{controller.abort();activeControllers.delete(controller);};
 },[revoked]);
 useEffect(()=>{
  const activeControllers=controllers.current;if(revoked||!settingsReady)return;
  const controller=new AbortController();activeControllers.add(controller);let busy=false;
  const load=async()=>{if(busy||controller.signal.aborted)return;busy=true;const generation=getAdminRevocationGeneration();const query=new URLSearchParams({preset:period,timezone,node,model,task});try{const result=await aiRequest<AIOverview>("overview",query.toString(),controller.signal);adminRequestInvalidated(generation);if(!controller.signal.aborted){setData(result);setError("");setChoices(previous=>{const merge=(a:{value:string;label:string}[],b:{value:string;label:string}[])=>Array.from(new Map([...a,...b].map(option=>[option.value,option])).values());return {nodes:merge(previous.nodes,(result.nodes||[]).map(n=>({value:n.uuid,label:n.name}))),models:merge(previous.models,(result.models||[]).filter(m=>m.model).map(m=>({value:m.model!,label:m.model!}))),tasks:merge(previous.tasks,(result.tasks||[]).map(t=>({value:t.task||"unknown",label:taskLabel(t.task)})))};});}}catch(err){if(!controller.signal.aborted){setData(null);setError(err instanceof Error?err.message:"AI monitoring unavailable");}}finally{busy=false;if(!controller.signal.aborted)setLoading(false);}};
  setData(null);setLoading(true);void load();const timer=setInterval(()=>void load(),10000);
  return()=>{clearInterval(timer);controller.abort();activeControllers.delete(controller);};
 },[period,timezone,node,model,task,reload,revoked,settingsReady]);
 if(revoked)return <div role="alert" className="km-panel p-4">Session revoked. Sign in again to view AI monitoring.</div>;
 const all={value:"",label:"All"};
 return <main className="min-w-0 space-y-4" aria-label="Hermes AI monitoring">
 <header className="flex flex-wrap items-start justify-between gap-3"><AdminPageTitle description="Collected Hermes usage">AI MONITORING</AdminPageTitle><div className="flex min-w-0 flex-wrap items-end gap-2"><div className="flex gap-1">{["24H","7D","30D"].map(value=><Button key={value} size="1" aria-pressed={period===value} variant={period===value?"solid":"soft"} onClick={()=>setPeriod(value)}>{value}</Button>)}</div>{choices.nodes.length>1&&<div className="w-44"><AISelect label="Node" value={node} onChange={setNode} options={[all,...choices.nodes]}/></div>}<Button variant="soft" onClick={()=>setReload(v=>v+1)} disabled={loading}>Refresh</Button></div></header>
 {loading&&<p role="status">Loading AI monitoring…</p>}{error&&<div role="alert" className="km-panel p-3 text-[var(--orange-11)]">{error} <Button variant="soft" onClick={()=>setReload(v=>v+1)}>Retry</Button></div>}
 <AIMonitoringView data={data} timezone={timezone}/>
 <AIAdvanced data={data} timezone={timezone}><section aria-label="Advanced filters" className="grid min-w-0 gap-2 sm:grid-cols-3"><Field label="Timezone" value={timezone} onChange={setTimezone}/><AISelect label="Model" value={model} onChange={setModel} options={[all,...choices.models]}/><AISelect label="Task" value={task} onChange={setTask} options={[all,...choices.tasks]}/></section><AutomaticPricing/></AIAdvanced>
 </main>;
}

import { useEffect, useMemo, useRef, useState } from "react";
import { Button, TextField } from "@radix-ui/themes";
import AdminPageTitle from "../../components/admin/AdminPageTitle.tsx";
import { AIMonitoringView, AISelect, type AINode, type AIOverview } from "../../components/admin/AIMonitoring.tsx";
import { aiNumber, aiPollDelay, parseAIRate, taskLabel } from "../../utils/aiMonitoring.ts";
import { aiRequest, type AISettings } from "../../utils/aiMonitoringApi.ts";
import { adminNodeRequest } from "../../utils/adminNodeRequest.ts";
import { subscribeAdminRevocation, getAdminRevocationGeneration, adminRequestInvalidated } from "../../utils/adminRevocation.ts";

const defaults:AISettings={timezone:"Asia/Shanghai",retention_days:30,detailed_retention_hours:24,custom_models:[]};
type Selector={provider:string;model:string;billing_mode:string};
type Inventory={models:string[];tasks:string[];selectors:Selector[]};
const emptyInventory:Inventory={models:[],tasks:[],selectors:[]};
type PriceRule={id:number;provider:string;model:string;billing_mode:string;input_per_million?:number|null;cache_read_per_million?:number|null;cache_write_per_million?:number|null;output_per_million?:number|null;source:string;version:string;processing_mode:string;reference_tier?:string;effective_at:number};
const all={value:"",label:"All"};
const tzOptions=[{value:"UTC",label:"UTC"},{value:"Asia/Shanghai",label:"Beijing"}];
const numeric=parseAIRate;

function PricingRetention({settings,setSettings,models,rules,onRefresh,selectors}:{selectors:Selector[];settings:AISettings;setSettings:(value:AISettings)=>void;models:{value:string;label:string}[];rules:PriceRule[];onRefresh:()=>void}) {
 const [provider,setProvider]=useState(""),[mode,setMode]=useState("");
 const [model,setModel]=useState("gpt-5.6-terra"),[custom,setCustom]=useState(""),[input,setInput]=useState(""),[cacheRead,setCacheRead]=useState(""),[cacheWrite,setCacheWrite]=useState(""),[output,setOutput]=useState(""),[busy,setBusy]=useState(false),[message,setMessage]=useState("");
 useEffect(()=>{if(models.length&&!models.some(option=>option.value===model))setModel(models[0].value);},[models,model]);
 const addCustom=async()=>{const value=custom.trim();if(!value)return;setBusy(true);setMessage("");try{const next={...settings,custom_models:Array.from(new Set([...(settings.custom_models||[]),value]))};const saved=await aiRequest<AISettings>("settings","",undefined,next);setSettings(saved);setCustom("");setMessage("Custom model added.");onRefresh();}catch(error){setMessage(error instanceof Error?error.message:"Unable to save custom model");}finally{setBusy(false);}};
 const addPrice=async()=>{const payload={provider,model,billing_mode:mode,input_per_million:numeric(input),cache_read_per_million:numeric(cacheRead),cache_write_per_million:numeric(cacheWrite),output_per_million:numeric(output),source:"operator verified",version:"manual",processing_mode:"manual",reference_tier:"",effective_at:Math.floor(Date.now()/1000)};if(!selectors.some(row=>row.provider===provider&&row.model===model&&row.billing_mode===mode)){setMessage("Select an observed provider, model and billing mode.");return;}if(payload.input_per_million==null||payload.output_per_million==null){setMessage("Input and output prices are required.");return;}setBusy(true);setMessage("");try{await aiRequest<PriceRule>("pricing","",undefined,payload);setInput("");setCacheRead("");setCacheWrite("");setOutput("");setMessage("Future-only pricing rule added.");onRefresh();}catch(error){setMessage(error instanceof Error?error.message:"Unable to save price rule");}finally{setBusy(false);}};
 return <details className="km-panel min-w-0 p-4"><summary className="cursor-pointer font-semibold">Pricing & Retention</summary><div className="mt-4 space-y-4"><section aria-label="Manual pricing"><h2 className="km-kicker mb-3">Manual pricing</h2><div className="grid min-w-0 gap-2 sm:grid-cols-2 lg:grid-cols-5"><AISelect label="Provider" value={provider} onChange={value=>{setProvider(value);setMode("");}} options={[{value:"",label:"Select provider"},...Array.from(new Set(selectors.map(row=>row.provider))).map(value=>({value,label:value}))]}/><AISelect label="Model" value={model} onChange={value=>{setModel(value);setMode("");}} options={models}/><AISelect label="Billing mode" value={mode} onChange={setMode} options={[{value:"",label:"Select billing mode"},...Array.from(new Set(selectors.filter(row=>row.provider===provider&&row.model===model).map(row=>row.billing_mode))).map(value=>({value,label:value}))]}/>{[["Input / 1M",input,setInput],["Cache read / 1M",cacheRead,setCacheRead],["Cache write / 1M",cacheWrite,setCacheWrite],["Output / 1M",output,setOutput]].map(([label,value,change])=><label key={label as string} className="flex min-w-0 flex-col gap-1 text-xs text-muted-foreground">{label as string}<TextField.Root inputMode="decimal" value={value as string} onChange={event=>(change as (value:string)=>void)(event.target.value)}/></label>)}</div><div className="mt-3 flex flex-wrap items-center gap-2"><Button size="1" onClick={()=>void addPrice()} disabled={busy||!model}>Add future rule</Button><p className="text-xs text-muted-foreground">Rates are API-equivalent estimates and apply only to future matching samples.</p></div></section><section aria-label="Custom model"><h2 className="km-kicker mb-3">Custom model</h2><div className="flex min-w-0 flex-wrap gap-2"><TextField.Root aria-label="Custom model identifier" className="min-w-0 flex-1" placeholder="exact-model-id" value={custom} onChange={event=>setCustom(event.target.value)}/><Button size="1" variant="soft" onClick={()=>void addCustom()} disabled={busy||!custom.trim()}>Add model</Button></div></section><section aria-label="Saved pricing rules"><h2 className="km-kicker mb-3">Saved rules</h2>{rules.length?<div className="max-w-full overflow-x-auto"><table className="w-full text-left text-xs"><thead><tr>{["Model","Mode","Input","Cache read","Cache write","Output"].map(label=><th className="border-b border-[var(--gray-6)] p-2" key={label}>{label}</th>)}</tr></thead><tbody>{rules.map(rule=><tr key={rule.id}><td className="border-b border-[var(--gray-5)] p-2">{rule.model}</td><td className="border-b border-[var(--gray-5)] p-2">{rule.billing_mode||"Any"}</td><td className="border-b border-[var(--gray-5)] p-2">{aiNumber(rule.input_per_million)}</td><td className="border-b border-[var(--gray-5)] p-2">{aiNumber(rule.cache_read_per_million)}</td><td className="border-b border-[var(--gray-5)] p-2">{aiNumber(rule.cache_write_per_million)}</td><td className="border-b border-[var(--gray-5)] p-2">{aiNumber(rule.output_per_million)}</td></tr>)}</tbody></table></div>:<p className="text-sm text-muted-foreground">No saved pricing rules.</p>}</section>{message&&<p role="status" className="text-sm text-muted-foreground">{message}</p>}</div></details>;
}

export default function AIMonitoringPage() {
 const [data,setData]=useState<AIOverview|null>(null),[nodeOptions,setNodeOptions]=useState<Pick<AINode,"uuid"|"name">[]>([]),[settings,setSettings]=useState<AISettings>(defaults),[rules,setRules]=useState<PriceRule[]>([]);
 const [period,setPeriod]=useState("24H"),[timezone,setTimezone]=useState("Asia/Shanghai"),[node,setNode]=useState(""),[model,setModel]=useState(""),[task,setTask]=useState("");
 const [error,setError]=useState(""),[loading,setLoading]=useState(true),[settingsReady,setSettingsReady]=useState(false),[revoked,setRevoked]=useState(false),[reload,setReload]=useState(0);
 const [configError,setConfigError]=useState("");
 const [inventory,setInventory]=useState<Inventory>(emptyInventory),[inventoryError,setInventoryError]=useState("");
 const [configReload,setConfigReload]=useState(0);
 const initialized=useRef(false);
 const controllers=useRef(new Set<AbortController>());
 useEffect(()=>{const active=controllers.current;return subscribeAdminRevocation(()=>{for(const controller of active)controller.abort();active.clear();setData(null);setInventory(emptyInventory);setInventoryError("");setModel("");setTask("");setRules([]);setNodeOptions([]);setNode("");setSettings(defaults);setConfigError("");setRevoked(true);setLoading(false);setError("");});},[]);
 useEffect(()=>{if(revoked)return;const active=controllers.current;const controller=new AbortController();active.add(controller);const generation=getAdminRevocationGeneration();void Promise.all([aiRequest<AISettings>("settings","",controller.signal),aiRequest<PriceRule[]>("pricing","",controller.signal)]).then(([saved,pricing])=>{adminRequestInvalidated(generation);if(!controller.signal.aborted){setConfigError("");setSettings(saved);if(!initialized.current){setTimezone(saved.timezone||defaults.timezone);initialized.current=true;}setRules(pricing);setSettingsReady(true);}}).catch(err=>{if(!controller.signal.aborted){setConfigError(err instanceof Error?err.message:"Configuration unavailable");setSettingsReady(true);}});return()=>{controller.abort();active.delete(controller);};},[revoked,configReload]);
 const [filtersOpen,setFiltersOpen]=useState(false),[settingsOpen,setSettingsOpen]=useState(false);
 const queryKey=JSON.stringify([period,timezone,node,model,task]);
 const dataKey=useRef("");
 useEffect(()=>{
  if(revoked||!settingsReady)return;
  const active=controllers.current;const controller=new AbortController();active.add(controller);
  let busy=false,failures=0;let timer:ReturnType<typeof setTimeout>|undefined;
  const load=async()=>{
   if(busy||controller.signal.aborted)return;
   if(typeof document!=="undefined"&&document.visibilityState==="hidden"){timer=setTimeout(()=>void load(),aiPollDelay(failures));return;}
   busy=true;const generation=getAdminRevocationGeneration();
   try {
    const result=await aiRequest<AIOverview>("overview",new URLSearchParams({preset:period,timezone,node,model,task}).toString(),controller.signal);
    adminRequestInvalidated(generation);
    if(!controller.signal.aborted){failures=0;dataKey.current=queryKey;setData(result);setError("");}
   }catch(err){if(!controller.signal.aborted){failures++;setError(err instanceof Error?err.message:"AI monitoring unavailable");}}
   finally{busy=false;if(!controller.signal.aborted){setLoading(false);timer=setTimeout(()=>void load(),aiPollDelay(failures));}}
  };
  if(dataKey.current!==queryKey){setData(null);setError("");}
  setLoading(true);void load();
  return()=>{clearTimeout(timer);controller.abort();active.delete(controller);};
 },[period,timezone,node,model,task,queryKey,reload,revoked,settingsReady]);
 // Inventory is independent of AI reporting and filters. Retain known choices on transient failures.
 useEffect(()=>{
  if(revoked)return;
  const active=controllers.current;
  const controller=new AbortController();active.add(controller);
  const generation=getAdminRevocationGeneration();
  void adminNodeRequest("/api/admin/client/list",{cache:"no-store",signal:controller.signal})
   .then(async response=>{if(!response.ok)throw new Error("Node inventory unavailable");return response.json();})
   .then((nodes:AINode[])=>{
    adminRequestInvalidated(generation);
    if(!controller.signal.aborted&&Array.isArray(nodes)){setNodeOptions(nodes.map(({uuid,name})=>({uuid,name})));setNode(selected=>nodes.some(row=>row.uuid===selected)?selected:"");setInventoryError("");}
   }).catch(()=>{if(!controller.signal.aborted)setInventoryError("Node inventory unavailable; previously loaded choices may be stale.");});
  void aiRequest<Inventory>("inventory","",controller.signal).then(value=>{adminRequestInvalidated(generation);if(!controller.signal.aborted&&Array.isArray(value.models)&&Array.isArray(value.tasks))setInventory(value);}).catch(()=>{if(!controller.signal.aborted)setInventoryError("Filter inventory unavailable; previously loaded choices may be stale.");});
  return()=>{controller.abort();active.delete(controller);};
 },[reload,revoked]);
 const modelOptions=useMemo(()=>Array.from(new Set(inventory.models)).map(value=>({value,label:value})),[inventory.models]);
 const taskOptions=useMemo(()=>Array.from(new Set(inventory.tasks)).map(value=>({value,label:taskLabel(value)})),[inventory.tasks]);
 if(revoked)return <div role="alert" className="km-panel p-4">Session revoked. Sign in again to view AI monitoring.</div>;
 const visibleData=dataKey.current===queryKey?data:null;
 const disabled=Boolean(node&&visibleData?.nodes?.some(item=>item.uuid===node&&!item.enabled));
 const summary=[node?`Node: ${nodeOptions.find(item=>item.uuid===node)?.name||node}`:"All nodes",period,model?`Model: ${model}`:"All models",task?`Task: ${taskLabel(task)}`:"All tasks",timezone].join(" · ");
 const reset=()=>{setNode("");setModel("");setTask("");setTimezone(settings.timezone||defaults.timezone);};
 return <main className="min-w-0 space-y-3" aria-label="Hermes AI monitoring">
 <AdminPageTitle description="Collected Hermes usage">AI MONITORING</AdminPageTitle>
 <header className="km-panel flex min-w-0 flex-wrap items-end gap-2 p-3">
  <div className="w-44 max-w-full"><AISelect label="Node" value={node} onChange={setNode} options={[all,...nodeOptions.map(item=>({value:item.uuid,label:item.name}))]}/></div>
  <div className="flex gap-1">{["24H","7D","30D"].map(value=><Button key={value} size="1" aria-pressed={period===value} variant={period===value?"solid":"soft"} onClick={()=>setPeriod(value)}>{value}</Button>)}</div>
  <Button variant="soft" aria-expanded={filtersOpen} aria-controls="ai-filters" onClick={()=>setFiltersOpen(value=>!value)}>Filters</Button>
  <Button variant="soft" onClick={()=>setReload(value=>value+1)} disabled={loading}>Refresh</Button>
  <Button variant="soft" aria-expanded={settingsOpen} onClick={()=>setSettingsOpen(value=>!value)}>Settings</Button>
 </header>
 <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"><p className="min-w-0 break-words">{summary}</p><Button size="1" variant="ghost" onClick={reset}>Reset</Button></div>
 <section id="ai-filters" hidden={!filtersOpen} aria-label="Filters" className="km-panel grid min-w-0 gap-2 p-3 sm:grid-cols-3">
  <AISelect label="Model" value={model} onChange={setModel} options={[all,...modelOptions]}/><AISelect label="Task" value={task} onChange={setTask} options={[all,...taskOptions]}/><AISelect label="Timezone" value={timezone} onChange={setTimezone} options={tzOptions}/>
 </section>
 {configError&&<p role="alert">{configError}</p>}{inventoryError&&<p role="alert">{inventoryError}</p>}
 {loading&&<p role="status">{visibleData?"Refreshing current monitoring…":"Loading AI monitoring…"}</p>}
 {error&&<div role="alert" className="km-panel p-3 text-[var(--orange-11)]">{visibleData&&"Stale — last successful result. "}{error} <Button variant="soft" onClick={()=>setReload(value=>value+1)}>Retry</Button></div>}
 {disabled&&<p role="status" className="km-panel p-3">Monitoring disabled for this node. Historical collected usage is retained. <a href="/admin/servers">Open node configuration</a></p>}
 {visibleData&&<AIMonitoringView data={visibleData} timezone={timezone}/>}
 {settingsOpen&&!configError&&settingsReady&&<section role="dialog" aria-label="AI Monitoring Settings" aria-modal="false" className="km-panel fixed inset-y-0 right-0 z-50 w-full max-w-xl overflow-y-auto bg-[var(--color-panel-solid)] p-4 space-y-4" onKeyDown={event=>{if(event.key==="Escape")setSettingsOpen(false);}}>
  <div className="flex items-center justify-between gap-2"><h2 className="font-semibold">Settings</h2><Button autoFocus variant="soft" onClick={()=>setSettingsOpen(false)}>Close settings</Button></div>
  <PricingRetention selectors={inventory.selectors} settings={settings} setSettings={setSettings} models={modelOptions} rules={rules} onRefresh={()=>{setConfigReload(value=>value+1);setReload(value=>value+1);}}/>
  <section aria-label="Collection & Retention"><h3 className="km-kicker">Collection & Retention</h3><p className="mt-2 text-sm">Retention: {settings.retention_days} days · Detailed samples: {settings.detailed_retention_hours} hours · Default timezone: {settings.timezone}</p><p className="mt-2 text-xs text-muted-foreground">Read-only policy. Reducing retention can permanently remove historical data. Collection is opt-in per node; this page never enables it.</p><a href="/admin/servers" className="text-sm underline">Open node configuration</a></section>
 </section>}
 </main>;
}

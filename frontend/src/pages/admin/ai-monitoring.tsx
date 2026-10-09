import React, { useEffect, useRef, useState } from "react";
import { Button, TextField } from "@radix-ui/themes";
import AdminPageTitle from "../../components/admin/AdminPageTitle.tsx";
import { AIAdvanced, AIMonitoringView, AISelect, AI_COST_LABEL, AI_COST_HELP, type AIOverview } from "../../components/admin/AIMonitoring.tsx";
import { aiRange, aiISOSeconds, taskLabel } from "../../utils/aiMonitoring.ts";
import { aiRequest, referenceRule, type AISettings, type AIPricing, type PriceRule } from "../../utils/aiMonitoringApi.ts";
import { subscribeAdminRevocation, getAdminRevocationGeneration, adminRequestInvalidated } from "../../utils/adminRevocation.ts";

const defaults:AISettings={timezone:"Asia/Shanghai",retention_days:30,detailed_retention_hours:24};
function Field({label,value,onChange,type="text",required=false}:{label:string;value:string;onChange:(value:string)=>void;type?:"text"|"number";required?:boolean}) {
 return <label className="flex min-w-0 flex-col gap-1 text-xs text-muted-foreground">{label}<TextField.Root aria-label={label} type={type} value={value} required={required} onChange={e=>onChange(e.target.value)} className="min-w-0"/></label>;
}
export default function AIMonitoringPage() {
 const [data,setData]=useState<AIOverview|null>(null),[pricing,setPricing]=useState<AIPricing|null>(null),[settings,setSettings]=useState<AISettings>(defaults);
 const [period,setPeriod]=useState("24H"),[timezone,setTimezone]=useState("Asia/Shanghai"),[node,setNode]=useState(""),[model,setModel]=useState(""),[task,setTask]=useState("");

 const [error,setError]=useState(""),[configError,setConfigError]=useState(""),[loading,setLoading]=useState(true),[settingsReady,setSettingsReady]=useState(false),[revoked,setRevoked]=useState(false),[reload,setReload]=useState(0);
 const controllers=useRef(new Set<AbortController>());
 const [choices,setChoices]=useState<{nodes:{value:string;label:string}[];models:{value:string;label:string}[];tasks:{value:string;label:string}[]}>({nodes:[],models:[],tasks:[]});
 useEffect(()=>{
  const activeControllers=controllers.current;
  const unsubscribe=subscribeAdminRevocation(()=>{
   for(const controller of activeControllers) controller.abort();
   activeControllers.clear();setData(null);setPricing(null);setChoices({nodes:[],models:[],tasks:[]});setRevoked(true);setLoading(false);setError("");setConfigError("");
  });
  return()=>{unsubscribe();for(const controller of activeControllers)controller.abort();activeControllers.clear();};
 },[]);
 useEffect(()=>{
  const activeControllers=controllers.current;
  if(revoked) return;
  const controller=new AbortController(); activeControllers.add(controller);
  const generation=getAdminRevocationGeneration();
  Promise.all([aiRequest<AIPricing>("pricing","",controller.signal),aiRequest<AISettings>("settings","",controller.signal)]).then(([prices,saved])=>{adminRequestInvalidated(generation);if(controller.signal.aborted)return;setPricing(prices);setSettings(saved);setTimezone(saved.timezone || defaults.timezone);setConfigError("");setSettingsReady(true);}).catch(err=>{if(!controller.signal.aborted){setConfigError(err instanceof Error ? err.message:"Configuration unavailable");setSettingsReady(true);}});
  return()=>{controller.abort();activeControllers.delete(controller);};
 },[revoked]);
 useEffect(()=>{
  const activeControllers=controllers.current;
  if(revoked || !settingsReady) return;
  const controller=new AbortController();activeControllers.add(controller);let busy=false;
  const load=async()=>{
   if(busy || controller.signal.aborted) return;busy=true;
   const generation=getAdminRevocationGeneration();
   const range=aiRange(period);
   if(!range){setLoading(false);busy=false;return;}
   const query=new URLSearchParams({from:String(range.from),to:String(range.to),timezone,node,model,task});
   try {const result=await aiRequest<AIOverview>("overview",query.toString(),controller.signal);adminRequestInvalidated(generation);if(!controller.signal.aborted){setData(result);setError("");setChoices(previous=>{
 const merge=(a:{value:string;label:string}[],b:{value:string;label:string}[])=>Array.from(new Map([...a,...b].map(option=>[option.value,option])).values());
 return {nodes:merge(previous.nodes,(result.nodes || []).map(n=>({value:n.uuid,label:n.name}))),models:merge(previous.models,(result.models || []).filter(m=>m.model).map(m=>({value:m.model!,label:m.model!}))),tasks:merge(previous.tasks,(result.tasks || []).map(t=>({value:t.task || "unknown",label:taskLabel(t.task)})))};
});}}
   catch(err){if(!controller.signal.aborted){setData(null);setError(err instanceof Error ? err.message:"AI monitoring unavailable");}}
   finally{busy=false;if(!controller.signal.aborted)setLoading(false);}
  };
  setData(null);setLoading(true);void load();const timer=setInterval(()=>void load(),10000);
  return()=>{clearInterval(timer);controller.abort();activeControllers.delete(controller);};
 },[period,timezone,node,model,task,reload,revoked,settingsReady]);
 const saveConfig=async(resource:"settings"|"pricing",body:unknown)=>{
  const generation=getAdminRevocationGeneration(),controller=new AbortController();controllers.current.add(controller);
  try {
   await aiRequest(resource,"",controller.signal,body);
   const result=await aiRequest<AISettings&AIPricing>(resource,"",controller.signal);adminRequestInvalidated(generation);
   if(controller.signal.aborted)throw new DOMException("Request aborted","AbortError");
   if(resource==="settings") {const expected=body as AISettings;if(result.timezone!==expected.timezone || result.retention_days!==expected.retention_days || result.detailed_retention_hours!==expected.detailed_retention_hours)throw new Error("Saved settings could not be verified");setSettings(result);setTimezone(result.timezone);}
   else {const expected=body as PriceRule;if(!result.rules?.some(rule=>rule.model===expected.model && rule.provider===expected.provider && rule.version===expected.version && Date.parse(rule.effective_at)===Date.parse(expected.effective_at) && rule.processing_mode===expected.processing_mode && rule.reference_tier===expected.reference_tier && rule.reference_model===expected.reference_model && rule.billing_mode===expected.billing_mode && rule.input_per_million===expected.input_per_million && rule.output_per_million===expected.output_per_million && rule.cache_read_per_million===expected.cache_read_per_million && rule.cache_write_per_million===expected.cache_write_per_million && rule.source===expected.source && rule.source_url===expected.source_url))throw new Error("Appended pricing rule could not be verified");setPricing(result);}
   setReload(value=>value+1);
  }finally{controllers.current.delete(controller);}
 };
 if(revoked)return <div role="alert" className="km-panel p-4">Session revoked. Sign in again to view AI monitoring.</div>;
 const all={value:"",label:"All"};
 return <main className="min-w-0 space-y-4" aria-label="Hermes AI monitoring">
 <header className="flex flex-wrap items-start justify-between gap-3"><AdminPageTitle description="Collected Hermes usage">AI MONITORING</AdminPageTitle><div className="flex min-w-0 flex-wrap items-end gap-2"><div className="flex gap-1">{["24H","7D","30D"].map(value=><Button key={value} size="1" aria-pressed={period===value} variant={period===value ? "solid":"soft"} onClick={()=>setPeriod(value)}>{value}</Button>)}</div>{choices.nodes.length>1&&<div className="w-44"><AISelect label="Node" value={node} onChange={setNode} options={[all,...choices.nodes]}/></div>}<Button variant="soft" onClick={()=>setReload(v=>v+1)} disabled={loading}>Refresh</Button></div></header>
 {loading && <p role="status">Loading AI monitoring…</p>}{error && <div role="alert" className="km-panel p-3 text-[var(--orange-11)]">{error} <Button variant="soft" onClick={()=>setReload(v=>v+1)}>Retry</Button></div>}
 <AIMonitoringView data={data} timezone={timezone}/>
 {configError && <p role="alert">{configError}</p>}
 <AIAdvanced data={data} timezone={timezone}><section aria-label="Advanced filters" className="grid min-w-0 gap-2 sm:grid-cols-3"><Field label="Timezone" value={timezone} onChange={setTimezone}/><AISelect label="Model" value={model} onChange={setModel} options={[all,...choices.models]}/><AISelect label="Task" value={task} onChange={setTask} options={[all,...choices.tasks]}/></section><PricingSettings pricing={pricing} settings={settings} onSave={saveConfig}/></AIAdvanced>
 </main>;
}
function PricingSettings({pricing,settings,onSave}:{pricing:AIPricing|null;settings:AISettings;onSave:(resource:"pricing"|"settings",body:unknown)=>Promise<void>}) {
 const [reference,setReference]=useState(""),[provider,setProvider]=useState(""),[model,setModel]=useState(""),[billingMode,setBillingMode]=useState(""),[mode,setMode]=useState(""),[tier,setTier]=useState(""),[source,setSource]=useState(""),[sourceUrl,setSourceUrl]=useState(""),[version,setVersion]=useState(""),[effective,setEffective]=useState("");
 const [rates,setRates]=useState({input:"",output:"",read:"",write:""});
 const [draft,setDraft]=useState(settings),[message,setMessage]=useState(""),[pending,setPending]=useState(false);
 useEffect(()=>setDraft(settings),[settings]);
 const append=async(e:React.FormEvent)=>{
  e.preventDefault();setMessage("");if(pending)return;
  try{
   if(!model.trim() || !effective || !Number.isFinite(Date.parse(effective)))throw new Error("Exact local model and effective ISO timestamp are required.");
   aiISOSeconds(effective);
   const entry=reference ? pricing?.reference_catalog?.entries[Number(reference)-1]:undefined;
   let rule:PriceRule;
   if(entry && pricing?.reference_catalog)rule=referenceRule(entry,pricing.reference_catalog,model,billingMode,new Date(effective).toISOString());
   else {
    const numeric=(value:string,nullable=false)=>{if(!value.trim()){if(nullable)return null;throw new Error("Input and output reference rates are required.");}const n=Number(value);if(!Number.isFinite(n)||n<0)throw new Error("Rates must be finite and nonnegative.");return n;};
    if(!provider.trim()||!mode.trim()||!tier.trim()||!source.trim()||!version.trim())throw new Error("Provider, processing mode, reference tier, source and version are required. No mode or tier is inferred.");
    rule={provider,model,billing_mode:billingMode,processing_mode:mode,reference_tier:tier,input_per_million:numeric(rates.input)!,output_per_million:numeric(rates.output)!,cache_read_per_million:numeric(rates.read,true),cache_write_per_million:numeric(rates.write,true),source,source_url:sourceUrl,version,effective_at:new Date(effective).toISOString()};
   }
   setPending(true);await onSave("pricing",rule);setMessage("Pricing version appended and verified. Historical costs are unchanged.");
  }catch(err){setMessage(err instanceof Error ? err.message:"Could not append pricing rule");}finally{setPending(false);}
 };
 const saveSettings=async(e:React.FormEvent)=>{e.preventDefault();if(pending)return;setMessage("");try{new Intl.DateTimeFormat("en",{timeZone:draft.timezone});if(!Number.isInteger(draft.retention_days)||draft.retention_days<1||draft.retention_days>30||!Number.isInteger(draft.detailed_retention_hours)||draft.detailed_retention_hours<1||draft.detailed_retention_hours>24)throw new Error("Retention must be 1–30 days; detailed retention must be 1–24 hours.");setPending(true);await onSave("settings",draft);setMessage("Settings saved and verified.");}catch(err){setMessage(err instanceof Error ? err.message:"Could not save settings");}finally{setPending(false);}};
 const catalog=pricing?.reference_catalog,entry=reference ? catalog?.entries[Number(reference)-1]:undefined;
 return <section className="km-panel min-w-0 space-y-4 p-3" aria-label="Pricing and retention settings"><h2 className="km-kicker">Pricing & retention</h2><p className="text-sm text-muted-foreground">{AI_COST_HELP} Rules are append-only. Unknown aliases and unspecified modes or context tiers remain Unpriced. No historical repricing.</p>
 <form onSubmit={append} className="min-w-0 space-y-3"><h3 className="font-semibold">Append explicit pricing version</h3><AISelect label="Official reference (provider / exact model / processing mode / context tier)" value={reference} onChange={setReference} options={[{value:"",label:"Administrator override — no automatic reference mapping"},...(catalog?.entries || []).map((entry,index)=>({value:String(index+1),label:`${entry.provider} / ${entry.model} / ${entry.processing_mode} / ${entry.context_tier}`}))]}/>{entry && <div className="break-all text-xs text-muted-foreground">Explicit reference: {entry.provider} / {entry.model} / {entry.processing_mode} / {entry.context_tier}<br/>Input {entry.input_per_million} · output {entry.output_per_million} · cache read {entry.cache_read_per_million ?? "Unpriced"} · cache write {entry.cache_write_per_million ?? "Unpriced"} USD per million tokens<br/>Source {catalog?.source_url} · version {catalog?.version} · verified {catalog?.verified_at || "N/A"}</div>}
 <div className="grid min-w-0 gap-2 sm:grid-cols-2"><Field label="Exact observed local model (no alias guessing)" value={model} onChange={setModel} required/><Field label="Exact billing mode (blank only matches blank)" value={billingMode} onChange={setBillingMode}/><Field label="Effective at (ISO timestamp with offset)" value={effective} onChange={setEffective} required/></div>
 {!entry && <div className="grid min-w-0 gap-2 sm:grid-cols-2 xl:grid-cols-3"><Field label="Provider" value={provider} onChange={setProvider} required/><Field label="Processing mode" value={mode} onChange={setMode} required/><Field label="Reference / context tier" value={tier} onChange={setTier} required/>{([['input','Noncached input'],['output','Output (includes reasoning)'],['read','Cache read'],['write','Cache write']] as const).map(([key,label])=><Field key={key} label={`${label} USD per million${key==='read'||key==='write' ? ' (blank = unpriced)':''}`} value={rates[key]} onChange={value=>setRates(prev=>({...prev,[key]:value}))}/>)}<Field label="Source" value={source} onChange={setSource} required/><Field label="Source URL" value={sourceUrl} onChange={setSourceUrl}/><Field label="Version" value={version} onChange={setVersion} required/></div>}
 <Button type="submit" disabled={pending || !pricing} aria-busy={pending}>Append pricing rule</Button></form>
 <div className="max-w-full overflow-x-auto"><table className="w-full text-left text-xs"><caption className="text-left mb-2">Immutable pricing versions · {AI_COST_LABEL}</caption><thead><tr>{["Provider / model","Billing mode / processing mode / tier","Rates per million: input / cache read / cache write / output","Source / version","Effective at"].map(label=><th key={label} className="p-2">{label}</th>)}</tr></thead><tbody>{pricing?.rules.map((rule,index)=><tr key={`${rule.version}/${index}`} className="border-t border-[var(--gray-6)]"><td className="break-all p-2">{rule.provider} / {rule.model}{rule.reference_model && <p>Reference SKU: {rule.reference_model}</p>}</td><td className="p-2">{rule.billing_mode || "(blank)"} / {rule.processing_mode} / {rule.reference_tier || "Unspecified"}</td><td className="p-2">{rule.input_per_million} / {rule.cache_read_per_million ?? "Unpriced"} / {rule.cache_write_per_million ?? "Unpriced"} / {rule.output_per_million}</td><td className="break-all p-2">{rule.source}<br/>{rule.version}<br/>{rule.source_url}</td><td className="p-2">{rule.effective_at}</td></tr>)}</tbody></table>{!pricing?.rules.length && <p className="text-xs text-muted-foreground">No pricing rules available. Unmapped models remain Unpriced.</p>}</div>
 <form onSubmit={saveSettings} className="space-y-3"><h3 className="font-semibold">Timezone & retention</h3><div className="grid min-w-0 gap-2 sm:grid-cols-3"><Field label="Default timezone" value={draft.timezone} onChange={value=>setDraft(prev=>({...prev,timezone:value}))} required/><Field label="Retention days (1–30)" value={String(draft.retention_days)} type="number" onChange={value=>setDraft(prev=>({...prev,retention_days:Number(value)}))}/><Field label="Detailed retention hours (1–24)" value={String(draft.detailed_retention_hours)} type="number" onChange={value=>setDraft(prev=>({...prev,detailed_retention_hours:Number(value)}))}/></div><Button type="submit" disabled={pending || !pricing} aria-busy={pending}>Save settings</Button></form>{message && <p role="status" className="break-words text-sm">{message}</p>}
 </section>;
}

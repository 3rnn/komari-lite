import { adminNodeRequest } from "./adminNodeRequest.ts";
import { isAdminLogoutPending } from "./adminRevocation.ts";
export type AISettings = { timezone:string; retention_days:number; detailed_retention_hours:number; custom_models?:string[] };
export type PriceEntry = { provider:string;model:string;processing_mode:string;context_tier:string;input_per_million:number|string;output_per_million:number|string;cache_read_per_million:number|string|null;cache_write_per_million:number|string|null };
export type PriceRule = { provider:string;model:string;billing_mode:string;processing_mode:string;reference_model?:string;reference_tier?:string;input_per_million:number;output_per_million:number;cache_read_per_million:number|null;cache_write_per_million:number|null;source:string;source_url:string;version:string;effective_at:string };
export type ReferenceCatalog = { source:string;source_url:string;version:string;verified_at?:string;source_sha256?:string;entries:PriceEntry[] };
export type AIPricing = { rules:PriceRule[];reference_catalog?:ReferenceCatalog };
/** Returns a catalog position only for a verified exact standard/short model SKU. */
export function defaultOfficialReference(entries:PriceEntry[], model:string): string {
 const index=entries.findIndex(entry=>entry.provider==="openai" && entry.model===model && entry.processing_mode==="standard" && entry.context_tier==="short");
 return index < 0 ? "" : String(index+1);
}
export function referenceRule(entry:PriceEntry,catalog:Pick<ReferenceCatalog,"source"|"source_url"|"version">,model:string,billingMode:string,effectiveAt:string):PriceRule {
 return {provider:entry.provider,model,billing_mode:billingMode,processing_mode:entry.processing_mode,reference_model:entry.model,reference_tier:entry.context_tier,input_per_million:Number(entry.input_per_million),output_per_million:Number(entry.output_per_million),cache_read_per_million:entry.cache_read_per_million==null ? null:Number(entry.cache_read_per_million),cache_write_per_million:entry.cache_write_per_million==null ? null:Number(entry.cache_write_per_million),source:catalog.source,source_url:catalog.source_url,version:catalog.version,effective_at:effectiveAt};
}
/** No persistent AI cache: data is bound to the mounted admin session. */
export async function aiRequest<T>(resource:"overview"|"pricing"|"settings",query="",signal?:AbortSignal,body?:unknown):Promise<T> {
 if(isAdminLogoutPending()) throw new DOMException("Admin logout pending","AbortError");
 const url=`/api/admin/ai/${resource}${query ? `?${query}`:""}`;
 const init:RequestInit={method:body===undefined ? "GET":"POST",credentials:"same-origin",cache:"no-store",signal,...(body===undefined ? {}:{headers:{"Content-Type":"application/json"},body:JSON.stringify(body)})};
 let response=await adminNodeRequest(url,init);
 if(response.status===429 && body===undefined && !signal?.aborted) {
  const seconds=Math.min(10,Math.max(1,Number(response.headers.get("Retry-After"))||1));
  await new Promise<void>((resolve,reject)=>{
   if(signal?.aborted){reject(new DOMException("Request aborted","AbortError"));return;}
   const abort=()=>{clearTimeout(timer);reject(new DOMException("Request aborted","AbortError"));};
   const timer=setTimeout(()=>{signal?.removeEventListener("abort",abort);resolve();},seconds*1000);
   signal?.addEventListener("abort",abort,{once:true});
  });
  response=await adminNodeRequest(url,init);
 }
 if(!response.ok) throw new Error(`AI monitoring ${resource} unavailable (HTTP ${response.status})`);
 return await response.json() as T;
}

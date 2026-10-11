import { adminNodeRequest } from "./adminNodeRequest.ts";
import { isAdminLogoutPending } from "./adminRevocation.ts";
export type AISettings = { timezone:string; retention_days:number; detailed_retention_hours:number; custom_models?:string[] };
/** No persistent AI cache: data is bound to the mounted admin session. */
export async function aiRequest<T>(resource:"overview"|"settings",query="",signal?:AbortSignal,body?:unknown):Promise<T> {
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

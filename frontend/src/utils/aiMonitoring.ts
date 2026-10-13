export function aiTimestamp(value: number | string | undefined, timezone: string) {
 void timezone;
 if (value == null) return "N/A";
 const date = new Date(typeof value === "number" ? value * 1000 : value);
 if (!Number.isFinite(date.getTime())) return "N/A";
 try {
  const format=(timeZone:string)=>new Intl.DateTimeFormat("en-GB",{timeZone,day:"2-digit",month:"short",year:"numeric",hour:"2-digit",minute:"2-digit",hourCycle:"h23"}).format(date);
  return `UTC ${format("UTC")} · Beijing ${format("Asia/Shanghai")}`;
 } catch { return "N/A"; }
}

export type Usage = {
  model?: string; provider?: string; billing_mode?: string; task?: string;
  calls?: number; input_tokens?: number; output_tokens?: number; cache_read_tokens?: number;
  cache_write_tokens?: number; reasoning_tokens?: number; total_tokens?: number;
  api_equivalent_cost_usd?: number | null; unpriced_calls?: number; average_tps?: number | null; output_tps?: number | null; cost_share?: number | null;
};
export function aiNumber(value: number | null | undefined, digits = 2): string {
  return typeof value === "number" && Number.isFinite(value) ? value.toLocaleString("en-US", { maximumFractionDigits: digits }) : "N/A";
}
export function aiCompactNumber(value: number | null | undefined): string {
 if (typeof value !== "number" || !Number.isFinite(value)) return "N/A";
 const absolute=Math.abs(value);
 const [divisor,suffix]=absolute>=1e9?[1e9,"B"]:absolute>=1e6?[1e6,"M"]:absolute>=1e3?[1e3,"K"]:[1,""];
 const scaled=value/divisor;
 const digits=suffix===""?0:suffix==="K"?(Math.abs(scaled)>=100?1:2):(Math.abs(scaled)>=100?0:2);
 return `${Number(scaled.toFixed(digits))}${suffix}`;
}
export function aiUSD(value: number | null | undefined): string {
 return typeof value === "number" && Number.isFinite(value) ? `$${aiNumber(value,2)}` : "Unpriced";
}
export function canonicalTokens(row: Usage): number {
  return (row.input_tokens ?? 0) + (row.cache_read_tokens ?? 0) + (row.cache_write_tokens ?? 0) + (row.output_tokens ?? 0);
}
export function taskLabel(task?: string): string { return !task?.trim() || task === "unknown" ? "Unknown task" : task; }
export function aiRange(period: string, now = Math.floor(Date.now() / 1000)) {
  const seconds: Record<string, number> = { "1H": 3600, "24H": 86400, "7D": 604800, "30D": 2592000 };
  return { from: now - (seconds[period] ?? seconds["24H"]), to: now };
}
export function sortUsage<T extends Usage>(rows: T[], key: keyof Usage, ascending: boolean): T[] {
  return [...rows].sort((a, b) => {
    const av = a[key], bv = b[key];
    if (av == null) return bv == null ? 0 : 1;
    if (bv == null) return -1;
    const result = typeof av === "number" && typeof bv === "number" ? av - bv : String(av).localeCompare(String(bv));
    return ascending ? result : -result;
  });
}
export function aiISOSeconds(value: string): number {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) || !Number.isFinite(Date.parse(value))) throw new Error("Use a valid ISO timestamp with an explicit timezone offset.");
  return Date.parse(value) / 1000;
}
export function aiCustomRange(from: string, to: string) {
  const start = aiISOSeconds(from), end = aiISOSeconds(to);
  if (start >= end || end - start > 2592000) throw new Error("Choose an increasing custom range of at most 30 days.");
  return { from: start, to: end };
}
export type AITrendMetric = "total_tokens" | "calls";
export function aiTrendUnit(metric: AITrendMetric): string { return metric === "total_tokens" ? "Tokens" : "Requests"; }
export function aiTrendTick(value: number | null | undefined, metric: AITrendMetric): string {
 void metric;
 if (typeof value !== "number" || !Number.isFinite(value)) return "N/A";
 const absolute = Math.abs(value);
 const [divisor,suffix] = absolute >= 1e9 ? [1e9,"B"] : absolute >= 1e6 ? [1e6,"M"] : absolute >= 1e3 ? [1e3,"K"] : [1,""];
 const scaled=value/divisor, digits=Math.abs(scaled)>=100?0:Math.abs(scaled)>=10?1:2;
 return Number(scaled.toFixed(digits)).toString()+suffix;
}
export function aiTrendTime(value: number, timezone: string, spanSeconds: number): string {
 const options:Intl.DateTimeFormatOptions=spanSeconds<=36*3600?{hour:"2-digit",minute:"2-digit",hourCycle:"h23"}:spanSeconds<=14*86400?{day:"2-digit",month:"short"}:{day:"2-digit",month:"short",year:"2-digit"};
 try{return new Intl.DateTimeFormat("en-GB",{...options,timeZone:timezone}).format(new Date(value*1000));}catch{return new Intl.DateTimeFormat("en-GB",{...options,timeZone:"UTC"}).format(new Date(value*1000));}
}
export function hermesProfileEnabled(profile: { enable_hermes_ai?: boolean }): boolean {
  return profile.enable_hermes_ai === true;
}
export function hermesInstallArgs(platform: string, enabled: boolean): string[] {
  if (enabled && platform === "windows") throw new Error("Hermes AI monitoring is unsupported on Windows");
  return enabled ? ["--enable-hermes-ai"] : [];
}
export function encodeAISelectValue(value: string): string { return value === "" ? "__ai_all__" : `value:${value}`; }
export function decodeAISelectValue(value: string): string { return value === "__ai_all__" ? "" : value.startsWith("value:") ? value.slice("value:".length) : ""; }

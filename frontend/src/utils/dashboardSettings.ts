export const DASHBOARD_MODULE_IDS = [
  "server_status",
  "traffic_summary",
  "storage_summary",
  "resource_ranking",
  "daily_traffic_ranking",
  "latency_ranking",
  "latency_jitter_ranking",
  "packet_loss_ranking",
  "latency_trend",
  "traffic_trend",
  "billing_trend",
  "alerts",
  "storage_detail",
] as const;

export type DashboardModuleId = (typeof DASHBOARD_MODULE_IDS)[number];
export type DashboardModuleSpan = 2 | 3 | 6;
export type DashboardPresetId =
  | "overview"
  | "network"
  | "resources"
  | "traffic"
  | "operations"
  | "lite"
  | "custom";

export interface DashboardModuleSetting {
  id: DashboardModuleId;
  enabled: boolean;
  span: DashboardModuleSpan;
}

export interface DashboardSettings {
  preset: DashboardPresetId;
  modules: DashboardModuleSetting[];
  refresh_seconds: 15 | 30 | 60 | 120;
  chart_refresh_seconds: 15 | 30 | 60 | 120;
  ranking_limit: 5 | 10 | 15 | 20;
}

interface DashboardPresetDefinition {
  id: Exclude<DashboardPresetId, "custom">;
  enabled: readonly DashboardModuleId[];
  refresh_seconds: DashboardSettings["refresh_seconds"];
  chart_refresh_seconds: DashboardSettings["chart_refresh_seconds"];
  ranking_limit: DashboardSettings["ranking_limit"];
}

export const FORMAL_DASHBOARD_MODULES: readonly DashboardModuleId[] = [
  "server_status",
  "traffic_summary",
  "storage_summary",
  "latency_trend",
  "traffic_trend",
  "billing_trend",
  "alerts",
];

export const DASHBOARD_PRESETS: readonly DashboardPresetDefinition[] = [
  {
    id: "overview",
    enabled: FORMAL_DASHBOARD_MODULES,
    refresh_seconds: 30,
    chart_refresh_seconds: 30,
    ranking_limit: 5,
  },
  {
    id: "network",
    enabled: [
      "server_status",
      "traffic_summary",
      "storage_summary",
      "latency_trend",
      "daily_traffic_ranking",
      "latency_ranking",
      "latency_jitter_ranking",
      "packet_loss_ranking",
      "traffic_trend",
      "billing_trend",
      "alerts",
    ],
    refresh_seconds: 30,
    chart_refresh_seconds: 60,
    ranking_limit: 5,
  },
  {
    id: "resources",
    enabled: [
      "server_status",
      "storage_summary",
      "resource_ranking",
      "alerts",
      "storage_detail",
    ],
    refresh_seconds: 30,
    chart_refresh_seconds: 120,
    ranking_limit: 5,
  },
  {
    id: "traffic",
    enabled: [
      "server_status",
      "traffic_summary",
      "storage_summary",
      "daily_traffic_ranking",
      "alerts",
      "traffic_trend",
      "billing_trend",
    ],
    refresh_seconds: 60,
    chart_refresh_seconds: 120,
    ranking_limit: 5,
  },
  {
    id: "operations",
    enabled: [
      "server_status",
      "traffic_summary",
      "storage_summary",
      "alerts",
      "resource_ranking",
      "storage_detail",
      "latency_ranking",
      "latency_jitter_ranking",
      "packet_loss_ranking",
    ],
    refresh_seconds: 30,
    chart_refresh_seconds: 120,
    ranking_limit: 5,
  },
  {
    id: "lite",
    enabled: [
      "server_status",
      "storage_summary",
      "resource_ranking",
      "alerts",
      "storage_detail",
    ],
    refresh_seconds: 60,
    chart_refresh_seconds: 120,
    ranking_limit: 5,
  },
] as const;

const moduleIdSet = new Set<string>(DASHBOARD_MODULE_IDS);
const presetIdSet = new Set<string>([
  ...DASHBOARD_PRESETS.map((preset) => preset.id),
  "custom",
]);

function buildPresetModules(preset: DashboardPresetDefinition): DashboardModuleSetting[] {
  const enabled = new Set<DashboardModuleId>(preset.enabled);
  const ordered = [
    ...preset.enabled,
    ...DASHBOARD_MODULE_IDS.filter((id) => !enabled.has(id)),
  ];
  return ordered.map((id) => ({ id, enabled: enabled.has(id), span: DASHBOARD_BASE_SPANS[id] }));
}

export function dashboardSettingsForPreset(
  presetId: Exclude<DashboardPresetId, "custom">,
): DashboardSettings {
  const preset = DASHBOARD_PRESETS.find((item) => item.id === presetId)
    ?? DASHBOARD_PRESETS[0];
  return {
    preset: preset.id,
    modules: buildPresetModules(preset),
    refresh_seconds: preset.refresh_seconds,
    chart_refresh_seconds: preset.chart_refresh_seconds,
    ranking_limit: preset.ranking_limit,
  };
}

const DASHBOARD_BASE_SPANS: Record<DashboardModuleId, DashboardModuleSpan> = {
  server_status: 2,
  traffic_summary: 2,
  storage_summary: 2,
  resource_ranking: 6,
  daily_traffic_ranking: 3,
  latency_ranking: 3,
  latency_jitter_ranking: 3,
  packet_loss_ranking: 3,
  latency_trend: 6,
  traffic_trend: 3,
  billing_trend: 3,
  alerts: 3,
  storage_detail: 3,
};

export const DEFAULT_DASHBOARD_SETTINGS = dashboardSettingsForPreset("overview");

export interface DashboardPackedModule {
  id: DashboardModuleId;
  span: number;
}

export function packDashboardModules(
  modules: readonly DashboardModuleId[],
  spans: Partial<Record<DashboardModuleId, DashboardModuleSpan>> = {},
  fillRows = true,
): DashboardPackedModule[] {
  const packed: DashboardPackedModule[] = [];
  let used = 0;

  for (const id of modules) {
    const span = spans[id] ?? DASHBOARD_BASE_SPANS[id];
    if (used > 0 && used + span > 6) {
      if (fillRows) packed[packed.length - 1].span += 6 - used;
      used = 0;
    }
    packed.push({ id, span });
    used += span;
    if (used === 6) used = 0;
  }

  if (fillRows && used > 0 && packed.length > 0) {
    packed[packed.length - 1].span += 6 - used;
  }
  return packed;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function sanitizeDashboardSettings(value: unknown): DashboardSettings {
  if (!isObject(value)) return dashboardSettingsForPreset("overview");

  if (!Array.isArray(value.modules) || value.modules.length === 0) {
    return dashboardSettingsForPreset("overview");
  }

  const seen = new Set<DashboardModuleId>();
  const modules: DashboardModuleSetting[] = [];
  for (const rawModule of value.modules) {
    if (!isObject(rawModule) || typeof rawModule.id !== "string") continue;
    if (!moduleIdSet.has(rawModule.id) || seen.has(rawModule.id as DashboardModuleId)) continue;
    const id = rawModule.id as DashboardModuleId;
    seen.add(id);
    const span = rawModule.span === 2 || rawModule.span === 3 || rawModule.span === 6
      ? rawModule.span
      : DASHBOARD_BASE_SPANS[id];
    modules.push({ id, enabled: rawModule.enabled === true, span });
  }
  for (const id of DASHBOARD_MODULE_IDS) {
    if (!seen.has(id)) modules.push({ id, enabled: false, span: DASHBOARD_BASE_SPANS[id] });
  }
  if (!modules.some((module) => module.enabled)) {
    modules[0] = { ...modules[0], enabled: true };
  }

  const preset = typeof value.preset === "string" && presetIdSet.has(value.preset)
    ? value.preset as DashboardPresetId
    : "custom";
  if (preset !== "custom") return dashboardSettingsForPreset(preset);
  const refresh = value.refresh_seconds === 15
    || value.refresh_seconds === 60
    || value.refresh_seconds === 120
    ? value.refresh_seconds
    : 30;
  const chartRefresh = value.chart_refresh_seconds === 15
    || value.chart_refresh_seconds === 30
    || value.chart_refresh_seconds === 60
    || value.chart_refresh_seconds === 120
    ? value.chart_refresh_seconds
    : value.chart_refresh_seconds === 300
      ? 120
      : 30;
  const rankingLimit = value.ranking_limit === 10
    || value.ranking_limit === 15
    || value.ranking_limit === 20
    ? value.ranking_limit
    : 5;

  return {
    preset,
    modules,
    refresh_seconds: refresh,
    chart_refresh_seconds: chartRefresh,
    ranking_limit: rankingLimit,
  };
}

export function enabledDashboardModules(settings: DashboardSettings): DashboardModuleId[] {
  return settings.modules.filter((module) => module.enabled).map((module) => module.id);
}

export function dashboardModuleSpans(settings: DashboardSettings): Partial<Record<DashboardModuleId, DashboardModuleSpan>> {
  return Object.fromEntries(settings.modules.map((module) => [module.id, module.span]));
}

export function dashboardSummarySections(settings: DashboardSettings): string[] {
  const enabled = new Set(enabledDashboardModules(settings));
  const sections = new Set<string>();
  if (enabled.has("server_status")) sections.add("servers");
  if (enabled.has("resource_ranking")) sections.add("resources");
  if (enabled.has("storage_summary") || enabled.has("storage_detail")) sections.add("storage");
  if (enabled.has("alerts") || enabled.has("latency_trend")) sections.add("alerts");
  return [...sections];
}

export function dashboardChartSections(settings: DashboardSettings): string[] {
  const enabled = new Set(enabledDashboardModules(settings));
  const sections: string[] = [];
  if (
    enabled.has("traffic_summary")
    || enabled.has("traffic_trend")
    || enabled.has("billing_trend")
    || enabled.has("daily_traffic_ranking")
  ) {
    sections.push("traffic");
  }
  if (enabled.has("latency_trend") || enabled.has("latency_ranking")) sections.push("latency");
  if (enabled.has("latency_jitter_ranking")) sections.push("latency_jitter");
  if (enabled.has("packet_loss_ranking")) sections.push("packet_loss");
  return sections;
}

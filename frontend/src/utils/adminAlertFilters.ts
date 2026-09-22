import type {
  DashboardAlertItemsResponse,
  DashboardAlertKind,
  DashboardAlertLatest,
} from "@/utils/dashboard";

export const serverAlertKinds = new Set<DashboardAlertKind>([
  "offline",
  "resource",
  "traffic",
  "billing",
]);

const ALERT_ITEMS_CACHE_TTL_MS = 30_000;
const alertItemsCache = new Map<string, {
  expiresAt: number;
  response: DashboardAlertItemsResponse;
}>();
const pendingAlertItems = new Map<string, Promise<DashboardAlertItemsResponse>>();

function alertItemsCacheKey(kind: DashboardAlertKind, accountKey: string): string {
  return `${accountKey}:${kind}`;
}

export function getDashboardAlertItemsSnapshot(
  kind: DashboardAlertKind,
  accountKey = "authenticated",
): DashboardAlertItemsResponse | null {
  const key = alertItemsCacheKey(kind, accountKey);
  const cached = alertItemsCache.get(key);
  if (!cached) return null;
  if (cached.expiresAt <= Date.now()) {
    alertItemsCache.delete(key);
    return null;
  }
  return cached.response;
}

export function dashboardAlertCategoryPath(kind: DashboardAlertKind): string {
  if (kind === "latency_loss") return "/admin/notification/ping-loss?state=active";
  return `/admin/servers?alert=${encodeURIComponent(kind)}`;
}

export function dashboardAlertDetailPath(
  kind: DashboardAlertKind,
  alert?: DashboardAlertLatest,
): string {
  if (!alert) return dashboardAlertCategoryPath(kind);
  if (kind === "latency_loss" && alert.node_uuid && alert.task_id) {
    const params = new URLSearchParams({ node: alert.node_uuid, task: String(alert.task_id) });
    return `/admin/notification/ping-loss?${params}`;
  }

  if (alert.node_uuid) {
    return `/admin/servers?node=${encodeURIComponent(alert.node_uuid)}`;
  }
  return dashboardAlertCategoryPath(kind);
}

export async function requestDashboardAlertItems(
  kind: DashboardAlertKind,
  signal?: AbortSignal,
  accountKey = "authenticated",
): Promise<DashboardAlertItemsResponse> {
  const cached = getDashboardAlertItemsSnapshot(kind, accountKey);
  if (cached) return cached;
  const params = new URLSearchParams({ kind });
  const response = await fetch(`/api/admin/dashboard/alerts?${params}`, {
    cache: "no-store",
    signal,
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.message || `HTTP ${response.status}`);
  }
  const data = await response.json() as DashboardAlertItemsResponse;
  const normalized = { ...data, items: Array.isArray(data.items) ? data.items : [] };
  alertItemsCache.set(alertItemsCacheKey(kind, accountKey), {
    expiresAt: Date.now() + ALERT_ITEMS_CACHE_TTL_MS,
    response: normalized,
  });
  return normalized;
}

export function prefetchDashboardAlertItems(
  kind: DashboardAlertKind,
  accountKey = "authenticated",
): Promise<DashboardAlertItemsResponse> {
  const cached = getDashboardAlertItemsSnapshot(kind, accountKey);
  if (cached) return Promise.resolve(cached);
  const key = alertItemsCacheKey(kind, accountKey);
  const pending = pendingAlertItems.get(key);
  if (pending) return pending;
  const request = requestDashboardAlertItems(kind, undefined, accountKey)
    .finally(() => {
      if (pendingAlertItems.get(key) === request) pendingAlertItems.delete(key);
    });
  pendingAlertItems.set(key, request);
  return request;
}

export function formatBillingAlertStatus(
  dueAt: string | undefined,
  locale: string,
  now = Date.now(),
): string {
  if (!dueAt) return "";
  const due = new Date(dueAt).getTime();
  if (!Number.isFinite(due)) return "";
  const days = Math.ceil(Math.abs(due - now) / 86_400_000);
  if (due < now) return locale.startsWith("zh") ? `已到期 ${days} 天` : `Expired ${days}d`;
  return locale.startsWith("zh") ? `${days} 天后到期` : `Due in ${days}d`;
}

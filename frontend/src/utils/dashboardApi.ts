import {
  type DashboardChartsData,
  type DashboardData,
} from "@/utils/dashboard";
import { DASHBOARD_SESSION_MAX_AGE_MS, readDashboardSession } from "@/utils/dashboardSession";
import { clearDashboardAlertItemsCache } from "@/utils/adminAlertFilters";
import { adminRequestInvalidated, getAdminRevocationGeneration, registerAdminCacheClearer, revokeAdminSession } from "@/utils/adminRevocation";

let dashboardSnapshot: { accountKey: string; key: string; data: DashboardData; savedAt: number } | null = null;
let pendingDashboardRequest: { accountKey: string; key: string; request: Promise<DashboardData> } | null = null;
let dashboardChartsSnapshot: { accountKey: string; key: string; data: DashboardChartsData; savedAt: number } | null = null;
let pendingDashboardChartsRequest: { accountKey: string; key: string; request: Promise<DashboardChartsData> } | null = null;
let cacheGeneration = 0;

function clearDashboardDataSnapshots(): void {
  cacheGeneration++;
  dashboardSnapshot = null;
  dashboardChartsSnapshot = null;
  pendingDashboardRequest = null;
  pendingDashboardChartsRequest = null;
}

export function clearDashboardSnapshots(): void {
  clearDashboardDataSnapshots();
  clearDashboardAlertItemsCache();
}

registerAdminCacheClearer(clearDashboardDataSnapshots);

function isFresh(savedAt: number): boolean {
  const age = Date.now() - savedAt;
  return age >= 0 && age <= DASHBOARD_SESSION_MAX_AGE_MS;
}

export async function requestDashboard(
  sections: string[],
  rankingLimit: number,
  accountKey = "authenticated",
): Promise<DashboardData> {
  const key = `${sections.join(",")}:${rankingLimit}`;
  if (pendingDashboardRequest?.accountKey === accountKey && pendingDashboardRequest.key === key) {
    return pendingDashboardRequest.request;
  }
  const params = new URLSearchParams({ sections: sections.join(","), limit: String(rankingLimit) });
  const generation = cacheGeneration;
  const revocationGeneration = getAdminRevocationGeneration();
  const request = fetch(`/api/admin/dashboard?${params}`, { cache: "no-store" })
    .then(async (response) => {
      adminRequestInvalidated(revocationGeneration);
      if (generation !== cacheGeneration) throw new DOMException("Dashboard request invalidated", "AbortError");
      if (response.status === 401) revokeAdminSession();
      if (!response.ok) {
        let message = `HTTP ${response.status}`;
        try {
          const payload = await response.json();
          if (payload?.message) message = String(payload.message);
        } catch {
          // Keep the HTTP status fallback.
        }
        throw Object.assign(new Error(message), { status: response.status });
      }
      const data = await response.json() as DashboardData;
      adminRequestInvalidated(revocationGeneration);
      return data;
    })
    .then((data) => {
      if (generation !== cacheGeneration) throw new DOMException("Dashboard request invalidated", "AbortError");
      dashboardSnapshot = { accountKey, key, data, savedAt: Date.now() };
      return data;
    })
    .finally(() => {
      if (pendingDashboardRequest?.request === request) {
        pendingDashboardRequest = null;
      }
    });
  pendingDashboardRequest = { accountKey, key, request };
  return request;
}

export async function requestDashboardCharts(
  sections: string[],
  rankingLimit: number,
  accountKey = "authenticated",
): Promise<DashboardChartsData> {
  const key = `${sections.join(",")}:${rankingLimit}`;
  if (pendingDashboardChartsRequest?.accountKey === accountKey && pendingDashboardChartsRequest.key === key) {
    return pendingDashboardChartsRequest.request;
  }
  const params = new URLSearchParams({ sections: sections.join(","), limit: String(rankingLimit) });
  const generation = cacheGeneration;
  const revocationGeneration = getAdminRevocationGeneration();
  const request = fetch(`/api/admin/dashboard/charts?${params}`, { cache: "no-store" })
    .then(async (response) => {
      adminRequestInvalidated(revocationGeneration);
      if (generation !== cacheGeneration) throw new DOMException("Dashboard request invalidated", "AbortError");
      if (response.status === 401) revokeAdminSession();
      if (!response.ok) {
        let message = `HTTP ${response.status}`;
        try {
          const payload = await response.json();
          if (payload?.message) message = String(payload.message);
        } catch {
          // Keep the HTTP status fallback.
        }
        throw Object.assign(new Error(message), { status: response.status });
      }
      const data = await response.json() as DashboardChartsData;
      adminRequestInvalidated(revocationGeneration);
      return data;
    })
    .then((data) => {
      if (generation !== cacheGeneration) throw new DOMException("Dashboard request invalidated", "AbortError");
      dashboardChartsSnapshot = { accountKey, key, data, savedAt: Date.now() };
      return data;
    })
    .finally(() => {
      if (pendingDashboardChartsRequest?.request === request) {
        pendingDashboardChartsRequest = null;
      }
    });
  pendingDashboardChartsRequest = { accountKey, key, request };
  return request;
}

export function getDashboardSnapshot(key: string, accountKey = "authenticated"): DashboardData | null {
  if (dashboardSnapshot && !isFresh(dashboardSnapshot.savedAt)) dashboardSnapshot = null;
  if (dashboardSnapshot?.accountKey === accountKey && dashboardSnapshot.key === key) {
    return dashboardSnapshot.data;
  }
  const data = readDashboardSession<DashboardData>("summary", accountKey, key);
  if (data) dashboardSnapshot = { accountKey, key, data, savedAt: Date.now() };
  return data;
}

export function getDashboardChartsSnapshot(key: string, accountKey = "authenticated"): DashboardChartsData | null {
  if (dashboardChartsSnapshot && !isFresh(dashboardChartsSnapshot.savedAt)) dashboardChartsSnapshot = null;
  if (dashboardChartsSnapshot?.accountKey === accountKey && dashboardChartsSnapshot.key === key) {
    return dashboardChartsSnapshot.data;
  }
  const data = readDashboardSession<DashboardChartsData>("charts", accountKey, key);
  if (data) dashboardChartsSnapshot = { accountKey, key, data, savedAt: Date.now() };
  return data;
}

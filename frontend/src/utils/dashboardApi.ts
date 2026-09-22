import {
  type DashboardChartsData,
  type DashboardData,
} from "@/utils/dashboard";
import { readDashboardSession, writeDashboardSession } from "@/utils/dashboardSession";

let dashboardSnapshot: { accountKey: string; key: string; data: DashboardData } | null = null;
let pendingDashboardRequest: { accountKey: string; key: string; request: Promise<DashboardData> } | null = null;
let dashboardChartsSnapshot: { accountKey: string; key: string; data: DashboardChartsData } | null = null;
let pendingDashboardChartsRequest: { accountKey: string; key: string; request: Promise<DashboardChartsData> } | null = null;

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
  const request = fetch(`/api/admin/dashboard?${params}`, { cache: "no-store" })
    .then(async (response) => {
      if (!response.ok) {
        let message = `HTTP ${response.status}`;
        try {
          const payload = await response.json();
          if (payload?.message) message = String(payload.message);
        } catch {
          // Keep the HTTP status fallback.
        }
        throw new Error(message);
      }
      return response.json() as Promise<DashboardData>;
    })
    .then((data) => {
      dashboardSnapshot = { accountKey, key, data };
      writeDashboardSession("summary", accountKey, key, data);
      return data;
    })
    .finally(() => {
      if (pendingDashboardRequest?.accountKey === accountKey && pendingDashboardRequest.key === key) {
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
  const request = fetch(`/api/admin/dashboard/charts?${params}`, { cache: "no-store" })
    .then(async (response) => {
      if (!response.ok) {
        let message = `HTTP ${response.status}`;
        try {
          const payload = await response.json();
          if (payload?.message) message = String(payload.message);
        } catch {
          // Keep the HTTP status fallback.
        }
        throw new Error(message);
      }
      return response.json() as Promise<DashboardChartsData>;
    })
    .then((data) => {
      dashboardChartsSnapshot = { accountKey, key, data };
      writeDashboardSession("charts", accountKey, key, data);
      return data;
    })
    .finally(() => {
      if (pendingDashboardChartsRequest?.accountKey === accountKey && pendingDashboardChartsRequest.key === key) {
        pendingDashboardChartsRequest = null;
      }
    });
  pendingDashboardChartsRequest = { accountKey, key, request };
  return request;
}

export function getDashboardSnapshot(key: string, accountKey = "authenticated"): DashboardData | null {
  if (dashboardSnapshot?.accountKey === accountKey && dashboardSnapshot.key === key) {
    return dashboardSnapshot.data;
  }
  const data = readDashboardSession<DashboardData>("summary", accountKey, key);
  if (data) dashboardSnapshot = { accountKey, key, data };
  return data;
}

export function getDashboardChartsSnapshot(key: string, accountKey = "authenticated"): DashboardChartsData | null {
  if (dashboardChartsSnapshot?.accountKey === accountKey && dashboardChartsSnapshot.key === key) {
    return dashboardChartsSnapshot.data;
  }
  const data = readDashboardSession<DashboardChartsData>("charts", accountKey, key);
  if (data) dashboardChartsSnapshot = { accountKey, key, data };
  return data;
}

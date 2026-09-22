import { fetchDashboardSettings } from "@/hooks/useDashboardSettings";
import {
  requestDashboard,
  requestDashboardCharts,
} from "@/utils/dashboardApi";
import {
  dashboardChartSections,
  dashboardSummarySections,
} from "@/utils/dashboardSettings";

export async function prefetchAdminDashboard(accountKey: string): Promise<void> {
  const settings = await fetchDashboardSettings({ accountKey });
  await Promise.all([
    requestDashboard(
      dashboardSummarySections(settings),
      settings.ranking_limit,
      accountKey,
    ),
    requestDashboardCharts(
      dashboardChartSections(settings),
      settings.ranking_limit,
      accountKey,
    ),
  ]);
}

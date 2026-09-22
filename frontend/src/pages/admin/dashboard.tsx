import React from "react";
import { Button, Callout, Skeleton } from "@radix-ui/themes";
import {
  AlertCircle,
  ArrowDownToLine,
  ArrowUpFromLine,
  Database,
  RefreshCw,
  Server,
  WalletCards,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import AdminPageTitle from "@/components/admin/AdminPageTitle";
import { useAccount } from "@/contexts/AccountContext";
import {
  AlertOverviewPanel,
  BillingTrendPanel,
  DailyTrafficRankingPanel,
  getDashboardChartsSnapshot,
  getDashboardSnapshot,
  LatencyPanel,
  LatencyJitterRankingPanel,
  LatencyRankingPanel,
  OverviewSkeleton,
  PacketLossRankingPanel,
  requestDashboard,
  requestDashboardCharts,
  ResourceRankingPanel,
  StoragePanel,
  SummaryPanel,
  TrafficTrendPanel,
} from "@/components/admin/DashboardPanels";
import { useDashboardSettings } from "@/hooks/useDashboardSettings";
import {
  dashboardLocalStorageTotal,
  dashboardRuntimeStorageTotal,
  dashboardTrafficAxisWidth,
  shortDashboardDay,
  type DashboardChartsData,
  type DashboardData,
} from "@/utils/dashboard";
import {
  dashboardChartSections,
  dashboardModuleSpans,
  dashboardSummarySections,
  enabledDashboardModules,
  packDashboardModules,
  type DashboardModuleId,
} from "@/utils/dashboardSettings";
import { formatBytes } from "@/utils/unitHelper";
import {
  readDashboardSession,
  writeDashboardSession,
  type DashboardViewState,
} from "@/utils/dashboardSession";

const moduleGridClass: Record<number, string> = {
  1: "md:col-span-1",
  2: "md:col-span-2",
  3: "md:col-span-3",
  4: "md:col-span-4",
  5: "md:col-span-5",
  6: "md:col-span-6",
};

export default function AdminDashboard() {
  const { t, i18n } = useTranslation();
  const { account } = useAccount();
  const accountKey = account?.uuid || account?.username || "authenticated";
  const {
    settings,
    loading: settingsLoading,
    error: settingsError,
    refetch: refetchSettings,
  } = useDashboardSettings(accountKey);

  const summarySections = React.useMemo(() => dashboardSummarySections(settings), [settings]);
  const chartSections = React.useMemo(() => dashboardChartSections(settings), [settings]);
  const summaryKey = `${summarySections.join(",")}:${settings.ranking_limit}`;
  const chartKey = `${chartSections.join(",")}:${settings.ranking_limit}`;
  const visibleModules = React.useMemo(() => enabledDashboardModules(settings), [settings]);
  const moduleSpans = React.useMemo(() => dashboardModuleSpans(settings), [settings]);
  const packedModules = React.useMemo(
    () => packDashboardModules(visibleModules, moduleSpans, settings.preset !== "custom"),
    [moduleSpans, settings.preset, visibleModules],
  );
  const viewKey = React.useMemo(
    () => settings.preset === "overview"
      ? "overview"
      : `${settings.preset}:${packedModules.map(({ id, span }) => `${id}:${span}`).join(",")}`,
    [packedModules, settings.preset],
  );
  const dashboardRootRef = React.useRef<HTMLDivElement>(null);
  const navigationAnchorRef = React.useRef<Omit<DashboardViewState, "scrollTop"> | null>(null);
  const restoredViewKeyRef = React.useRef<string | null>(null);

  const [data, setData] = React.useState<DashboardData | null>(() => getDashboardSnapshot(summaryKey, accountKey));
  const [charts, setCharts] = React.useState<DashboardChartsData | null>(() => getDashboardChartsSnapshot(chartKey, accountKey));
  const [loading, setLoading] = React.useState(() => getDashboardSnapshot(summaryKey, accountKey) === null);
  const [error, setError] = React.useState<string | null>(null);
  const [chartsError, setChartsError] = React.useState<string | null>(null);

  const loadSummary = React.useCallback(async (silent = false) => {
    if (summarySections.length === 0) {
      setData(null);
      if (!silent) setLoading(false);
      return;
    }
    if (!silent && !getDashboardSnapshot(summaryKey, accountKey)) setLoading(true);
    try {
      const next = await requestDashboard(summarySections, settings.ranking_limit, accountKey);
      setData(next);
      setError(null);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      if (!silent) setLoading(false);
    }
  }, [accountKey, settings.ranking_limit, summaryKey, summarySections]);

  const loadCharts = React.useCallback(async () => {
    if (chartSections.length === 0) {
      setCharts(null);
      setChartsError(null);
      return;
    }
    try {
      const next = await requestDashboardCharts(chartSections, settings.ranking_limit, accountKey);
      setCharts(next);
      setChartsError(null);
    } catch (reason) {
      setChartsError(reason instanceof Error ? reason.message : String(reason));
    }
  }, [accountKey, chartSections, settings.ranking_limit]);

  const refreshAll = React.useCallback(() => {
    void loadSummary(false);
    void loadCharts();
  }, [loadCharts, loadSummary]);

  React.useEffect(() => {
    const cachedSummary = getDashboardSnapshot(summaryKey, accountKey);
    const cachedCharts = getDashboardChartsSnapshot(chartKey, accountKey);
    setData(cachedSummary);
    setCharts(cachedCharts);
    setLoading(!cachedSummary);
    void loadSummary(Boolean(cachedSummary));
    void loadCharts();

    const refreshWhenVisible = (callback: () => void) => {
      if (document.visibilityState === "visible") callback();
    };
    const summaryInterval = summarySections.length > 0
      ? window.setInterval(
        () => refreshWhenVisible(() => void loadSummary(true)),
        settings.refresh_seconds * 1000,
      )
      : null;
    const chartInterval = chartSections.length > 0
      ? window.setInterval(
        () => refreshWhenVisible(() => void loadCharts()),
        settings.chart_refresh_seconds * 1000,
      )
      : null;
    return () => {
      if (summaryInterval !== null) window.clearInterval(summaryInterval);
      if (chartInterval !== null) window.clearInterval(chartInterval);
    };
  }, [
    chartKey,
    chartSections.length,
    loadCharts,
    loadSummary,
    settings.chart_refresh_seconds,
    settings.refresh_seconds,
    accountKey,
    summaryKey,
    summarySections.length,
  ]);

  const saveDashboardView = React.useCallback((anchor = navigationAnchorRef.current) => {
    const root = dashboardRootRef.current;
    const container = root?.closest<HTMLElement>("[data-admin-scroll-container]");
    if (!container) return;
    writeDashboardSession<DashboardViewState>("view", accountKey, viewKey, {
      scrollTop: container.scrollTop,
      ...(anchor ?? {}),
    });
  }, [accountKey, viewKey]);

  const rememberClickedModule = React.useCallback((event: React.MouseEvent<HTMLDivElement>) => {
    const target = event.target instanceof Element ? event.target : null;
    if (!target?.closest("a[href]")) return;
    const moduleElement = target.closest<HTMLElement>("[data-dashboard-module]");
    const container = dashboardRootRef.current?.closest<HTMLElement>("[data-admin-scroll-container]");
    if (!moduleElement || !container) return;
    const anchor = {
      moduleId: moduleElement.dataset.dashboardModule,
      moduleOffset: moduleElement.getBoundingClientRect().top - container.getBoundingClientRect().top,
    };
    navigationAnchorRef.current = anchor;
    saveDashboardView(anchor);
  }, [saveDashboardView]);

  React.useLayoutEffect(() => {
    if (settingsLoading || restoredViewKeyRef.current === viewKey) return;
    restoredViewKeyRef.current = viewKey;
    const snapshot = readDashboardSession<DashboardViewState>("view", accountKey, viewKey);
    const root = dashboardRootRef.current;
    const container = root?.closest<HTMLElement>("[data-admin-scroll-container]");
    if (!snapshot || !root || !container) return;

    let animationFrame = 0;
    let frameCount = 0;
    let stableFrames = 0;
    let previousHeight = -1;
    const restore = () => {
      frameCount++;
      const moduleElement = snapshot.moduleId
        ? root.querySelector<HTMLElement>(`[data-dashboard-module="${snapshot.moduleId}"]`)
        : null;
      if (moduleElement && Number.isFinite(snapshot.moduleOffset)) {
        const currentOffset = moduleElement.getBoundingClientRect().top - container.getBoundingClientRect().top;
        container.scrollTop = Math.max(0, container.scrollTop + currentOffset - (snapshot.moduleOffset ?? 0));
      } else {
        container.scrollTop = Math.max(0, snapshot.scrollTop);
      }

      if (container.scrollHeight === previousHeight) stableFrames++;
      else stableFrames = 0;
      previousHeight = container.scrollHeight;
      if (stableFrames < 2 && frameCount < 12) {
        animationFrame = window.requestAnimationFrame(restore);
      }
    };
    animationFrame = window.requestAnimationFrame(restore);
    return () => window.cancelAnimationFrame(animationFrame);
  }, [accountKey, settingsLoading, viewKey]);

  React.useEffect(() => {
    const saveBeforePageHide = () => saveDashboardView();
    const saveWhenHidden = () => {
      if (document.visibilityState === "hidden") saveDashboardView();
    };
    window.addEventListener("pagehide", saveBeforePageHide);
    document.addEventListener("visibilitychange", saveWhenHidden);
    return () => {
      saveDashboardView();
      window.removeEventListener("pagehide", saveBeforePageHide);
      document.removeEventListener("visibilitychange", saveWhenHidden);
    };
  }, [saveDashboardView]);

  const locale = i18n.resolvedLanguage || i18n.language || "zh-CN";
  const dailyChartData = React.useMemo(
    () => (charts?.traffic.daily ?? []).map((item) => ({
      ...item,
      label: shortDashboardDay(item.day, locale),
    })),
    [charts?.traffic.daily, locale],
  );
  const hourlyTrafficAxisWidth = React.useMemo(
    () => dashboardTrafficAxisWidth(
      (charts?.traffic.hourly ?? []).flatMap((item) => [item.up, item.down]),
    ),
    [charts?.traffic.hourly],
  );
  const dailyTrafficAxisWidth = React.useMemo(
    () => dashboardTrafficAxisWidth(
      (charts?.traffic.daily ?? []).map((item) => item.billable),
    ),
    [charts?.traffic.daily],
  );

  const renderModule = (module: DashboardModuleId): React.ReactNode => {
    switch (module) {
      case "server_status":
        return data ? (
          <SummaryPanel
            icon={<Server size={18} />}
            label={t("admin_dashboard.servers")}
            value={`${data.servers.online} / ${data.servers.total}`}
            tone={data.servers.offline > 0 ? "orange" : "green"}
          >
            <div className="flex items-center justify-between gap-3">
              <span>{t("admin_dashboard.online_count", { count: data.servers.online })}</span>
              <span className={data.servers.offline > 0 ? "text-[var(--orange-11)]" : "text-[var(--green-11)]"}>
                {t("admin_dashboard.offline_count", { count: data.servers.offline })}
              </span>
            </div>
          </SummaryPanel>
        ) : <Skeleton className="h-[112px] w-full" />;
      case "traffic_summary":
        return (
          <SummaryPanel
            icon={<WalletCards size={18} />}
            label={t("admin_dashboard.today_billable")}
            value={charts && !charts.traffic.error ? formatBytes(charts.traffic.today_billable) : "-"}
          >
            {charts && !charts.traffic.error ? (
              <div className="grid grid-cols-2 gap-3">
                <span className="flex min-w-0 items-center gap-1.5">
                  <ArrowUpFromLine size={14} className="shrink-0 text-[var(--accent-11)]" />
                  <span className="truncate">{t("admin_dashboard.upload")} {formatBytes(charts.traffic.today_up)}</span>
                </span>
                <span className="flex min-w-0 items-center justify-end gap-1.5 text-right">
                  <ArrowDownToLine size={14} className="shrink-0 text-[var(--orange-11)]" />
                  <span className="truncate">{t("admin_dashboard.download")} {formatBytes(charts.traffic.today_down)}</span>
                </span>
              </div>
            ) : <span>{chartsError ? t("admin_dashboard.data_unavailable") : t("admin_dashboard.chart_loading")}</span>}
          </SummaryPanel>
        );
      case "storage_summary":
        return data ? (
          <SummaryPanel
            icon={<Database size={18} />}
            label={t("admin_dashboard.database_usage")}
            value={dashboardLocalStorageTotal(data) === null
              ? t("admin_dashboard.external_storage")
              : formatBytes(dashboardLocalStorageTotal(data) ?? 0)}
          >
            <div className="flex items-center justify-between gap-3">
              <span>{t("admin_dashboard.database_files")} {formatBytes(data.storage.database_files)}</span>
              <span>WAL + SHM {formatBytes(dashboardRuntimeStorageTotal(data))}</span>
            </div>
          </SummaryPanel>
        ) : <Skeleton className="h-[112px] w-full" />;
      case "resource_ranking":
        return data
          ? <ResourceRankingPanel data={data} limit={settings.ranking_limit} />
          : <Skeleton className="h-[260px] w-full" />;
      case "daily_traffic_ranking":
        return (
          <DailyTrafficRankingPanel
            charts={charts}
            error={chartsError}
            limit={settings.ranking_limit}
          />
        );
      case "latency_ranking":
        return (
          <LatencyRankingPanel
            charts={charts}
            error={chartsError}
            limit={settings.ranking_limit}
          />
        );
      case "latency_jitter_ranking":
        return (
          <LatencyJitterRankingPanel
            charts={charts}
            error={chartsError}
            limit={settings.ranking_limit}
          />
        );
      case "packet_loss_ranking":
        return (
          <PacketLossRankingPanel
            charts={charts}
            error={chartsError}
            limit={settings.ranking_limit}
          />
        );
      case "latency_trend":
        return (
          <LatencyPanel
            charts={charts}
            locale={locale}
            requestError={chartsError}
            warningCount={data?.alerts?.latency_loss?.error ? 0 : data?.alerts?.latency_loss?.current ?? 0}
          />
        );
      case "traffic_trend":
        return <TrafficTrendPanel charts={charts} error={chartsError} axisWidth={hourlyTrafficAxisWidth} />;
      case "billing_trend":
        return (
          <BillingTrendPanel
            charts={charts}
            error={chartsError}
            data={dailyChartData}
            axisWidth={dailyTrafficAxisWidth}
          />
        );
      case "alerts":
        return data
          ? <AlertOverviewPanel data={data} locale={locale} accountKey={accountKey} />
          : <Skeleton className="h-[210px] w-full" />;
      case "storage_detail":
        return data
          ? <StoragePanel data={data} locale={locale} />
          : <Skeleton className="h-[190px] w-full" />;
      default:
        return null;
    }
  };

  const generatedAt = data?.generated_at || charts?.generated_at;
  const initialDataPending = summarySections.length > 0 && loading && !data;
  const formalLayout = (
    <>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        {(["server_status", "traffic_summary", "storage_summary"] as const).map((module) => (
          <div key={module} data-dashboard-module={module} className="min-w-0">{renderModule(module)}</div>
        ))}
      </div>
      <div data-dashboard-module="latency_trend" className="min-w-0">{renderModule("latency_trend")}</div>
      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        {(["traffic_trend", "billing_trend"] as const).map((module) => (
          <div key={module} data-dashboard-module={module} className="min-w-0 [&>*]:h-full">{renderModule(module)}</div>
        ))}
      </div>
      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        {(["alerts"] as const).map((module) => (
          <div
            key={module}
            data-dashboard-module={module}
            data-dashboard-span="3"
            className="min-w-0 [&>*]:h-full"
          >
            {renderModule(module)}
          </div>
        ))}
      </div>
    </>
  );

  return (
    <div
      ref={dashboardRootRef}
      data-admin-route-pending={initialDataPending ? "true" : undefined}
      onClickCapture={rememberClickedModule}
      className="flex flex-col gap-3 p-0 md:p-4"
    >
      <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
        <AdminPageTitle description={t("admin_dashboard.subtitle")}>
          {t("admin_dashboard.title")}
        </AdminPageTitle>
        {generatedAt ? (
          <Button style={{ marginRight: 0 }} variant="ghost" color="gray" size="1" onClick={refreshAll}>
            <RefreshCw size={14} />
            {t("admin_dashboard.updated_at", {
              time: new Intl.DateTimeFormat(locale, {
                hour: "2-digit",
                minute: "2-digit",
                second: "2-digit",
              }).format(new Date(generatedAt)),
            })}
          </Button>
        ) : null}
      </div>

      {settingsError || error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Icon><AlertCircle size={16} /></Callout.Icon>
          <Callout.Text className="flex flex-wrap items-center gap-2">
            <span>{t("admin_dashboard.load_failed")}: {settingsError?.message || error}</span>
            <Button size="1" variant="soft" onClick={() => {
              if (settingsError) void refetchSettings(true);
              refreshAll();
            }}>
              <RefreshCw size={14} />
              {t("common.retry")}
            </Button>
          </Callout.Text>
        </Callout.Root>
      ) : null}

      {loading && !data && chartSections.length === 0 ? (
        <OverviewSkeleton />
      ) : settings.preset === "overview" ? formalLayout : (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-6">
          {packedModules.map(({ id, span }) => (
            <div
              key={id}
              data-dashboard-module={id}
              data-dashboard-span={span}
              className={`min-w-0 [&>*]:h-full ${moduleGridClass[span]}`}
            >
              {renderModule(id)}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

import { Callout, Skeleton } from "@radix-ui/themes";
import {
  Activity,
  AlertCircle,
  ArrowRight,
  ArrowUpDown,
  BellRing,
  CheckCircle2,
  Clock3,
  Cpu,
  HardDrive,
  MemoryStick,
  Timer,
  WifiOff,
} from "lucide-react";
import React from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router-dom";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { ChartContainer } from "@/components/ui/chart";
import {
  type DashboardAlertKind,
  type DashboardAlertSummary,
  type DashboardChartsData,
  type DashboardData,
  type DashboardDatabaseStatus,
  type DashboardResourceRankItem,
} from "@/utils/dashboard";
import {
  dashboardAlertCategoryPath,
  dashboardAlertDetailPath,
  formatBillingAlertStatus,
  prefetchDashboardAlertItems,
  serverAlertKinds,
} from "@/utils/adminAlertFilters";
import { formatBytes } from "@/utils/unitHelper";

export {
  getDashboardChartsSnapshot,
  getDashboardSnapshot,
  requestDashboard,
  requestDashboardCharts,
} from "@/utils/dashboardApi";

export function OverviewSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="h-[112px] rounded-md border bg-[var(--color-panel-solid)] p-3">
          <Skeleton width="7rem" height="1rem" />
          <Skeleton className="mt-4" width="9rem" height="1.9rem" />
          <Skeleton className="mt-3" width="72%" height="0.85rem" />
        </div>
      ))}
    </div>
  );
}

export function SummaryPanel({
  icon,
  label,
  value,
  children,
  tone = "accent",
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  children: React.ReactNode;
  tone?: "accent" | "green" | "orange";
}) {
  const toneClass = {
    accent: "text-[var(--accent-11)]",
    green: "text-[var(--green-11)]",
    orange: "text-[var(--orange-11)]",
  }[tone];
  return (
    <section className="min-h-[112px] rounded-md border bg-[var(--color-panel-solid)] p-3">
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm font-medium text-muted-foreground">{label}</span>
        <span className={`flex size-7 items-center justify-center ${toneClass}`}>{icon}</span>
      </div>
      <div className="mt-2 text-2xl font-bold tabular-nums text-foreground">{value}</div>
      <div className="mt-2 text-sm text-muted-foreground">{children}</div>
    </section>
  );
}

function PanelHeader({
  title,
  description,
  trailing,
  responsive = false,
}: {
  title: string;
  description?: string;
  trailing?: React.ReactNode;
  responsive?: boolean;
}) {
  return (
    <div className={`mb-3 flex items-start justify-between gap-3 ${responsive ? "@max-[28rem]:flex-col @max-[28rem]:gap-2" : ""}`}>
      <div className="min-w-0">
        <h2 className="text-base font-semibold text-foreground">{title}</h2>
        {description ? <p className="mt-0.5 text-sm text-muted-foreground">{description}</p> : null}
      </div>
      {responsive && trailing ? <div className="@max-[28rem]:ml-auto">{trailing}</div> : trailing}
    </div>
  );
}

function DashboardRankingItemLink({ href, children }: { href?: string; children: React.ReactNode }) {
  const className = "block min-w-0 rounded px-1.5 py-0.5 text-inherit no-underline transition-colors hover:bg-[var(--gray-a2)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent-8)]";
  if (!href) return <div className={className}>{children}</div>;
  return (
    <a
      href={href}
      className={className}
    >
      {children}
    </a>
  );
}

function DashboardRankingGrid({ limit, children }: { limit: number; children: React.ReactNode }) {
  return (
    <div className="@container">
      <div className={limit >= 15
        ? "grid grid-cols-1 gap-y-2 @min-[34rem]:grid-cols-2 @min-[34rem]:gap-x-5"
        : "grid grid-cols-1 gap-y-2"}
      >
        {children}
      </div>
    </div>
  );
}

function DashboardRankingItem({
  index,
  name,
  value,
  valueClassName = "",
  progress,
  detail,
}: {
  index: number;
  name: string;
  value: React.ReactNode;
  valueClassName?: string;
  progress: React.ReactNode;
  detail?: React.ReactNode;
}) {
  return (
    <div className="grid min-h-[3.5rem] min-w-0 grid-rows-[1rem_0.375rem_1rem] gap-y-1.5">
      <div className="flex min-w-0 items-center justify-between gap-3 text-xs leading-4">
        <span className="min-w-0 truncate text-foreground">
          <span className="mr-1 text-muted-foreground">{index + 1}.</span>
          <span className="font-medium">{name}</span>
        </span>
        <strong className={`shrink-0 font-semibold tabular-nums ${valueClassName}`}>{value}</strong>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-[var(--gray-a4)]">
        {progress}
      </div>
      <div className="flex min-w-0 items-center justify-end text-[11px] leading-4 text-muted-foreground">
        {detail ?? <span aria-hidden="true">&nbsp;</span>}
      </div>
    </div>
  );
}

function DatabaseStatusLine({ status }: { status: DashboardDatabaseStatus }) {
  const { t } = useTranslation();
  if (!status.error) return null;
  return (
    <Callout.Root color="red" size="1" className="mt-3">
      <Callout.Icon>
        <AlertCircle size={15} />
      </Callout.Icon>
      <Callout.Text>
        {t("admin_dashboard.database_read_failed")}: {status.error}
      </Callout.Text>
    </Callout.Root>
  );
}

function relativeTime(value: string | null, locale: string, fallback: string): string {
  if (!value) return fallback;
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp)) return fallback;
  const seconds = Math.round((timestamp - Date.now()) / 1000);
  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  if (Math.abs(seconds) < 60) return formatter.format(seconds, "second");
  const minutes = Math.round(seconds / 60);
  if (Math.abs(minutes) < 60) return formatter.format(minutes, "minute");
  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return formatter.format(hours, "hour");
  return formatter.format(Math.round(hours / 24), "day");
}

export function AlertOverviewPanel({
  data,
  locale,
  accountKey = "authenticated",
}: {
  data: DashboardData;
  locale: string;
  accountKey?: string;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const prepareCategory = React.useCallback((kind: DashboardAlertKind) => {
    if (serverAlertKinds.has(kind)) {
      void prefetchDashboardAlertItems(kind, accountKey).catch(() => undefined);
    }
  }, [accountKey]);
  const openCategory = React.useCallback(async (
    event: React.MouseEvent<HTMLAnchorElement>,
    kind: DashboardAlertKind,
    to: string,
  ) => {
    if (
      !serverAlertKinds.has(kind)
      || event.button !== 0
      || event.metaKey
      || event.ctrlKey
      || event.shiftKey
      || event.altKey
    ) return;
    event.preventDefault();
    try {
      await prefetchDashboardAlertItems(kind, accountKey);
    } catch {
      // The destination page keeps its normal error handling if prefetch fails.
    }
    navigate(to);
  }, [accountKey, navigate]);
  const items: Array<{
    kind: DashboardAlertKind;
    label: string;
    summary: DashboardAlertSummary;
    marker: string;
  }> = [
    { kind: "offline", label: t("admin_dashboard.alert_offline"), summary: data.alerts.offline, marker: "bg-[var(--red-9)]" },
    { kind: "resource", label: t("admin_dashboard.alert_resource"), summary: data.alerts.resource, marker: "bg-[var(--orange-9)]" },
    { kind: "latency_loss", label: t("admin_dashboard.alert_latency_loss"), summary: data.alerts.latency_loss, marker: "bg-[var(--orange-9)]" },
    { kind: "traffic", label: t("admin_dashboard.alert_traffic"), summary: data.alerts.traffic, marker: "bg-[var(--accent-9)]" },
    { kind: "billing", label: t("admin_dashboard.alert_billing"), summary: data.alerts.billing, marker: "bg-[var(--gray-8)]" },
  ];
  const available = items.filter((item) => !item.summary.error);
  const current = available.reduce((total, item) => total + item.summary.current, 0);
  const affected = available.reduce((total, item) => total + item.summary.affected_nodes, 0);
  const recovered = available.reduce((total, item) => total + item.summary.recovered_today, 0);

  return (
    <section
      className="flex h-full min-w-0 flex-col rounded-md border bg-[var(--color-panel-solid)] p-3"
      style={{ containerType: "inline-size" }}
    >
      <PanelHeader
        title={t("admin_dashboard.alerts_overview")}
        description={t("admin_dashboard.alerts_overview_hint")}
        trailing={<BellRing size={18} className="mt-0.5 text-muted-foreground" />}
      />
      <div className="grid grid-cols-3 divide-x pb-3">
        {[
          [t("admin_dashboard.current_alerts"), current, "text-[var(--red-11)]"],
          [t("admin_dashboard.affected_nodes"), affected, "text-[var(--orange-11)]"],
          [t("admin_dashboard.recovered_today"), recovered, "text-[var(--green-11)]"],
        ].map(([label, value, color], index) => (
          <div key={String(label)} className={index === 0 ? "pr-3" : "px-3"}>
            <div className={`text-xl font-semibold leading-none tabular-nums ${color}`}>{value}</div>
            <div className="mt-1 text-[11px] text-muted-foreground">{label}</div>
          </div>
        ))}
      </div>
      <div className="dashboard-alert-grid grid flex-1 auto-rows-fr">
        {items.map((item) => {
          const latest = item.summary.latest_alert;
          const categoryTo = dashboardAlertCategoryPath(item.kind);
          const detailTo = dashboardAlertDetailPath(item.kind, latest);
          const detailSummary = item.kind === "billing"
            ? formatBillingAlertStatus(latest?.due_at, locale)
            : item.kind === "offline"
              ? item.label
              : item.kind === "latency_loss"
                ? latest?.task_name || item.label
                : latest?.title || item.label;
          return (
            <div
              key={item.kind}
              className="flex min-h-16 min-w-0 flex-col justify-start gap-1.5 border-t border-[var(--gray-a5)] px-1 py-3"
            >
              <Link
                to={categoryTo}
                onPointerEnter={() => prepareCategory(item.kind)}
                onFocus={() => prepareCategory(item.kind)}
                onTouchStart={() => prepareCategory(item.kind)}
                onClick={(event) => void openCategory(event, item.kind, categoryTo)}
                className="flex min-w-0 items-start justify-between gap-2 rounded-sm text-xs outline-none hover:text-[var(--accent-11)] focus-visible:ring-2 focus-visible:ring-[var(--accent-8)]"
              >
                <span className="flex min-w-0 items-start gap-1.5 font-medium">
                  <span className={`mt-1 size-1.5 shrink-0 rounded-full ${item.marker}`} />
                  <span className="break-words leading-4">{item.label}</span>
                </span>
                <strong className="shrink-0 font-semibold tabular-nums text-foreground">
                  {item.summary.error ? "-" : item.summary.current}
                </strong>
              </Link>
              {item.summary.error ? (
                <span className="break-words text-[11px] leading-4 text-[var(--red-11)]">
                  {t("admin_dashboard.not_available")}
                </span>
              ) : item.summary.current === 0 ? (
                <span className="flex items-center gap-1 text-[11px] leading-4 text-[var(--green-11)]">
                  <CheckCircle2 size={12} className="shrink-0" />
                  {t("admin_dashboard.alert_normal", "正常")}
                </span>
              ) : latest ? (
                <Link
                  to={detailTo}
                  className="min-w-0 break-words rounded-sm text-[11px] leading-4 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-[var(--accent-8)]"
                >
                  <span className="font-medium text-foreground">{latest.node_name || latest.task_name || item.label}</span>
                  {detailSummary ? <span> · {detailSummary}</span> : null}
                  {latest.occurred_at ? (
                    <span className="mt-0.5 flex items-center gap-1 text-[10px] text-muted-foreground">
                      <Clock3 size={10} />
                      {relativeTime(latest.occurred_at, locale, t("admin_dashboard.not_available"))}
                    </span>
                  ) : null}
                </Link>
              ) : (
                <Link
                  to={categoryTo}
                  onPointerEnter={() => prepareCategory(item.kind)}
                  onFocus={() => prepareCategory(item.kind)}
                  onTouchStart={() => prepareCategory(item.kind)}
                  onClick={(event) => void openCategory(event, item.kind, categoryTo)}
                  className="break-words text-[11px] leading-4 text-muted-foreground hover:text-foreground"
                >
                  {t("admin_dashboard.affected_nodes", { count: item.summary.affected_nodes })}
                </Link>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

export function LatencyPanel({
  charts,
  locale,
  requestError,
  warningCount,
}: {
  charts: DashboardChartsData | null;
  locale: string;
  requestError?: string | null;
  warningCount: number;
}) {
  const { t } = useTranslation();
  const points = React.useMemo(
    () => (charts?.latency.points ?? []).map((point) => ({
      ...point,
      label: new Intl.DateTimeFormat(locale, { hour: "2-digit", minute: "2-digit" }).format(new Date(point.time)),
    })),
    [charts?.latency.points, locale],
  );
  return (
    <section className="min-h-[148px] rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.latency_overview")}
        description={t("admin_dashboard.latency_overview_hint")}
        trailing={(
          <span className="mt-0.5 flex shrink-0 items-center gap-3 text-xs text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="size-2 rounded-sm bg-[var(--accent-9)]" />
              {t("admin_dashboard.average_latency")}
            </span>
          </span>
        )}
      />
      <div className="grid min-h-[78px] grid-cols-1 items-center gap-3 md:grid-cols-[17rem_minmax(0,1fr)]">
        <div className="grid grid-cols-3 divide-x">
          {[
            [charts ? `${charts.latency.average.toFixed(0)} ms` : "-", t("admin_dashboard.average_latency"), "text-foreground"],
            [charts?.latency.targets ?? "-", t("admin_dashboard.monitor_targets"), "text-[var(--accent-11)]"],
            [warningCount, t("admin_dashboard.packet_loss_alerts"), "text-[var(--orange-11)]"],
          ].map(([value, label, color], index) => (
            <div key={String(label)} className={index === 0 ? "pr-3" : "px-3"}>
              <div className={`text-xl font-semibold leading-none tabular-nums ${color}`}>{value}</div>
              <div className="mt-1.5 text-[11px] text-muted-foreground">{label}</div>
            </div>
          ))}
        </div>
        {requestError || charts?.latency.error ? (
          <div className="flex h-[78px] items-center justify-center text-xs text-[var(--red-11)]">
            {t("admin_dashboard.data_unavailable")}
          </div>
        ) : charts ? (
          <ChartContainer
            config={{ average: { label: t("admin_dashboard.average_latency"), color: "var(--accent-9)" } }}
            className="h-[78px] w-full aspect-auto"
          >
            <LineChart data={points} margin={{ top: 3, right: 3, left: 0, bottom: 0 }}>
              <CartesianGrid vertical={false} strokeDasharray="3 3" />
              <XAxis dataKey="label" tickLine={false} axisLine={false} minTickGap={44} />
              <YAxis hide domain={["auto", "auto"]} />
              <Tooltip
                content={({ active, payload, label }) => active && payload?.length ? (
                  <div className="rounded-md border bg-background px-3 py-2 text-xs shadow-sm">
                    <div className="text-muted-foreground">{label}</div>
                    <div className="mt-1 font-medium">{Number(payload[0]?.value ?? 0).toFixed(1)} ms</div>
                  </div>
                ) : null}
              />
              <Line type="monotone" dataKey="average" stroke="var(--color-average)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
            </LineChart>
          </ChartContainer>
        ) : (
          <Skeleton className="h-[78px] w-full" />
        )}
      </div>
    </section>
  );
}
export function StoragePanel({ data, locale }: { data: DashboardData; locale: string }) {
  const { t } = useTranslation();
  const storageTotal = data.storage.database_files + data.storage.wal + data.storage.shm;
  const storageParts = [
    {
      label: t("admin_dashboard.database_files"),
      value: data.storage.database_files,
      barClass: "bg-[var(--accent-9)]",
      labelClass: "font-medium text-foreground",
    },
    {
      label: "WAL",
      value: data.storage.wal,
      barClass: "bg-[var(--orange-9)]",
      labelClass: "text-muted-foreground",
    },
    {
      label: "SHM",
      value: data.storage.shm,
      barClass: "bg-[var(--gray-9)]",
      labelClass: "text-muted-foreground",
    },
  ];

  return (
    <section className="h-full rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.database_usage")}
        trailing={<span className="text-sm font-semibold tabular-nums text-foreground">{formatBytes(storageTotal)}</span>}
      />
      <div className="space-y-2 text-xs">
        <div className="space-y-2">
          {storageParts.map((part) => {
            const share = storageTotal > 0 ? (part.value / storageTotal) * 100 : 0;
            return (
              <div
                key={part.label}
                className="grid grid-cols-[minmax(4.5rem,auto)_minmax(2.5rem,1fr)_5.5rem] items-center gap-2"
              >
                <span className={part.labelClass}>{part.label}</span>
                <div className="h-1.5 overflow-hidden rounded-full bg-[var(--gray-a4)]">
                  <div
                    className={`h-full rounded-full ${part.barClass}`}
                    style={{ width: `${share}%` }}
                  />
                </div>
                <span className="text-right tabular-nums text-muted-foreground">
                  {formatBytes(part.value)}
                </span>
              </div>
            );
          })}
        </div>
        <div className="grid grid-cols-2 gap-3 border-t pt-2">
          <div className="min-w-0">
            <span>{t("admin_dashboard.retention_period")}</span>
            <div className="mt-0.5 truncate tabular-nums text-muted-foreground">
              {data.storage.retention_days > 0
                ? t("admin_dashboard.days", { count: data.storage.retention_days })
                : t("admin_dashboard.not_available")}
            </div>
          </div>
          <div className="min-w-0 text-right">
            <span>{t("admin_dashboard.last_compaction")}</span>
            <div className="mt-0.5 truncate tabular-nums text-muted-foreground">
              {relativeTime(data.storage.last_compacted_at, locale, t("admin_dashboard.not_available"))}
            </div>
          </div>
        </div>
      </div>
      <DatabaseStatusLine status={data.database.main} />
      <DatabaseStatusLine status={data.database.monitoring} />
    </section>
  );
}

export function ResourceRankingPanel({ data, limit }: { data: DashboardData; limit: number }) {
  const { t } = useTranslation();
  const groups: Array<{
    key: "cpu" | "memory" | "disk";
    label: string;
    icon: React.ReactNode;
    items: DashboardResourceRankItem[];
    color: string;
  }> = [
    {
      key: "cpu",
      label: t("admin_dashboard.top_cpu"),
      icon: <Cpu size={16} />,
      items: data.resources?.cpu ?? [],
      color: "bg-[var(--accent-9)]",
    },
    {
      key: "memory",
      label: t("admin_dashboard.top_memory"),
      icon: <MemoryStick size={16} />,
      items: data.resources?.memory ?? [],
      color: "bg-[var(--green-9)]",
    },
    {
      key: "disk",
      label: t("admin_dashboard.top_disk"),
      icon: <HardDrive size={16} />,
      items: data.resources?.disk ?? [],
      color: "bg-[var(--orange-9)]",
    },
  ];

  return (
    <section className="rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.resource_ranking")}
        description={t("admin_dashboard.resource_ranking_hint")}
        trailing={(
          <span className="rounded-full bg-[var(--accent-a3)] px-2.5 py-1 text-xs font-medium text-[var(--accent-11)]">
            Top {limit}
          </span>
        )}
      />
      <div
        className="grid gap-5"
        style={{ gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 16rem), 1fr))" }}
      >
        {groups.map((group) => (
          <div key={group.key} className="min-w-0">
            <div className="mb-2 flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <span className="text-[var(--accent-11)]">{group.icon}</span>
              {group.label}
            </div>
            {group.items.length === 0 ? (
              <div className="flex min-h-28 items-center justify-center text-xs text-muted-foreground">
                {t("admin_dashboard.no_live_resource_data")}
              </div>
            ) : (
              <div className={limit >= 15 ? "space-y-2" : "space-y-2.5"}>
                {group.items.map((item, index) => {
                  const value = Math.max(0, Math.min(100, item[group.key]));
                  return (
                    <DashboardRankingItemLink key={item.uuid} href={item.detail_url}>
                      <div className="mb-1 flex items-center justify-between gap-2 text-xs">
                        <span className="min-w-0 truncate text-foreground">
                          <span className="mr-1 text-muted-foreground">{index + 1}.</span>
                          {item.name}
                        </span>
                        <strong className="shrink-0 font-semibold tabular-nums">{value.toFixed(1)}%</strong>
                      </div>
                      <div className="h-1.5 overflow-hidden rounded-full bg-[var(--gray-a4)]">
                        <div className={`h-full rounded-full ${group.color}`} style={{ width: `${value}%` }} />
                      </div>
                    </DashboardRankingItemLink>
                  );
                })}
              </div>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}

export function TrafficTrendPanel({
  charts,
  error,
  axisWidth,
}: {
  charts: DashboardChartsData | null;
  error: string | null;
  axisWidth: number;
}) {
  const { t } = useTranslation();
  return (
    <section className="@container flex h-full min-w-0 flex-col rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.today_traffic")}
        description={t("admin_dashboard.hourly_traffic_hint")}
        responsive
        trailing={(
          <div className="mt-0.5 flex shrink-0 items-center gap-3 text-xs text-muted-foreground">
            <span className="flex items-center gap-1.5"><span className="size-2 rounded-sm bg-[var(--accent-9)]" />{t("admin_dashboard.upload")}</span>
            <span className="flex items-center gap-1.5"><span className="size-2 rounded-sm bg-[var(--orange-9)]" />{t("admin_dashboard.download")}</span>
          </div>
        )}
      />
      {error || charts?.traffic.error ? (
        <div className="flex min-h-[220px] flex-1 items-center justify-center text-sm text-[var(--red-11)]">
          {t("admin_dashboard.data_unavailable")}
        </div>
      ) : charts ? (
        <ChartContainer
          config={{
            up: { label: t("admin_dashboard.upload"), color: "var(--accent-9)" },
            down: { label: t("admin_dashboard.download"), color: "var(--orange-9)" },
          }}
          className="min-h-[220px] w-full flex-1 aspect-auto"
        >
          <LineChart data={charts.traffic.hourly} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
            <CartesianGrid vertical={false} strokeDasharray="3 3" />
            <XAxis dataKey="hour" tickLine={false} axisLine={false} minTickGap={28} />
            <YAxis tickLine={false} axisLine={false} width={axisWidth} tickFormatter={(value) => formatBytes(Number(value)).replace(" ", "")} />
            <Tooltip
              content={({ active, payload, label }) => active && payload?.length ? (
                <div className="rounded-md border bg-background px-3 py-2 text-xs shadow-sm">
                  <div className="mb-1.5 text-muted-foreground">{label}</div>
                  <div className="font-medium text-[var(--accent-11)]">{t("admin_dashboard.upload")}: {formatBytes(Number(payload[0]?.value ?? 0))}</div>
                  <div className="mt-1 font-medium text-[var(--orange-11)]">{t("admin_dashboard.download")}: {formatBytes(Number(payload[1]?.value ?? 0))}</div>
                </div>
              ) : null}
            />
            <Line type="monotone" dataKey="up" stroke="var(--color-up)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
            <Line type="monotone" dataKey="down" stroke="var(--color-down)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} isAnimationActive={false} />
          </LineChart>
        </ChartContainer>
      ) : <Skeleton className="min-h-[220px] w-full flex-1" />}
    </section>
  );
}

export function BillingTrendPanel({
  charts,
  error,
  data,
  axisWidth,
}: {
  charts: DashboardChartsData | null;
  error: string | null;
  data: Array<DashboardChartsData["traffic"]["daily"][number] & { label: string }>;
  axisWidth: number;
}) {
  const { t } = useTranslation();
  return (
    <section className="@container flex h-full min-w-0 flex-col rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.daily_billable")}
        description={t("admin_dashboard.daily_billable_hint")}
        responsive
        trailing={<span className="rounded-full bg-[var(--accent-a3)] px-2.5 py-1 text-xs font-medium text-[var(--accent-11)]">{t("admin_dashboard.recent_month")}</span>}
      />
      {charts && !charts.traffic.error && !charts.traffic.history_ready ? (
        <p className="mb-2 text-xs text-muted-foreground">{t("admin_dashboard.history_preparing")}</p>
      ) : null}
      {error || charts?.traffic.error ? (
        <div className="flex min-h-[220px] flex-1 items-center justify-center text-sm text-[var(--red-11)]">
          {t("admin_dashboard.data_unavailable")}
        </div>
      ) : charts ? (
        <ChartContainer config={{ billable: { label: t("admin_dashboard.billable"), color: "var(--accent-9)" } }} className="min-h-[220px] w-full flex-1 aspect-auto">
          <BarChart data={data} margin={{ top: 8, right: 4, left: 0, bottom: 0 }}>
            <CartesianGrid vertical={false} strokeDasharray="3 3" />
            <XAxis dataKey="label" tickLine={false} axisLine={false} interval="preserveStartEnd" minTickGap={24} />
            <YAxis tickLine={false} axisLine={false} width={axisWidth} tickFormatter={(value) => formatBytes(Number(value)).replace(" ", "")} />
            <Tooltip
              cursor={{ fill: "var(--accent-a3)" }}
              content={({ active, payload, label }) => active && payload?.length ? (
                <div className="rounded-md border bg-background px-3 py-2 text-xs shadow-sm">
                  <div className="mb-1 text-muted-foreground">{label}</div>
                  <div className="font-medium">{t("admin_dashboard.billable")}: {formatBytes(Number(payload[0]?.value ?? 0))}</div>
                </div>
              ) : null}
            />
            <Bar dataKey="billable" fill="var(--color-billable)" radius={[2, 2, 0, 0]} maxBarSize={24} isAnimationActive={false} />
          </BarChart>
        </ChartContainer>
      ) : <Skeleton className="min-h-[220px] w-full flex-1" />}
    </section>
  );
}

export function DailyTrafficRankingPanel({
  charts,
  error,
  limit,
}: {
  charts: DashboardChartsData | null;
  error: string | null;
  limit: number;
}) {
  const { t } = useTranslation();
  const items = charts?.traffic.ranking ?? [];
  const maximum = items[0]?.billable ?? 0;
  return (
    <section className="h-full min-w-0 rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.daily_traffic_ranking")}
        description={t("admin_dashboard.daily_traffic_ranking_hint")}
        trailing={(
          <span className="flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--accent-a3)] px-2.5 py-1 text-xs font-medium text-[var(--accent-11)]">
            <ArrowUpDown size={13} /> Top {limit}
          </span>
        )}
      />
      {error || charts?.traffic.error ? (
        <div className="flex min-h-44 items-center justify-center text-sm text-[var(--red-11)]">
          {t("admin_dashboard.data_unavailable")}
        </div>
      ) : !charts ? (
        <Skeleton className="h-[220px] w-full" />
      ) : items.length === 0 ? (
        <div className="flex min-h-44 items-center justify-center text-sm text-muted-foreground">
          {t("admin_dashboard.no_daily_traffic_data")}
        </div>
      ) : (
        <DashboardRankingGrid limit={limit}>
          {items.map((item, index) => (
            <DashboardRankingItemLink key={item.uuid} href={item.detail_url}>
              <DashboardRankingItem
                index={index}
                name={item.name}
                value={formatBytes(item.billable)}
                progress={(
                  <div
                    className="flex h-full overflow-hidden rounded-full"
                    style={{ width: `${maximum > 0 ? (item.billable / maximum) * 100 : 0}%` }}
                  >
                    <span
                      className="h-full bg-[var(--orange-9)]"
                      style={{ width: `${item.up + item.down > 0 ? (item.up / (item.up + item.down)) * 100 : 0}%` }}
                    />
                    <span
                      className="h-full bg-[var(--blue-9)]"
                      style={{ width: `${item.up + item.down > 0 ? (item.down / (item.up + item.down)) * 100 : 0}%` }}
                    />
                  </div>
                )}
                detail={(
                  <div className="flex min-w-0 flex-wrap items-center justify-end gap-y-1">
                    <span className="inline-flex items-center gap-1.5 pr-2">
                      <span className="size-1.5 rounded-full bg-[var(--orange-9)]" />
                      {t("admin_dashboard.upload")} {formatBytes(item.up)}
                    </span>
                    <span className="inline-flex items-center gap-1.5 border-l border-[var(--gray-a6)] pl-2">
                      <span className="size-1.5 rounded-full bg-[var(--blue-9)]" />
                      {t("admin_dashboard.download")} {formatBytes(item.down)}
                    </span>
                  </div>
                )}
              />
            </DashboardRankingItemLink>
          ))}
        </DashboardRankingGrid>
      )}
    </section>
  );
}

export function LatencyRankingPanel({
  charts,
  error,
  limit,
}: {
  charts: DashboardChartsData | null;
  error: string | null;
  limit: number;
}) {
  const { t } = useTranslation();
  const items = charts?.latency.ranking ?? [];
  const maximum = items[0]?.average ?? 0;
  return (
    <section className="h-full min-w-0 rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.latency_ranking")}
        description={t("admin_dashboard.latency_ranking_hint")}
        trailing={(
          <span className="flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--orange-a3)] px-2.5 py-1 text-xs font-medium text-[var(--orange-11)]">
            <Timer size={13} /> Top {limit}
          </span>
        )}
      />
      {error || charts?.latency.error ? (
        <div className="flex min-h-44 items-center justify-center text-sm text-[var(--red-11)]">
          {t("admin_dashboard.data_unavailable")}
        </div>
      ) : !charts ? (
        <Skeleton className="h-[220px] w-full" />
      ) : items.length === 0 ? (
        <div className="flex min-h-44 items-center justify-center text-sm text-muted-foreground">
          {t("admin_dashboard.no_latency_ranking_data")}
        </div>
      ) : (
        <DashboardRankingGrid limit={limit}>
          {items.map((item, index) => (
            <DashboardRankingItemLink key={item.uuid} href={item.detail_url}>
              <DashboardRankingItem
                index={index}
                name={item.name}
                value={`${item.average.toFixed(1)} ms`}
                progress={(
                <div
                  className="h-full rounded-full bg-[var(--orange-9)]"
                  style={{ width: `${maximum > 0 ? (item.average / maximum) * 100 : 0}%` }}
                />
                )}
              />
            </DashboardRankingItemLink>
          ))}
        </DashboardRankingGrid>
      )}
    </section>
  );
}

export function LatencyJitterRankingPanel({
  charts,
  error,
  limit,
}: {
  charts: DashboardChartsData | null;
  error: string | null;
  limit: number;
}) {
  const { t } = useTranslation();
  const items = charts?.latency.jitter_ranking ?? [];
  const maximum = Math.max(0, ...items.map((item) => Math.abs(item.delta)));
  return (
    <section className="h-full min-w-0 rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.latency_jitter_ranking")}
        description={t("admin_dashboard.latency_jitter_ranking_hint")}
        trailing={(
          <span className="flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--orange-a3)] px-2.5 py-1 text-xs font-medium text-[var(--orange-11)]">
            <Activity size={13} /> Top {limit}
          </span>
        )}
      />
      {error || charts?.latency.jitter_error ? (
        <div className="flex min-h-44 items-center justify-center text-sm text-[var(--red-11)]">
          {t("admin_dashboard.data_unavailable")}
        </div>
      ) : !charts ? (
        <Skeleton className="h-[220px] w-full" />
      ) : items.length === 0 ? (
        <div className="flex min-h-44 items-center justify-center text-sm text-muted-foreground">
          {t("admin_dashboard.no_latency_jitter_data")}
        </div>
      ) : (
        <DashboardRankingGrid limit={limit}>
          {items.map((item, index) => {
            const increased = item.delta > 0;
            return (
              <DashboardRankingItemLink
                key={item.uuid}
                href={item.detail_url}
              >
                <DashboardRankingItem
                  index={index}
                  name={item.name}
                  value={`${item.delta > 0 ? "+" : ""}${item.delta.toFixed(1)} ms`}
                  valueClassName={increased ? "text-[var(--orange-11)]" : item.delta < 0 ? "text-[var(--green-11)]" : ""}
                  progress={(
                    <div
                      className={`h-full rounded-full ${increased ? "bg-[var(--orange-9)]" : "bg-[var(--green-9)]"}`}
                      style={{ width: `${maximum > 0 ? (Math.abs(item.delta) / maximum) * 100 : 0}%` }}
                    />
                  )}
                  detail={(
                    <div className="flex items-center justify-end gap-1 tabular-nums">
                      <span>{t("admin_dashboard.previous_minute")} {item.previous.toFixed(1)} ms</span>
                      <ArrowRight size={11} aria-hidden="true" />
                      <span>{t("admin_dashboard.current_minute")} {item.current.toFixed(1)} ms</span>
                    </div>
                  )}
                />
              </DashboardRankingItemLink>
            );
          })}
        </DashboardRankingGrid>
      )}
    </section>
  );
}

export function PacketLossRankingPanel({
  charts,
  error,
  limit,
}: {
  charts: DashboardChartsData | null;
  error: string | null;
  limit: number;
}) {
  const { t } = useTranslation();
  const items = charts?.packet_loss?.ranking ?? [];
  const maximum = items[0]?.loss_rate ?? 0;
  return (
    <section className="h-full min-w-0 rounded-md border bg-[var(--color-panel-solid)] p-3">
      <PanelHeader
        title={t("admin_dashboard.packet_loss_ranking")}
        description={t("admin_dashboard.packet_loss_ranking_hint")}
        trailing={(
          <span className="flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--red-a3)] px-2.5 py-1 text-xs font-medium text-[var(--red-11)]">
            <WifiOff size={13} /> Top {limit}
          </span>
        )}
      />
      {error || charts?.packet_loss?.error ? (
        <div className="flex min-h-44 items-center justify-center text-sm text-[var(--red-11)]">
          {t("admin_dashboard.data_unavailable")}
        </div>
      ) : !charts ? (
        <Skeleton className="h-[220px] w-full" />
      ) : items.length === 0 ? (
        <div className="flex min-h-44 items-center justify-center gap-2 text-sm font-medium text-[var(--green-11)]">
          <CheckCircle2 className="shrink-0" size={18} aria-hidden="true" />
          {t("admin_dashboard.packet_loss_all_normal")}
        </div>
      ) : (
        <DashboardRankingGrid limit={limit}>
          {items.map((item, index) => {
            const tone = item.loss_rate >= 50
              ? "bg-[var(--red-9)]"
              : item.loss_rate >= 10
                ? "bg-[var(--orange-9)]"
                : "bg-[var(--accent-9)]";
            const textTone = item.loss_rate >= 50
              ? "text-[var(--red-11)]"
              : item.loss_rate >= 10
                ? "text-[var(--orange-11)]"
                : "text-[var(--accent-11)]";
            return (
              <DashboardRankingItemLink key={`${item.uuid}:${item.task_id}`} href={item.detail_url}>
                <DashboardRankingItem
                  index={index}
                  name={item.name}
                  value={`${item.loss_rate.toFixed(1)}%`}
                  valueClassName={textTone}
                  progress={(
                    <div
                      className={`h-full rounded-full ${tone}`}
                      style={{ width: `${maximum > 0 ? (item.loss_rate / maximum) * 100 : 0}%` }}
                    />
                  )}
                  detail={(
                    <div className="flex min-w-0 flex-1 items-center justify-between gap-3">
                      <span className="min-w-0 truncate">{item.task_name}</span>
                      <span className="shrink-0 tabular-nums">
                        {t("admin_dashboard.packet_loss_samples", { lost: item.lost, total: item.total })}
                      </span>
                    </div>
                  )}
                />
              </DashboardRankingItemLink>
            );
          })}
        </DashboardRankingGrid>
      )}
    </section>
  );
}

/* Legacy composition retained only while the customizable page is developed locally.
function LegacyAdminDashboard() {
  const { t, i18n } = useTranslation();
  const isMobile = useIsMobile();
  const [data, setData] = React.useState<DashboardData | null>(dashboardSnapshot);
  const [charts, setCharts] = React.useState<DashboardChartsData | null>(dashboardChartsSnapshot);
  const [loading, setLoading] = React.useState(!dashboardSnapshot);
  const [error, setError] = React.useState<string | null>(null);
  const [chartsError, setChartsError] = React.useState<string | null>(null);

  const loadSummary = React.useCallback(async (silent = false) => {
    if (!silent && !dashboardSnapshot) setLoading(true);
    try {
      const next = await requestDashboard();
      setData(next);
      setError(null);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);

  const loadCharts = React.useCallback(async () => {
    try {
      const next = await requestDashboardCharts();
      setCharts(next);
      setChartsError(null);
    } catch (reason) {
      setChartsError(reason instanceof Error ? reason.message : String(reason));
    }
  }, []);

  const refreshAll = React.useCallback(() => {
    void loadSummary(false);
    void loadCharts();
  }, [loadCharts, loadSummary]);

  React.useEffect(() => {
    void loadSummary(Boolean(dashboardSnapshot));
    void loadCharts();
    const summaryInterval = window.setInterval(() => void loadSummary(true), SUMMARY_REFRESH_INTERVAL_MS);
    const chartInterval = window.setInterval(() => void loadCharts(), CHART_REFRESH_INTERVAL_MS);
    return () => {
      window.clearInterval(summaryInterval);
      window.clearInterval(chartInterval);
    };
  }, [loadCharts, loadSummary]);

  const locale = i18n.resolvedLanguage || i18n.language || "zh-CN";
  const dailyChartData = React.useMemo(
    () =>
      (charts?.traffic.daily ?? []).map((item) => ({
        ...item,
        label: shortDashboardDay(item.day, locale),
      })),
    [charts?.traffic.daily, locale],
  );
  const hourlyTrafficAxisWidth = React.useMemo(
    () =>
      dashboardTrafficAxisWidth(
        (charts?.traffic.hourly ?? []).flatMap((item) => [item.up, item.down]),
      ),
    [charts?.traffic.hourly],
  );
  const dailyTrafficAxisWidth = React.useMemo(
    () =>
      dashboardTrafficAxisWidth(
        (charts?.traffic.daily ?? []).map((item) => item.billable),
      ),
    [charts?.traffic.daily],
  );

  return (
    <div className="flex flex-col gap-3 p-0 md:p-4">
      <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
        <AdminPageTitle description={t("admin_dashboard.subtitle")}>
          {t("admin_dashboard.title")}
        </AdminPageTitle>
        {data?.generated_at ? (
          <Button style={{ marginRight: 0 }} variant="ghost" color="gray" size="1" onClick={refreshAll}>
            <RefreshCw size={14} />
            {t("admin_dashboard.updated_at", {
              time: new Intl.DateTimeFormat(locale, {
                hour: "2-digit",
                minute: "2-digit",
                second: "2-digit",
              }).format(new Date(data.generated_at)),
            })}
          </Button>
        ) : null}
      </div>

      {error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Icon>
            <AlertCircle size={16} />
          </Callout.Icon>
          <Callout.Text className="flex flex-wrap items-center gap-2">
            <span>{t("admin_dashboard.load_failed")}: {error}</span>
            <Button size="1" variant="soft" onClick={refreshAll}>
              <RefreshCw size={14} />
              {t("common.retry")}
            </Button>
          </Callout.Text>
        </Callout.Root>
      ) : null}

      {loading && !data ? <OverviewSkeleton /> : null}

      {data ? (
        <>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
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

            <SummaryPanel
              icon={<WalletCards size={18} />}
              label={t("admin_dashboard.today_billable")}
              value={charts && !charts.traffic.error ? formatBytes(charts.traffic.today_billable) : "-"}
            >
              {charts && !charts.traffic.error ? <div className="grid grid-cols-2 gap-3">
                <span className="flex min-w-0 items-center gap-1.5">
                  <ArrowUpFromLine size={14} className="shrink-0 text-[var(--accent-11)]" />
                  <span className="truncate">{t("admin_dashboard.upload")} {formatBytes(charts.traffic.today_up)}</span>
                </span>
                <span className="flex min-w-0 items-center justify-end gap-1.5 text-right">
                  <ArrowDownToLine size={14} className="shrink-0 text-[var(--orange-11)]" />
                  <span className="truncate">{t("admin_dashboard.download")} {formatBytes(charts.traffic.today_down)}</span>
                </span>
              </div> : (
                <span>{chartsError || charts?.traffic.error ? t("admin_dashboard.data_unavailable") : t("admin_dashboard.chart_loading")}</span>
              )}
            </SummaryPanel>

            <SummaryPanel
              icon={<Database size={18} />}
              label={t("admin_dashboard.database_usage")}
              value={dashboardLocalStorageTotal(data) === null ? t("admin_dashboard.external_storage") : formatBytes(dashboardLocalStorageTotal(data) ?? 0)}
            >
              <div className="flex items-center justify-between gap-3">
                <span>{t("admin_dashboard.database_files")} {formatBytes(data.storage.database_files)}</span>
                <span>WAL + SHM {formatBytes(dashboardRuntimeStorageTotal(data))}</span>
              </div>
            </SummaryPanel>
          </div>

          <div className="grid grid-cols-1 gap-3 xl:grid-cols-[minmax(0,1.55fr)_minmax(24rem,0.95fr)]">
            <div className="flex min-w-0 flex-col gap-3">
              <LatencyPanel
                charts={charts}
                locale={locale}
                requestError={chartsError}
                warningCount={data.alerts.latency_loss.error ? 0 : data.alerts.latency_loss.current}
              />

              <section className="min-w-0 rounded-md border bg-[var(--color-panel-solid)] p-3">
                <PanelHeader
                  title={t("admin_dashboard.today_traffic")}
                  description={t("admin_dashboard.hourly_traffic_hint")}
                  trailing={(
                    <div className="mt-0.5 flex shrink-0 items-center gap-3 text-xs text-muted-foreground">
                      <span className="flex items-center gap-1.5"><span className="size-2 rounded-sm bg-[var(--accent-9)]" />{t("admin_dashboard.upload")}</span>
                      <span className="flex items-center gap-1.5"><span className="size-2 rounded-sm bg-[var(--orange-9)]" />{t("admin_dashboard.download")}</span>
                    </div>
                  )}
                />
                {chartsError || charts?.traffic.error ? (
                  <div className="flex h-[220px] items-center justify-center text-sm text-[var(--red-11)]">
                    {t("admin_dashboard.data_unavailable")}
                  </div>
                ) : charts ? <ChartContainer
                  config={{
                    up: { label: t("admin_dashboard.upload"), color: "var(--accent-9)" },
                    down: { label: t("admin_dashboard.download"), color: "var(--orange-9)" },
                  }}
                  className="h-[220px] w-full aspect-auto"
                >
                  <LineChart data={charts.traffic.hourly} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                    <CartesianGrid vertical={false} strokeDasharray="3 3" />
                    <XAxis dataKey="hour" tickLine={false} axisLine={false} minTickGap={28} />
                    <YAxis tickLine={false} axisLine={false} width={hourlyTrafficAxisWidth} tickFormatter={(value) => formatBytes(Number(value)).replace(" ", "")} />
                    <Tooltip
                      content={({ active, payload, label }) => active && payload?.length ? (
                        <div className="rounded-md border bg-background px-3 py-2 text-xs shadow-lg">
                          <div className="mb-1.5 text-muted-foreground">{label}</div>
                          <div className="font-medium text-[var(--accent-11)]">{t("admin_dashboard.upload")}: {formatBytes(Number(payload[0]?.value ?? 0))}</div>
                          <div className="mt-1 font-medium text-[var(--orange-11)]">{t("admin_dashboard.download")}: {formatBytes(Number(payload[1]?.value ?? 0))}</div>
                        </div>
                      ) : null}
                    />
                    <Line type="monotone" dataKey="up" stroke="var(--color-up)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} />
                    <Line type="monotone" dataKey="down" stroke="var(--color-down)" strokeWidth={2} dot={false} activeDot={{ r: 3 }} />
                  </LineChart>
                </ChartContainer> : <Skeleton className="h-[220px] w-full" />}
              </section>

              <section className="min-w-0 rounded-md border bg-[var(--color-panel-solid)] p-3">
                <PanelHeader
                  title={t("admin_dashboard.daily_billable")}
                  description={t("admin_dashboard.daily_billable_hint")}
                  trailing={<span className="rounded-full bg-[var(--accent-a3)] px-2.5 py-1 text-xs font-medium text-[var(--accent-11)]">{t("admin_dashboard.recent_month")}</span>}
                />
                {charts && !charts.traffic.error && !charts.traffic.history_ready ? (
                  <p className="mb-2 text-xs text-muted-foreground">{t("admin_dashboard.history_preparing")}</p>
                ) : null}
                {chartsError || charts?.traffic.error ? (
                  <div className="flex h-[220px] items-center justify-center text-sm text-[var(--red-11)]">
                    {t("admin_dashboard.data_unavailable")}
                  </div>
                ) : charts ? <ChartContainer config={{ billable: { label: t("admin_dashboard.billable"), color: "var(--accent-9)" } }} className="h-[220px] w-full aspect-auto">
                  <BarChart data={dailyChartData} margin={{ top: 8, right: 4, left: 0, bottom: 0 }}>
                    <CartesianGrid vertical={false} strokeDasharray="3 3" />
                    <XAxis dataKey="label" tickLine={false} axisLine={false} interval="preserveStartEnd" minTickGap={24} />
                    <YAxis tickLine={false} axisLine={false} width={dailyTrafficAxisWidth} tickFormatter={(value) => formatBytes(Number(value)).replace(" ", "")} />
                    <Tooltip
                      cursor={{ fill: "var(--accent-a3)" }}
                      content={({ active, payload, label }) => active && payload?.length ? (
                        <div className="rounded-md border bg-background px-3 py-2 text-xs shadow-lg">
                          <div className="mb-1 text-muted-foreground">{label}</div>
                          <div className="font-medium">{t("admin_dashboard.billable")}: {formatBytes(Number(payload[0]?.value ?? 0))}</div>
                        </div>
                      ) : null}
                    />
                    <Bar dataKey="billable" fill="var(--color-billable)" radius={[2, 2, 0, 0]} maxBarSize={24} />
                  </BarChart>
                </ChartContainer> : <Skeleton className="h-[220px] w-full" />}
              </section>
            </div>

            <div className="flex min-w-0 flex-col gap-3">
              <AlertOverviewPanel data={data} locale={locale} />
              <StoragePanel data={data} locale={locale} />
            </div>
          </div>
        </>
      ) : null}
    </div>
  );
}
*/

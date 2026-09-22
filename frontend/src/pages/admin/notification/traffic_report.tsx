import AppDialogContent from "@/components/AppDialogContent";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  NodeDetailsProvider,
  useNodeDetails,
} from "@/contexts/NodeDetailsContext";
import {
  TrafficReportNotificationProvider,
  useTrafficReportNotification,
  type TrafficReportNotification,
} from "@/contexts/TrafficReportContext";
import React from "react";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import { AdminSelectionCount } from "@/components/admin/AdminSelectionCount";
import {
  AdminPagination,
  useAdminPagination,
} from "@/components/admin/AdminPagination";
import { Clock3, Pencil, Save, Search, Send, Settings2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  Badge,
  Button,
  Dialog,
  Flex,
  Switch,
  TextField,
} from "@radix-ui/themes";
import { toast } from "sonner";
import Loading from "@/components/loading";
import { useSettings } from "@/lib/api";

type TrafficReportFormValues = {
  enable: boolean;
  daily: boolean;
  weekly: boolean;
  monthly: boolean;
  include_traffic: boolean;
  include_billing: boolean;
};

const validateReportSelection = (
  values: TrafficReportFormValues,
  t: (key: string, options?: Record<string, unknown>) => string
) => {
  if (values.enable && !values.daily && !values.weekly && !values.monthly) {
    throw new Error(t("notification.traffic_report.errors.select_cadence"));
  }
  if (values.enable && !values.include_traffic && !values.include_billing) {
    throw new Error(t("notification.traffic_report.errors.select_content"));
  }
};

const getErrorMessage = (
  error: unknown,
  t: (key: string, options?: Record<string, unknown>) => string
) => {
  return error instanceof Error ? error.message : t("common.error");
};

const parseJsonOrThrow = async (res: Response, fallbackMessage: string) => {
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.message || fallbackMessage);
  }
  if (res.status === 204) {
    return null;
  }

  const contentType = res.headers.get("content-type") || "";
  if (!contentType.includes("application/json")) {
    return null;
  }

  const text = await res.text();
  if (!text) {
    return null;
  }

  return JSON.parse(text);
};

const TrafficReportPage = () => {
  return (
    <TrafficReportNotificationProvider>
      <NodeDetailsProvider>
        <InnerLayout />
      </NodeDetailsProvider>
    </TrafficReportNotificationProvider>
  );
};

const ReportOption = ({
  id,
  checked,
  onCheckedChange,
  children,
}: {
  id: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  children: React.ReactNode;
}) => (
  <div className="flex items-center gap-2">
    <Checkbox
      id={id}
      checked={checked}
      onCheckedChange={(value) => onCheckedChange(value === true)}
    />
    <label htmlFor={id} className="cursor-pointer select-none">
      {children}
    </label>
  </div>
);

// 表单：编辑单条或批量修改
const TrafficReportEditForm = ({
  initialValues,
  onSubmit,
  loading,
  onCancel,
  statusLabel,
}: {
  initialValues: TrafficReportFormValues;
  onSubmit: (values: TrafficReportFormValues) => void;
  loading?: boolean;
  onCancel?: () => void;
  statusLabel?: string;
}) => {
  const { t } = useTranslation();
  const formId = React.useId();
  const [enabled, setEnabled] = React.useState(initialValues.enable);
  const [daily, setDaily] = React.useState(initialValues.daily);
  const [weekly, setWeekly] = React.useState(initialValues.weekly);
  const [monthly, setMonthly] = React.useState(initialValues.monthly);
  const [includeTraffic, setIncludeTraffic] = React.useState(
    initialValues.include_traffic
  );
  const [includeBilling, setIncludeBilling] = React.useState(
    initialValues.include_billing
  );

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit({
          enable: enabled,
          daily,
          weekly,
          monthly,
          include_traffic: includeTraffic,
          include_billing: includeBilling,
        });
      }}
      className="mt-4 flex flex-col gap-5"
    >
      <div className="flex items-center justify-between gap-4">
        <label htmlFor={`${formId}-status`} className="font-medium">
          {statusLabel ?? t("common.status")}
        </label>
        <Switch
          id={`${formId}-status`}
          name="status"
          checked={enabled}
          onCheckedChange={setEnabled}
        />
      </div>

      <fieldset className="grid min-w-0 gap-3 border-0 p-0">
        <legend className="font-medium">
          {t("notification.traffic_report.report_type")}
        </legend>
        <div className="grid gap-3 pl-1">
          <ReportOption
            id={`${formId}-daily`}
            checked={daily}
            onCheckedChange={setDaily}
          >
            {t("notification.traffic_report.daily")}
          </ReportOption>
          <ReportOption
            id={`${formId}-weekly`}
            checked={weekly}
            onCheckedChange={setWeekly}
          >
            {t("notification.traffic_report.weekly")}
          </ReportOption>
          <ReportOption
            id={`${formId}-monthly`}
            checked={monthly}
            onCheckedChange={setMonthly}
          >
            {t("notification.traffic_report.monthly")}
          </ReportOption>
        </div>
      </fieldset>

      <fieldset className="grid min-w-0 gap-3 border-0 p-0">
        <legend className="font-medium">
          {t("notification.traffic_report.report_content")}
        </legend>
        <div className="grid gap-3 pl-1">
          <ReportOption
            id={`${formId}-include-traffic`}
            checked={includeTraffic}
            onCheckedChange={setIncludeTraffic}
          >
            {t("notification.traffic_report.traffic_content")}
          </ReportOption>
          <ReportOption
            id={`${formId}-include-billing`}
            checked={includeBilling}
            onCheckedChange={setIncludeBilling}
          >
            {t("notification.traffic_report.billing_content")}
          </ReportOption>
        </div>
      </fieldset>

      <Flex gap="2" justify="end" className="mt-2">
        {onCancel && (
          <Dialog.Close>
            <Button variant="soft" color="gray" type="button" onClick={onCancel}>
              {t("common.cancel")}
            </Button>
          </Dialog.Close>
        )}
        <Button variant="solid" type="submit" disabled={loading}>
          {t("common.save")}
        </Button>
      </Flex>
    </form>
  );
};

const reportContentLabel = (
  n: TrafficReportNotification | undefined,
  t: (key: string) => string
): string => {
  if (!n) return "-";
  const parts: string[] = [];
  if (n.include_traffic) {
    parts.push(t("notification.traffic_report.traffic_content"));
  }
  if (n.include_billing) {
    parts.push(t("notification.traffic_report.billing_content"));
  }
  return parts.length > 0
    ? parts.join(t("notification.traffic_report.separator"))
    : "-";
};

// 把三个 bool 转成展示文字
const reportTypeLabel = (
  n: TrafficReportNotification | undefined,
  t: (key: string) => string
): string => {
  if (!n) return "-";
  const parts: string[] = [];
  if (n.daily) parts.push(t("notification.traffic_report.daily"));
  if (n.weekly) parts.push(t("notification.traffic_report.weekly"));
  if (n.monthly) parts.push(t("notification.traffic_report.monthly"));
  return parts.length > 0
    ? parts.join(t("notification.traffic_report.separator"))
    : "-";
};

const InnerLayout = () => {
  const [search, setSearch] = React.useState("");
  const [selected, setSelected] = React.useState<string[]>([]);
  const {
    loading: onLoading,
    error: onError,
    trafficReportNotification,
    refresh,
  } = useTrafficReportNotification();
  const {
    nodeDetail,
    isLoading: onNodeLoading,
    error: onNodeError,
  } = useNodeDetails();
  const { t } = useTranslation();
  const {
    settings,
    loading: settingsLoading,
    error: settingsError,
    updateSetting,
  } = useSettings();
  const [batchLoading, setBatchLoading] = React.useState(false);
  const [batchDialogOpen, setBatchDialogOpen] = React.useState(false);
  const [defaultDialogOpen, setDefaultDialogOpen] = React.useState(false);
  const [defaultSaving, setDefaultSaving] = React.useState(false);
  const [defaultConfig, setDefaultConfig] = React.useState({
    enabled: false,
    daily: true,
    weekly: false,
    monthly: false,
    include_traffic: true,
    include_billing: false,
  });
  const [defaultForm, setDefaultForm] = React.useState({
    enable: false,
    daily: true,
    weekly: false,
    monthly: false,
    include_traffic: true,
    include_billing: false,
  });
  const [reportTime, setReportTime] = React.useState("00:00");
  const [reportTimeSaving, setReportTimeSaving] = React.useState(false);
  const [dailySending, setDailySending] = React.useState(false);
  const [batchForm, setBatchForm] = React.useState({
    enable: true,
    daily: false,
    weekly: false,
    monthly: false,
    include_traffic: true,
    include_billing: false,
  });
  const filteredNodeIds = React.useMemo(
    () => nodeDetail
      .filter((node) => node.name.toLowerCase().includes(search.toLowerCase()))
      .map((node) => node.uuid),
    [nodeDetail, search],
  );
  const selectedFilteredIds = React.useMemo(() => {
    const selectedSet = new Set(selected);
    return filteredNodeIds.filter((id) => selectedSet.has(id));
  }, [filteredNodeIds, selected]);
  const selectedFilteredCount = selectedFilteredIds.length;
  const allFilteredSelected = filteredNodeIds.length > 0
    && selectedFilteredCount === filteredNodeIds.length;

  const toggleSelectAll = () => {
    if (allFilteredSelected) {
      const filteredSet = new Set(filteredNodeIds);
      setSelected((current) => current.filter((id) => !filteredSet.has(id)));
      return;
    }
    setSelected((current) => Array.from(new Set([...current, ...filteredNodeIds])));
  };

  React.useEffect(() => {
    let cancelled = false;
    fetch("/api/admin/notification/traffic-report/default", {
      cache: "no-store",
    })
      .then((response) =>
        parseJsonOrThrow(
          response,
          t("notification.traffic_report.errors.fetch_default_failed"),
        )
      )
      .then((payload) => {
        if (cancelled) return;
        const value = payload?.data;
        setDefaultConfig({
          enabled: value?.enabled === true,
          daily: value?.daily !== false,
          weekly: value?.weekly === true,
          monthly: value?.monthly === true,
          include_traffic: value?.include_traffic !== false,
          include_billing: value?.include_billing === true,
        });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [t]);

  const saveDefaultConfig = async (values: TrafficReportFormValues) => {
    try {
      validateReportSelection(values, t);
    } catch (error) {
      toast.error(getErrorMessage(error, t));
      return;
    }

    const next = {
      enabled: values.enable,
      daily: values.daily,
      weekly: values.weekly,
      monthly: values.monthly,
      include_traffic: values.include_traffic,
      include_billing: values.include_billing,
    };
    setDefaultSaving(true);
    try {
      const response = await fetch(
        "/api/admin/notification/traffic-report/default",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(next),
        },
      );
      await parseJsonOrThrow(
        response,
        t("notification.traffic_report.errors.save_default_failed"),
      );
      setDefaultConfig(next);
      toast.success(t("common.updated_successfully"));
      setDefaultDialogOpen(false);
    } catch (error) {
      toast.error(getErrorMessage(error, t));
    } finally {
      setDefaultSaving(false);
    }
  };

  const savedReportTime = settings.traffic_report_time || "00:00";
  const reportTimeValid = /^(?:[01]\d|2[0-3]):[0-5]\d$/.test(reportTime);

  React.useEffect(() => {
    setReportTime(savedReportTime);
  }, [savedReportTime]);

  const saveReportTime = async () => {
    setReportTimeSaving(true);
    try {
      await updateSetting("traffic_report_time", reportTime);
      toast.success(t("settings.settings_saved"));
    } catch (error) {
      toast.error(getErrorMessage(error, t));
    } finally {
      setReportTimeSaving(false);
    }
  };

  const sendDailyReport = async () => {
    setDailySending(true);
    try {
      const response = await fetch(
        "/api/admin/notification/traffic-report/send-daily",
        { method: "POST" }
      );
      const payload = await parseJsonOrThrow(
        response,
        t("notification.traffic_report.errors.send_failed")
      );
      const result = payload?.data;
      if (!result?.sent) {
        toast.warning(t("notification.traffic_report.no_daily_targets"));
        return;
      }
      toast.success(
        t("notification.traffic_report.sent_success", {
          count: result.client_count,
        })
      );
    } catch (error) {
      toast.error(getErrorMessage(error, t));
    } finally {
      setDailySending(false);
    }
  };

  const handleBatchEdit = (values: TrafficReportFormValues) => {
    try {
      validateReportSelection(values, t);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("common.error"));
      return;
    }

    setBatchLoading(true);
    const payload = selectedFilteredIds.map((id) => ({
      client: id,
      ...values,
    }));
    fetch("/api/admin/notification/traffic-report/edit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    })
      .then((res) =>
        parseJsonOrThrow(
          res,
          t("notification.traffic_report.errors.update_failed")
        )
      )
      .then(() => {
        setBatchDialogOpen(false);
        toast.success(t("common.updated_successfully"));
        refresh();
      })
      .catch((error) => {
        console.error("Error updating traffic report notifications:", error);
        toast.error(getErrorMessage(error, t));
      })
      .finally(() => {
        setBatchLoading(false);
      });
  };

  if (onLoading || onNodeLoading || settingsLoading) {
    return <Loading text={t("loading")} />;
  }
  if (onError || onNodeError || settingsError) {
    return <div>{t("common.error")}: {onError?.message || onNodeError || settingsError}</div>;
  }

  return (
    <div className="flex flex-col gap-5 p-0 md:p-4">
      <Flex justify="between" align="center" gap="3" wrap="wrap">
        <AdminPageTitle
          description={t(
            "notification.traffic_report.description",
            "按节点设置日报、周报与月报的推送周期和内容。",
          )}
        >
          {t("notification.traffic_report.full_title")}
        </AdminPageTitle>
      </Flex>

      <div className="grid overflow-hidden rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] lg:grid-cols-2">
        <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-start gap-3">
            <Clock3
              className="mt-0.5 shrink-0 text-[var(--accent-10)]"
              size={18}
            />
            <div className="min-w-0">
              <div className="font-medium">
                {t("notification.traffic_report.report_time")}
              </div>
              <div className="text-sm text-muted-foreground">
                {t("notification.traffic_report.report_time_description")}
              </div>
            </div>
          </div>
          <Flex gap="2" align="center" className="w-full shrink-0 sm:w-auto">
            <TextField.Root
              type="time"
              aria-label={t("notification.traffic_report.report_time")}
              className="min-w-0 flex-1 sm:w-32 sm:flex-none"
              value={reportTime}
              onChange={(event) => setReportTime(event.target.value)}
              disabled={reportTimeSaving}
            />
            <Button
              type="button"
              variant="soft"
              onClick={saveReportTime}
              disabled={
                reportTimeSaving ||
                !reportTimeValid ||
                reportTime === savedReportTime
              }
            >
              <Save size={16} />
              {t("common.save")}
            </Button>
          </Flex>
        </div>

        <div className="flex flex-col gap-3 border-t border-[var(--gray-a5)] p-4 sm:flex-row sm:items-center sm:justify-between lg:border-l lg:border-t-0">
          <div className="flex min-w-0 items-start gap-3">
            <Send
              className="mt-0.5 shrink-0 text-[var(--accent-10)]"
              size={18}
            />
            <div className="min-w-0">
              <div className="font-medium">
                {t("notification.traffic_report.send_daily")}
              </div>
              <div className="text-sm text-muted-foreground">
                {t("notification.traffic_report.send_daily_description")}
              </div>
            </div>
          </div>
          <Button
            type="button"
            className="w-full shrink-0 sm:w-auto"
            onClick={sendDailyReport}
            disabled={dailySending}
          >
            <Send size={16} />
            {t("notification.traffic_report.send_now")}
          </Button>
        </div>
      </div>

      <div className="flex flex-col gap-3">
        <TrafficReportTable
          search={search}
          selected={selected}
          onSelectionChange={setSelected}
          paginationSummary={
            <AdminSelectionCount
              count={selectedFilteredCount}
              total={filteredNodeIds.length}
              className="hidden md:inline-flex"
            />
          }
        />
        <div className="order-first flex min-w-0 flex-col gap-2 px-1 md:flex-row md:items-center md:justify-end">
          <AdminSelectionCount
            count={selectedFilteredCount}
            total={filteredNodeIds.length}
            className="shrink-0 text-sm text-muted-foreground md:hidden"
          />
          <div className="flex min-w-0 flex-wrap items-center gap-2 md:justify-end">
            <Button
              type="button"
              variant="soft"
              className="shrink-0"
              disabled={filteredNodeIds.length === 0}
              onClick={toggleSelectAll}
            >
              {t(allFilteredSelected ? "common.deselect_all" : "common.select_all")}
            </Button>
            <Dialog.Root open={batchDialogOpen} onOpenChange={setBatchDialogOpen}>
              <Dialog.Trigger>
                <Button
                  variant="soft"
                  className="shrink-0"
                  onClick={() => {
                    const first = trafficReportNotification.find(
                      (n) => n.client === selectedFilteredIds[0]
                    );
                    setBatchForm({
                      enable: first?.enable ?? true,
                      daily: first?.daily ?? false,
                      weekly: first?.weekly ?? false,
                      monthly: first?.monthly ?? false,
                      include_traffic: first?.include_traffic ?? true,
                      include_billing: first?.include_billing ?? false,
                    });
                  }}
                  disabled={batchLoading || selectedFilteredCount === 0}
                >
                  {t("notification.traffic_report.batch_edit")}
                </Button>
              </Dialog.Trigger>
              <AppDialogContent maxWidth="560px">
                <Dialog.Title>
                  {t("notification.traffic_report.batch_edit")}
                </Dialog.Title>
                <TrafficReportEditForm
                  initialValues={batchForm}
                  loading={batchLoading}
                  onSubmit={handleBatchEdit}
                  onCancel={() => setBatchDialogOpen(false)}
                />
              </AppDialogContent>
            </Dialog.Root>
            <TextField.Root
              type="text"
              className="order-last min-w-0 w-full basis-full sm:order-none sm:w-64 sm:basis-auto sm:flex-none"
              placeholder={t("common.search")}
              value={search}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                setSearch(e.target.value)
              }
            >
              <TextField.Slot>
                <Search size={16} />
              </TextField.Slot>
            </TextField.Root>
            <Dialog.Root open={defaultDialogOpen} onOpenChange={setDefaultDialogOpen}>
              <Dialog.Trigger>
                <Button
                  type="button"
                  variant="soft"
                  className="shrink-0"
                  onClick={() => {
                    setDefaultForm({
                      enable: defaultConfig.enabled,
                      daily: defaultConfig.daily,
                      weekly: defaultConfig.weekly,
                      monthly: defaultConfig.monthly,
                      include_traffic: defaultConfig.include_traffic,
                      include_billing: defaultConfig.include_billing,
                    });
                  }}
                >
                  <Settings2 size={16} />
                  {t("notification.traffic_report.default_config")}
                </Button>
              </Dialog.Trigger>
              <AppDialogContent
                title={t("notification.traffic_report.default_config")}
                description={t(
                  "notification.traffic_report.default_config_description",
                )}
                maxWidth="560px"
              >
                <TrafficReportEditForm
                  initialValues={defaultForm}
                  loading={defaultSaving}
                  statusLabel={t(
                    "notification.traffic_report.default_config_enabled",
                  )}
                  onSubmit={saveDefaultConfig}
                  onCancel={() => setDefaultDialogOpen(false)}
                />
              </AppDialogContent>
            </Dialog.Root>
          </div>
        </div>
      </div>
    </div>
  );
};

const TrafficReportTable = ({
  search,
  selected,
  onSelectionChange,
  paginationSummary,
}: {
  search: string;
  selected: string[];
  onSelectionChange: (ids: string[]) => void;
  paginationSummary?: React.ReactNode;
}) => {
  const { trafficReportNotification } = useTrafficReportNotification();
  const { nodeDetail } = useNodeDetails();
  const { t } = useTranslation();

  const filtered = nodeDetail.filter((node) =>
    node.name.toLowerCase().includes(search.toLowerCase())
  );
  const { page, setPage, pageItems, pageSize, setPageSize } =
    useAdminPagination(filtered);
  React.useEffect(() => setPage(1), [search, setPage]);

  return (
    <div className="admin-responsive-table-wrap overflow-hidden rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)]">
      <div className="overflow-x-auto">
      <Table className="admin-responsive-table admin-selection-table min-w-[640px]">
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 px-3 text-center">
              <span className="sr-only">{t("common.select")}</span>
            </TableHead>
            <TableHead>{t("common.server")}</TableHead>
            <TableHead>{t("common.status")}</TableHead>
            <TableHead>{t("notification.traffic_report.report_type")}</TableHead>
            <TableHead>{t("notification.traffic_report.report_content")}</TableHead>
            <TableHead className="text-center">{t("common.action")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageItems.map((node) => {
            const n = trafficReportNotification.find(
              (item) => item.client === node.uuid
            );
            return (
              <TableRow key={node.uuid}>
                <TableCell className="w-12 px-3" data-label={t("common.select", "选择")}>
                  <div className="flex items-center justify-center">
                    <Checkbox
                      checked={selected.includes(node.uuid)}
                      aria-label={t("common.select", "选择")}
                      onCheckedChange={(checked) => {
                        if (checked) {
                          onSelectionChange([...selected, node.uuid]);
                        } else {
                          onSelectionChange(
                            selected.filter((id) => id !== node.uuid)
                          );
                        }
                      }}
                    />
                  </div>
                </TableCell>
                <TableCell data-label={t("common.server")}>{node.name}</TableCell>
                <TableCell data-label={t("common.status")}>
                  <Badge color={n?.enable ? "green" : "red"}>
                    {n?.enable ? t("common.enabled") : t("common.disabled")}
                  </Badge>
                </TableCell>
                <TableCell data-label={t("notification.traffic_report.report_type")}>{reportTypeLabel(n, t)}</TableCell>
                <TableCell data-label={t("notification.traffic_report.report_content")}>{reportContentLabel(n, t)}</TableCell>
                <TableCell className="text-center" data-label={t("common.action")}>
                  <ActionButtons
                    nodeUUID={node.uuid}
                    trafficReport={n}
                  />
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
      </div>
      <AdminPagination
        page={page}
        total={filtered.length}
        pageSize={pageSize}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
        summary={paginationSummary}
      />
    </div>
  );
};

const ActionButtons = ({
  nodeUUID,
  trafficReport,
}: {
  nodeUUID: string;
  trafficReport: TrafficReportNotification | undefined;
}) => {
  const { t } = useTranslation();
  const { refresh } = useTrafficReportNotification();
  const [editOpen, setEditOpen] = React.useState(false);
  const [editSaving, setEditSaving] = React.useState(false);

  return (
    <Flex gap="2" align="center" className="admin-card-actions admin-single-text-action w-full">
      <Dialog.Root open={editOpen} onOpenChange={setEditOpen}>
        <Dialog.Trigger>
          <Button
            variant="ghost"
            className="admin-single-action-button"
            aria-label={t("common.modify", "修改")}
            title={t("common.modify", "修改")}
          >
            <Pencil size={16} />
            <span className="admin-single-action-label">
              {t("common.modify", "修改")}
            </span>
          </Button>
        </Dialog.Trigger>
        <AppDialogContent maxWidth="560px">
          <Dialog.Title>{t("common.edit")}</Dialog.Title>
          <TrafficReportEditForm
            initialValues={{
              enable: trafficReport?.enable ?? false,
              daily: trafficReport?.daily ?? false,
              weekly: trafficReport?.weekly ?? false,
              monthly: trafficReport?.monthly ?? false,
              include_traffic: trafficReport?.include_traffic ?? true,
              include_billing: trafficReport?.include_billing ?? false,
            }}
            loading={editSaving}
            onSubmit={(values) => {
              try {
                validateReportSelection(values, t);
              } catch (error) {
                toast.error(
                  error instanceof Error ? error.message : t("common.error")
                );
                return;
              }

              setEditSaving(true);
              fetch("/api/admin/notification/traffic-report/edit", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify([
                  {
                    client: nodeUUID,
                    ...values,
                  },
                ]),
              })
                .then((res) =>
                  parseJsonOrThrow(
                    res,
                    t("notification.traffic_report.errors.save_failed")
                  )
                )
                .then(() => {
                  setEditOpen(false);
                  toast.success(t("common.updated_successfully"));
                  refresh();
                })
                .catch((error) => {
                  console.error("Error saving traffic report settings:", error);
                  toast.error(getErrorMessage(error, t));
                })
                .finally(() => {
                  setEditSaving(false);
                });
            }}
            onCancel={() => setEditOpen(false)}
          />
        </AppDialogContent>
      </Dialog.Root>
    </Flex>
  );
};

export default TrafficReportPage;

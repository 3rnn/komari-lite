import AppDialogContent from "@/components/AppDialogContent";
import Loading from "@/components/loading";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import {
  AdminPagination,
  useAdminPagination,
} from "@/components/admin/AdminPagination";
import NodeSelectorDialog from "@/components/NodeSelectorDialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  LoadAlertProvider,
  useLoadAlert,
  type LoadAlert,
  type CurrentLoadAlert,
} from "@/contexts/LoadAlertContext";
import {
  NodeDetailsProvider,
  useNodeDetails,
} from "@/contexts/NodeDetailsContext";

import {
  Badge,
  Button,
  Callout,
  Dialog,
  DropdownMenu,
  Flex,
  IconButton,
  Select,
  Tabs,
  TextField,
} from "@radix-ui/themes";
import { Checkbox } from "@/components/ui/checkbox";
import {
  BellOff,
  BellRing,
  CircleAlert,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  Trash,
} from "lucide-react";
import React from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

const LoadPage = () => {
  return (
    <LoadAlertProvider>
      <NodeDetailsProvider>
        <InnerLayout />
      </NodeDetailsProvider>
    </LoadAlertProvider>
  );
};

const InnerLayout = () => {
  const {
    loadAlerts,
    currentAlerts,
    isLoading,
    currentLoading,
    error,
    currentError,
    refreshCurrent,
  } = useLoadAlert();
  const { isLoading: nodeDetailLoading, error: nodeDetailError } =
    useNodeDetails();
  const { nodeDetail } = useNodeDetails();
  const { t } = useTranslation();
  const [view, setView] = React.useState<"configuration" | "current">(
    "configuration",
  );
  const [search, setSearch] = React.useState("");
  const normalizedSearch = search.trim().toLocaleLowerCase();
  const sortedAlerts = React.useMemo(
    () =>
      (loadAlerts || [])
        .filter((alert) => {
          if (!normalizedSearch) return true;
          const clientText = (alert.clients || [])
            .map((uuid) => `${uuid} ${nodeDetail.find((node) => node.uuid === uuid)?.name || ""}`)
            .join(" ");
          return `${alert.name || ""} ${alert.metric || ""} ${clientText}`
            .toLocaleLowerCase()
            .includes(normalizedSearch);
        })
        .slice()
        .sort((a, b) => (b.id ?? 0) - (a.id ?? 0)),
    [loadAlerts, nodeDetail, normalizedSearch],
  );
  const sortedCurrentAlerts = React.useMemo(
    () =>
      (currentAlerts || [])
        .filter((alert) => {
          if (!normalizedSearch) return true;
          const status = alert.silenced
            ? t("notification.load.silenced")
            : t("notification.load.not_silenced");
          const node = nodeDetail.find((item) => item.uuid === alert.client);
          return `${alert.notification_name} ${alert.client_name} ${alert.client} ${node?.ipv4 || ""} ${node?.ipv6 || ""} ${alert.metric} ${status}`
            .toLocaleLowerCase()
            .includes(normalizedSearch);
        })
        .slice()
        .sort((a, b) => {
          const left = a.active_since ? new Date(a.active_since).getTime() : 0;
          const right = b.active_since ? new Date(b.active_since).getTime() : 0;
          return right - left;
        }),
    [currentAlerts, nodeDetail, normalizedSearch, t],
  );
  const configurationPagination =
    useAdminPagination(sortedAlerts);
  const currentPagination = useAdminPagination(sortedCurrentAlerts);

  React.useEffect(() => {
    if (view !== "current") return;
    void refreshCurrent().catch(() => {
      toast.error(t("notification.load.current_fetch_failed"));
    });
    const timer = window.setInterval(() => {
      void refreshCurrent().catch(() => undefined);
    }, 30_000);
    return () => window.clearInterval(timer);
  }, [refreshCurrent, t, view]);

  if ((isLoading && loadAlerts === null) || nodeDetailLoading) {
    return <Loading />;
  }
  if ((error && loadAlerts === null) || nodeDetailError) {
    return <div>{error || nodeDetailError}</div>;
  }
  return (
    <Flex direction="column" gap="4" className="w-full min-w-0 p-0 md:p-4">
      <AdminPageTitle
        description={t(
          "notification.load.description",
          "配置 CPU、内存、负载等资源告警规则，并指定适用节点。",
        )}
      >
        {t("notification.load.title")}
      </AdminPageTitle>

      {error ? (
        <Callout.Root color="red" role="alert">
          <Callout.Icon>
            <CircleAlert size={16} />
          </Callout.Icon>
          <Callout.Text>{error}</Callout.Text>
        </Callout.Root>
      ) : null}

      <Tabs.Root value={view} onValueChange={(value) => setView(value as typeof view)}>
        <div className="w-full overflow-x-auto pb-1">
          <Tabs.List className="w-max min-w-full">
            <Tabs.Trigger value="configuration" className="min-w-[8rem] flex-1">
              {t("notification.load.configuration")}
            </Tabs.Trigger>
            <Tabs.Trigger value="current" className="min-w-[8rem] flex-1">
              {t("notification.load.current_alerts")}
            </Tabs.Trigger>
          </Tabs.List>
        </div>
        <Tabs.Content value="configuration" className="admin-tab-panel pt-3">
          <LoadListToolbar search={search} onSearchChange={setSearch} showAdd />
          <LoadConfigurationTable
            alerts={configurationPagination.pageItems}
            total={sortedAlerts.length}
            pagination={configurationPagination}
          />
        </Tabs.Content>
        <Tabs.Content value="current" className="admin-tab-panel pt-3">
          <LoadListToolbar search={search} onSearchChange={setSearch} />
          {currentError ? (
            <Callout.Root color="red" className="mb-3" role="alert">
              <Callout.Icon>
                <CircleAlert size={16} />
              </Callout.Icon>
              <Callout.Text>{currentError}</Callout.Text>
            </Callout.Root>
          ) : null}
          <CurrentLoadAlertsTable
            alerts={currentPagination.pageItems}
            total={sortedCurrentAlerts.length}
            pagination={currentPagination}
            loading={currentLoading && currentAlerts === null}
          />
          <div className="mt-4 border-l-2 border-[var(--accent-8)] pl-3 text-sm leading-6 text-muted-foreground">
            <span
              dangerouslySetInnerHTML={{ __html: t("notification.load.silence_tips") }}
            />
          </div>
        </Tabs.Content>
      </Tabs.Root>
    </Flex>
  );
};

const LoadListToolbar = ({
  search,
  onSearchChange,
  showAdd = false,
}: {
  search: string;
  onSearchChange: (value: string) => void;
  showAdd?: boolean;
}) => {
  const { t } = useTranslation();
  return (
    <div className="mb-3 flex min-w-0 items-center justify-end gap-2">
      <TextField.Root
        className="min-w-0 flex-1 sm:max-w-64"
        value={search}
        placeholder={t("common.search")}
        onChange={(event) => onSearchChange(event.target.value)}
      >
        <TextField.Slot>
          <Search size={16} />
        </TextField.Slot>
      </TextField.Root>
      {showAdd ? <AddButton /> : null}
    </div>
  );
};

type PaginationState<T> = {
  page: number;
  setPage: (page: number) => void;
  pageItems: T[];
  pageSize: number;
  setPageSize: (pageSize: number) => void;
};

const LoadConfigurationTable = ({
  alerts,
  total,
  pagination,
}: {
  alerts: LoadAlert[];
  total: number;
  pagination: PaginationState<LoadAlert>;
}) => {
  const { t } = useTranslation();
  if (total === 0) {
    return <EmptyState>{t("notification.load.empty_configuration")}</EmptyState>;
  }
  return (
    <div className="admin-responsive-table-wrap overflow-hidden rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)]">
      <div className="overflow-x-auto">
        <Table className="admin-responsive-table admin-primary-first-table min-w-[840px]">
          <TableHeader>
            <TableHead>{t("common.name")}</TableHead>
            <TableHead>{t("common.server")}</TableHead>
            <TableHead>{t("loadAlert.metric")}</TableHead>
            <TableHead>{t("common.threshold")}</TableHead>
            <TableHead>{t("loadAlert.ratio")}</TableHead>
            <TableHead>{t("ping.interval")}</TableHead>
            <TableHead>{t("common.action")}</TableHead>
          </TableHeader>
          <TableBody>
            {alerts.map((alert) => <Row key={alert.id} alert={alert} />)}
          </TableBody>
        </Table>
      </div>
      <AdminPagination
        page={pagination.page}
        total={total}
        pageSize={pagination.pageSize}
        onPageChange={pagination.setPage}
        onPageSizeChange={pagination.setPageSize}
        showSummary={false}
      />
    </div>
  );
};

const EmptyState = ({ children }: { children: React.ReactNode }) => (
  <div className="flex min-h-36 items-center justify-center rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] px-4 text-sm text-muted-foreground">
    {children}
  </div>
);

const CurrentLoadAlertsTable = ({
  alerts,
  total,
  pagination,
  loading,
}: {
  alerts: CurrentLoadAlert[];
  total: number;
  pagination: PaginationState<CurrentLoadAlert>;
  loading: boolean;
}) => {
  const { t } = useTranslation();
  if (loading) return <Loading />;
  if (total === 0) {
    return <EmptyState>{t("notification.load.no_current_alerts")}</EmptyState>;
  }
  return (
    <div className="admin-responsive-table-wrap overflow-hidden rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)]">
      <div className="overflow-x-auto">
        <Table className="admin-responsive-table admin-primary-first-table min-w-[940px]">
          <TableHeader>
            <TableHead>{t("common.name")}</TableHead>
            <TableHead>{t("common.server")}</TableHead>
            <TableHead>{t("loadAlert.metric")}</TableHead>
            <TableHead>{t("notification.load.current_value")}</TableHead>
            <TableHead>{t("common.threshold")}</TableHead>
            <TableHead>{t("notification.load.triggered_at")}</TableHead>
            <TableHead>{t("notification.load.silence_status")}</TableHead>
            <TableHead>{t("common.action")}</TableHead>
          </TableHeader>
          <TableBody>
            {alerts.map((alert) => (
              <CurrentLoadAlertRow
                key={`${alert.notification_id}:${alert.client}`}
                alert={alert}
              />
            ))}
          </TableBody>
        </Table>
      </div>
      <AdminPagination
        page={pagination.page}
        total={total}
        pageSize={pagination.pageSize}
        onPageChange={pagination.setPage}
        onPageSizeChange={pagination.setPageSize}
        showSummary={false}
      />
    </div>
  );
};

const formatLoadValue = (metric: string, value: number) => {
  const unit = metric === "net_in" || metric === "net_out" ? " Mbps" : "%";
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value)}${unit}`;
};

const CurrentLoadAlertRow = ({ alert }: { alert: CurrentLoadAlert }) => {
  const { t } = useTranslation();
  const { refreshCurrent } = useLoadAlert();
  const { nodeDetail } = useNodeDetails();
  const [saving, setSaving] = React.useState(false);
  const node = nodeDetail.find((item) => item.uuid === alert.client);
  const clientName = alert.client_name || node?.name || alert.client;
  const clientAddress = node?.ipv4?.trim() || node?.ipv6?.trim() || "";
  const setSilence = async (mode: "off" | "24h" | "3d" | "7d" | "forever") => {
    setSaving(true);
    try {
      const response = await fetch("/api/admin/notification/load/silence", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          notification_id: alert.notification_id,
          client: alert.client,
          mode,
        }),
      });
      if (!response.ok) {
        const data = await response.json().catch(() => null);
        throw new Error(data?.message || t("common.error"));
      }
      toast.success(
        t(mode === "off" ? "notification.load.unsilenced_success" : "notification.load.silenced_success"),
      );
      await refreshCurrent();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("common.error"));
    } finally {
      setSaving(false);
    }
  };
  const silenceLabel = alert.silenced_forever
    ? t("notification.load.silenced_forever")
    : alert.silenced && alert.silenced_until
      ? t("notification.load.silenced_until", {
          time: new Date(alert.silenced_until).toLocaleString(),
        })
      : t("notification.load.not_silenced");

  return (
    <TableRow>
      <TableCell data-label={t("common.name")}>
        <span className="font-medium">{alert.notification_name}</span>
      </TableCell>
      <TableCell data-label={t("common.server")}>
        <div className="min-w-0">
          <div className="truncate">{clientName}</div>
          {clientAddress ? (
            <div className="truncate text-xs text-muted-foreground">{clientAddress}</div>
          ) : null}
        </div>
      </TableCell>
      <TableCell data-label={t("loadAlert.metric")}>{alert.metric.toUpperCase()}</TableCell>
      <TableCell data-label={t("notification.load.current_value")} className="tabular-nums">
        {formatLoadValue(alert.metric, alert.latest_value)}
      </TableCell>
      <TableCell data-label={t("common.threshold")} className="tabular-nums">
        {formatLoadValue(alert.metric, alert.threshold)}
      </TableCell>
      <TableCell data-label={t("notification.load.triggered_at")} className="whitespace-nowrap text-sm">
        {alert.active_since ? new Date(alert.active_since).toLocaleString() : "-"}
      </TableCell>
      <TableCell data-label={t("notification.load.silence_status")}>
        <Badge color={alert.silenced ? "orange" : "red"} variant="soft">
          {silenceLabel}
        </Badge>
      </TableCell>
      <TableCell data-label={t("common.action")}>
        <div className="admin-card-actions flex items-center gap-2">
          <DropdownMenu.Root>
            <DropdownMenu.Trigger>
              <IconButton
                type="button"
                variant="soft"
                color={alert.silenced ? "orange" : "gray"}
                disabled={saving}
                title={t("notification.load.silence_action")}
                aria-label={t("notification.load.silence_action")}
              >
                {alert.silenced ? <BellOff size={16} /> : <BellRing size={16} />}
              </IconButton>
            </DropdownMenu.Trigger>
            <DropdownMenu.Content align="end">
              {alert.silenced ? (
                <DropdownMenu.Item onSelect={() => void setSilence("off")}>
                  <BellRing size={15} />
                  {t("notification.load.unsilence")}
                </DropdownMenu.Item>
              ) : null}
              <DropdownMenu.Item onSelect={() => void setSilence("24h")}>
                {t("notification.load.silence_24h")}
              </DropdownMenu.Item>
              <DropdownMenu.Item onSelect={() => void setSilence("3d")}>
                {t("notification.load.silence_3d")}
              </DropdownMenu.Item>
              <DropdownMenu.Item onSelect={() => void setSilence("7d")}>
                {t("notification.load.silence_7d")}
              </DropdownMenu.Item>
              <DropdownMenu.Separator />
              <DropdownMenu.Item color="orange" onSelect={() => void setSilence("forever")}>
                {t("notification.load.silence_forever")}
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Root>
        </div>
      </TableCell>
    </TableRow>
  );
};

const Row = ({ alert }: { alert: LoadAlert }) => {
  const { t } = useTranslation();
  const { refresh } = useLoadAlert();
  const { nodeDetail } = useNodeDetails();
  const [editOpen, setEditOpen] = React.useState(false);
  const [editSaving, setEditSaving] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleteLoading, setDeleteLoading] = React.useState(false);
  const [form, setForm] = React.useState({
    name: alert.name || "",
    metric: alert.metric || "cpu",
    threshold: alert.threshold || 80,
    ratio: alert.ratio || 0.8,
    clients: alert.clients || [],
    default_on: alert.default_on ?? false,
    interval: alert.interval || 15,
  });

  const submitEdit = (newForm: typeof form) => {
    if (!newForm.default_on && newForm.clients.length === 0) {
      toast.error(t("ping.default_on_description"));
      return;
    }
    setEditSaving(true);
    fetch("/api/admin/notification/load/edit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        notifications: [
          {
            id: alert.id,
            name: newForm.name,
            metric: newForm.metric,
            threshold: newForm.threshold,
            ratio: newForm.ratio,
            clients: newForm.clients,
            default_on: newForm.default_on,
            interval: newForm.interval,
          },
        ],
      }),
    })
      .then((res) => {
        if (!res.ok) {
          return res.json().then((data) => {
            throw new Error(data?.message || t("common.error"));
          });
        }
        return res.json();
      })
      .then(() => {
        setEditOpen(false);
        toast.success(t("common.updated_successfully"));
        refresh();
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => setEditSaving(false));
  };

  // 编辑提交
  const handleEdit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    submitEdit(form);
  };

  // 删除
  const handleDelete = () => {
    setDeleteLoading(true);
    fetch("/api/admin/notification/load/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id: [alert.id] }),
    })
      .then((res) => {
        if (!res.ok) {
          return res.json().then((data) => {
            throw new Error(data?.message || t("common.error"));
          });
        }
        return res.json();
      })
      .then(() => {
        setDeleteOpen(false);
        toast.success(t("common.deleted_successfully"));
        refresh();
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => setDeleteLoading(false));
  };

  return (
    <TableRow key={alert.id}>
      <TableCell data-label={t("common.name")}>{alert.name}</TableCell>
      <TableCell data-label={t("common.server")}>
        <div className="flex min-w-0 items-start gap-2">
          <span className="min-w-0 flex-1 whitespace-normal break-words">
            {alert.clients && alert.clients.length > 0
              ? alert.clients
                  .map(
                    (uuid) =>
                      nodeDetail.find((node) => node.uuid === uuid)?.name || uuid,
                  )
                  .join(", ")
              : t("common.none")}
          </span>
          {alert.default_on && (
            <span className="shrink-0 text-xs text-accent-11">
              {t("ping.default_on_short")}
            </span>
          )}
          <NodeSelectorDialog
            value={form.clients ?? []}
            hiddenUuidOnlyClient
            onChange={(uuids) => {
              const nextForm = { ...form, clients: uuids };
              setForm(nextForm);
              submitEdit(nextForm);
            }}
          >
            <IconButton variant="ghost" className="shrink-0">
              <MoreHorizontal size="16" />
            </IconButton>
          </NodeSelectorDialog>
        </div>
      </TableCell>
      <TableCell data-label={t("loadAlert.metric")}>{alert.metric?.toUpperCase()}</TableCell>
      <TableCell data-label={t("common.threshold")}>{alert.threshold}%</TableCell>
      <TableCell data-label={t("loadAlert.ratio")}>{alert.ratio}</TableCell>
      <TableCell data-label={t("ping.interval")}>
        {alert.interval} {t("time.minute")}
      </TableCell>
      <TableCell data-label={t("common.action")}>
        <div className="admin-card-actions admin-dual-actions flex items-center gap-3">
        {/* 编辑按钮 */}
        <Dialog.Root open={editOpen} onOpenChange={setEditOpen}>
          <Dialog.Trigger>
            <IconButton variant="soft">
              <Pencil size="16" />
            </IconButton>
          </Dialog.Trigger>
          <AppDialogContent>
            <Dialog.Title>{t("common.edit")}</Dialog.Title>
            <form onSubmit={handleEdit} className="flex flex-col gap-2">
              <label>{t("common.name")}</label>
              <TextField.Root
                value={form.name}
                onChange={(e) =>
                  setForm((f) => ({ ...f, name: e.target.value }))
                }
                required
              />
              <label>{t("loadAlert.metric")}</label>
              <Select.Root
                value={form.metric}
                onValueChange={(v) =>
                  setForm((f) => ({ ...f, metric: v as any }))
                }
              >
                <Select.Trigger />
                <Select.Content>
                  <Select.Item value="cpu">CPU</Select.Item>
                  <Select.Item value="ram">RAM</Select.Item>
                  <Select.Item value="disk">Disk</Select.Item>
                  <Select.Item value="net_in">Net In</Select.Item>
                  <Select.Item value="net_out">Net Out</Select.Item>
                </Select.Content>
              </Select.Root>
              <label>{t("common.threshold")} (%)</label>
              <TextField.Root
                type="number"
                value={form.threshold}
                onChange={(e) =>
                  setForm((f) => ({ ...f, threshold: Number(e.target.value) }))
                }
                required
              />
              <label>{t("loadAlert.ratio")}</label>
              <TextField.Root
                type="number"
                step="0.1"
                min="0"
                max="1"
                value={form.ratio}
                onChange={(e) =>
                  setForm((f) => ({ ...f, ratio: Number(e.target.value) }))
                }
                required
              />
              <label>{t("common.server")}</label>
              <Flex>
                <NodeSelectorDialog
                  value={form.clients}
                  hiddenUuidOnlyClient
                  onChange={(v) => setForm((f) => ({ ...f, clients: v }))}
                />
              </Flex>
              <label className="flex min-h-10 items-center gap-2 text-sm font-normal">
                <Checkbox
                  checked={form.default_on}
                  onCheckedChange={(checked) =>
                    setForm((f) => ({ ...f, default_on: !!checked }))
                  }
                />
                <span>{t("ping.default_on")}</span>
              </label>
              <label className="text-sm font-normal text-gray-500">
                {t("ping.default_on_description")}
              </label>
              <label>
                {t("ping.interval")} ({t("time.minute")})
              </label>
              <TextField.Root
                type="number"
                value={form.interval}
                onChange={(e) =>
                  setForm((f) => ({ ...f, interval: Number(e.target.value) }))
                }
                required
              />
              <Flex gap="2" justify="end" className="mt-4">
                <Dialog.Close>
                  <Button
                    variant="soft"
                    color="gray"
                    type="button"
                    onClick={() => setEditOpen(false)}
                  >
                    {t("common.cancel")}
                  </Button>
                </Dialog.Close>
                <Button variant="solid" type="submit" disabled={editSaving}>
                  {t("common.save")}
                </Button>
              </Flex>
            </form>
          </AppDialogContent>
        </Dialog.Root>
        {/* 删除按钮 */}
        <Dialog.Root open={deleteOpen} onOpenChange={setDeleteOpen}>
          <Dialog.Trigger>
            <IconButton variant="soft" color="red">
              <Trash size="16" />
            </IconButton>
          </Dialog.Trigger>
          <AppDialogContent>
            <Dialog.Title>{t("common.delete")}</Dialog.Title>
            <Flex gap="2" justify="end" className="mt-4">
              <Dialog.Close>
                <Button
                  variant="soft"
                  color="gray"
                  type="button"
                  onClick={() => setDeleteOpen(false)}
                >
                  {t("common.cancel")}
                </Button>
              </Dialog.Close>
              <Button
                variant="solid"
                color="red"
                onClick={handleDelete}
                disabled={deleteLoading}
              >
                {t("common.delete")}
              </Button>
            </Flex>
          </AppDialogContent>
        </Dialog.Root>
        </div>
      </TableCell>
    </TableRow>
  );
};

const AddButton: React.FC = () => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = React.useState(false);
  const [selected, setSelected] = React.useState<string[]>([]);
  const [defaultOn, setDefaultOn] = React.useState(false);
  const { refresh } = useLoadAlert();
  const [selectedType, setSelectedType] = React.useState<
    "cpu" | "ram" | "disk" | "net_in" | "net_out"
  >("cpu");
  const [saving, setSaving] = React.useState(false);
  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!defaultOn && selected.length === 0) {
      toast.error(t("ping.default_on_description"));
      return;
    }
    const payload = {
      name: e.currentTarget.load_name.value,
      metric: selectedType,
      threshold: parseFloat(e.currentTarget.threshold.value),
      ratio: parseFloat(e.currentTarget.ratio.value),
      clients: selected,
      default_on: defaultOn,
      interval: parseInt(e.currentTarget.interval.value, 10),
    };
    setSaving(true);
    fetch("/api/admin/notification/load/add", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    })
      .then((response) => {
        if (response.ok) {
          setIsOpen(false);
          setSelected([]);
          setDefaultOn(false);
          setSelectedType("cpu");
          toast.success(t("common.success"));
        } else {
          response
            .json()
            .then((data) => {
              toast.error(data?.message || t("common.error"));
            })
            .catch((error) => {
              toast.error(error.message);
            });
        }
      })
      .catch((error) => {
        console.error("Error adding load alert:", error);
        toast.error(error.message);
      })
      .finally(() => {
        setSaving(false);
        refresh();
      });
  };
  return (
    <Dialog.Root open={isOpen} onOpenChange={setIsOpen}>
      <Dialog.Trigger>
        <Button className="w-full sm:w-auto">
          <Plus size={16} />
          {t("common.add")}
        </Button>
      </Dialog.Trigger>
      <AppDialogContent>
        <Dialog.Title>{t("common.add")}</Dialog.Title>
        <form onSubmit={handleSubmit}>
          <Flex direction="column" justify="end" gap="2" className="font-bold">
            <label htmlFor="load_name">{t("common.name")}</label>
            <TextField.Root id="load_name" name="load_name" />
            <label htmlFor="type">{t("loadAlert.metric")}</label>
            <Select.Root
              value={selectedType}
              onValueChange={(value) =>
                setSelectedType(
                  value as "cpu" | "ram" | "disk" | "net_in" | "net_out",
                )
              }
            >
              <Select.Trigger id="type" name="type" />
              <Select.Content>
                <Select.Item value="cpu">CPU</Select.Item>
                <Select.Item value="ram">RAM</Select.Item>
                <Select.Item value="disk">Disk</Select.Item>
                <Select.Item value="net_in">Net In(Mbps)</Select.Item>
                <Select.Item value="net_out">Net Out(Mbps)</Select.Item>
              </Select.Content>
            </Select.Root>
            <label htmlFor="threshold">{t("common.threshold")} (%/Mbps)</label>
            <TextField.Root
              id="threshold"
              name="threshold"
              type="number"
              defaultValue={80}
              step="0.1"
            />
            <label htmlFor="ratio">{t("loadAlert.ratio")}</label>
            <TextField.Root
              id="ratio"
              name="ratio"
              type="number"
              step="0.1"
              min="0"
              max="1"
              defaultValue={0.8}
            />
            <label htmlFor="select">{t("common.server")}</label>
            <div className="flex items-center justify-start gap-2">
              <NodeSelectorDialog value={selected} onChange={setSelected} />
              <label className="text-md font-normal">
                {t("common.selected", { count: selected.length })}
              </label>
            </div>
            <label className="flex min-h-10 items-center gap-2 text-sm font-normal">
              <Checkbox
                checked={defaultOn}
                onCheckedChange={(checked) => setDefaultOn(!!checked)}
              />
              <span>{t("ping.default_on")}</span>
            </label>
            <label className="text-sm font-normal text-gray-500">
              {t("ping.default_on_description")}
            </label>
            <label htmlFor="interval">
              {t("ping.interval")} ({t("time.minute")})
            </label>
            <TextField.Root
              id="interval"
              name="interval"
              defaultValue={15}
              type="number"
              placeholder="15"
            />
            <div className="flex justify-end gap-2">
              <Dialog.Close>
                <Button variant="soft">{t("common.close")}</Button>
              </Dialog.Close>
              <Button disabled={saving} type="submit">
                {t("common.add")}
              </Button>
            </div>
          </Flex>
        </form>
      </AppDialogContent>
    </Dialog.Root>
  );
};

export default LoadPage;

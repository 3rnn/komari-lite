import AppDialogContent from "@/components/AppDialogContent";
import { Checkbox } from "@/components/ui/checkbox";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import { AdminSelectionCount } from "@/components/admin/AdminSelectionCount";
import {
  AdminPagination,
  useAdminPagination,
} from "@/components/admin/AdminPagination";
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
  OfflineNotificationProvider,
  useOfflineNotification,
  type OfflineNotification,
} from "@/contexts/NotificationContext";
import React from "react";
import { Pencil, Search, Settings2 } from "lucide-react";
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
import Tips from "@/components/ui/tips";

const OfflinePage = () => {
  return (
    <OfflineNotificationProvider>
      <NodeDetailsProvider>
        <InnerLayout />
      </NodeDetailsProvider>
    </OfflineNotificationProvider>
  );
};
const NotificationEditForm = ({
  initialValues,
  onSubmit,
  loading,
  onCancel,
  statusLabel,
}: {
  initialValues: { enable: boolean; cooldown: number; grace_period: number };
  onSubmit: (values: {
    enable: boolean;
    cooldown: number;
    grace_period: number;
  }) => void;
  loading?: boolean;
  onCancel?: () => void;
  statusLabel?: string;
}) => {
  const { t } = useTranslation();
  const formId = React.useId();
  const [enabled, setEnabled] = React.useState(initialValues.enable);
  // const [cooldown, setCooldown] = React.useState(initialValues.cooldown);
  const [grace, setGrace] = React.useState(initialValues.grace_period);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit({ enable: enabled, cooldown: 3000, grace_period: grace });
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
      {/* <label htmlFor="cooldown">{t("notification.offline.cooldown")}</label>
      <TextField.Root
        type="number"
        min={1}
        value={cooldown}
        onChange={e => setCooldown(Number(e.target.value))}
        id="cooldown"
        name="cooldown"
      /> */}
      <div className="grid gap-2">
        <label
          htmlFor={`${formId}-grace-period`}
          className="flex items-center gap-2 font-medium"
        >
          {t("notification.offline.grace_period")}
          <Tips>{t("notification.offline.grace_period_tip")}</Tips>
        </label>
        <TextField.Root
          type="number"
          min={0}
          value={grace}
          onChange={(e) => setGrace(Number(e.target.value))}
          id={`${formId}-grace-period`}
          name="grace_period"
        />
      </div>
      <Flex gap="2" justify="end" className="mt-2">
        {onCancel && (
          <Dialog.Close>
            <Button
              variant="soft"
              color="gray"
              type="button"
              onClick={onCancel}
            >
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

const InnerLayout = () => {
  const [search, setSearch] = React.useState("");
  const [selected, setSelected] = React.useState<string[]>([]);
  const {
    loading: onLoading,
    error: onError,
    offlineNotification,
    refresh,
  } = useOfflineNotification();
  const {
    nodeDetail,
    isLoading: onNodeLoading,
    error: onNodeError,
  } = useNodeDetails();
  const { t } = useTranslation();
  const [batchLoading, setBatchLoading] = React.useState(false);
  const [batchDialogOpen, setBatchDialogOpen] = React.useState(false);
  const [defaultDialogOpen, setDefaultDialogOpen] = React.useState(false);
  const [defaultSaving, setDefaultSaving] = React.useState(false);
  const [defaultConfig, setDefaultConfig] = React.useState({
    enabled: false,
    grace_period: 180,
  });
  const [defaultForm, setDefaultForm] = React.useState({
    enable: false,
    cooldown: 1800,
    grace_period: 180,
  });
  const [batchForm, setBatchForm] = React.useState({
    enable: true,
    cooldown: 1800,
    grace_period: 300,
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
    fetch("/api/admin/notification/offline/default", { cache: "no-store" })
      .then(async (response) => {
        const data = await response.json().catch(() => null);
        if (!response.ok) {
          throw new Error(data?.message || response.statusText || "Request failed");
        }
        return data?.data;
      })
      .then((value) => {
        if (cancelled) return;
        setDefaultConfig({
          enabled: value?.enabled === true,
          grace_period: Number(value?.grace_period) || 180,
        });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  const saveDefaultConfig = (values: {
    enable: boolean;
    grace_period: number;
  }) => {
    const next = {
      enabled: values.enable,
      grace_period: values.grace_period,
    };
    setDefaultSaving(true);
    fetch("/api/admin/notification/offline/default", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(next),
    })
      .then(async (response) => {
        const data = await response.json().catch(() => null);
        if (!response.ok) {
          throw new Error(data?.message || response.statusText || "Request failed");
        }
        setDefaultConfig(next);
        toast.success(t("common.updated_successfully"));
        setDefaultDialogOpen(false);
      })
      .catch((error) => {
        toast.error(error instanceof Error ? error.message : String(error));
      })
      .finally(() => setDefaultSaving(false));
  };

  // 批量修改
  const handleBatchEdit = (values: {
    enable: boolean;
    cooldown: number;
    grace_period: number;
  }) => {
    setBatchLoading(true);
    const payload = selectedFilteredIds.map((id) => ({
      client: id,
      enable: values.enable,
      cooldown: values.cooldown,
      grace_period: values.grace_period,
    }));
    fetch("/api/admin/notification/offline/edit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    })
      .then((res) => {
        if (!res.ok) {
          toast.error(
            "Failed to update offline notifications: " + res.statusText
          );
        } else {
          toast.success(t("common.updated_successfully"));
        }
        return res.json();
      })
      .then(() => {
        setBatchLoading(false);
        setBatchDialogOpen(false);
        refresh();
      })
      .catch((error) => {
        console.error("Error updating offline notifications:", error);
        toast.error(t("common.error", { message: error.message }));
        setBatchLoading(false);
      });
  };

  if (onLoading || onNodeLoading) {
    return <Loading text="(o゜▽゜)o☆" />;
  }
  if (onError || onNodeError) {
    return <div>Error: {onError?.message || onNodeError}</div>;
  }
  return (
    <div className="flex flex-col gap-4 p-0 md:p-4">
      <AdminPageTitle
        description={t(
          "notification.offline.description",
          "按节点设置离线宽限期与冷却时间，减少短暂断连造成的重复通知。",
        )}
      >
        {t("notification.offline.full_title", "离线通知设置")}
      </AdminPageTitle>
      <div className="flex flex-col gap-3">
        <OfflineNotificationTable
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
                    const first = offlineNotification.find(
                      (n) => n.client === selectedFilteredIds[0]
                    );
                    setBatchForm({
                      enable: first?.enable ?? true,
                      cooldown: first?.cooldown ?? 1800,
                      grace_period: first?.grace_period ?? 300,
                    });
                  }}
                  disabled={batchLoading || selectedFilteredCount === 0}
                >
                  {t("notification.offline.batch_edit")}
                </Button>
              </Dialog.Trigger>
              <AppDialogContent>
                <Dialog.Title>{t("notification.offline.batch_edit")}</Dialog.Title>
                <NotificationEditForm
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
                      cooldown: 1800,
                      grace_period: defaultConfig.grace_period,
                    });
                  }}
                >
                  <Settings2 size={16} />
                  {t("notification.offline.default_config")}
                </Button>
              </Dialog.Trigger>
              <AppDialogContent
                title={t("notification.offline.default_config")}
                description={t("notification.offline.default_config_description")}
                maxWidth="560px"
              >
                <NotificationEditForm
                  initialValues={defaultForm}
                  loading={defaultSaving}
                  statusLabel={t("notification.offline.default_config_enabled")}
                  onSubmit={saveDefaultConfig}
                  onCancel={() => setDefaultDialogOpen(false)}
                />
              </AppDialogContent>
            </Dialog.Root>
          </div>
        </div>
      </div>
      <div className="border-l-2 border-[var(--accent-8)] pl-3 text-sm leading-6 text-muted-foreground">
        <span
          dangerouslySetInnerHTML={{ __html: t("notification.offline.tips") }}
        />
      </div>
    </div>
  );
};

const OfflineNotificationTable = ({
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
  const { offlineNotification } = useOfflineNotification();
  const { nodeDetail } = useNodeDetails();
  const { t } = useTranslation();
  const filtered = [...nodeDetail]
    .sort((a, b) => a.weight - b.weight)
    .filter((node) => node.name.toLowerCase().includes(search.toLowerCase()));
  const { page, setPage, pageItems, pageSize, setPageSize } =
    useAdminPagination(filtered);
  React.useEffect(() => setPage(1), [search, setPage]);
  return (
    <div className="admin-responsive-table-wrap overflow-hidden rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)]">
      <div className="overflow-x-auto">
      <Table className="admin-responsive-table admin-selection-table min-w-[720px]">
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 px-3 text-center">
              <span className="sr-only">{t("common.select")}</span>
            </TableHead>
            <TableHead>{t("common.server")}</TableHead>
            <TableHead>{t("common.status")}</TableHead>
            {/* <TableHead>{t("notification.offline.cooldown")}</TableHead> */}
            <TableHead>{t("notification.offline.grace_period")}</TableHead>
            <TableHead>{t("notification.offline.last_notified")}</TableHead>
            <TableHead className="text-center">{t("common.action")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageItems.map((node) => (
            <TableRow key={node.uuid}>
              <TableCell className="w-12 px-3" data-label={t("common.select")}>
                <div className="flex items-center justify-center">
                  <Checkbox
                    checked={selected.includes(node.uuid)}
                    aria-label={`${t("common.select")} ${node.name}`}
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
                <Badge
                  color={
                    offlineNotification.find((n) => n.client === node.uuid)
                      ?.enable
                      ? "green"
                      : "red"
                  }
                >
                  {offlineNotification.find((n) => n.client === node.uuid)
                    ?.enable
                    ? t("common.enabled")
                    : t("common.disabled")}
                </Badge>
              </TableCell>
              {/* <TableCell>
                {offlineNotification.find((n) => n.client === node.uuid)
                  ?.cooldown || 1800}{" "}
                {t("nodeCard.time_second")}
              </TableCell> */}
              <TableCell data-label={t("notification.offline.grace_period")}>
                {offlineNotification.find((n) => n.client === node.uuid)
                  ?.grace_period || 300}
                {t("nodeCard.time_second")}
              </TableCell>
              <TableCell data-label={t("notification.offline.last_notified")}>
                {(() => {
                  const lastNotified = offlineNotification.find(
                    (n) => n.client === node.uuid
                  )?.last_notified;
                  if (!lastNotified) return "-";
                  const date = new Date(lastNotified);
                  if (date.getFullYear() < 3)
                    return t("notification.offline.never_triggered");
                  return date.toLocaleString();
                })()}
              </TableCell>
              <TableCell className="text-center" data-label={t("common.action")}>
                <ActionButtons
                  offlineNotifications={offlineNotification.find(
                    (n) => n.client === node.uuid
                  )}
                />
              </TableCell>
            </TableRow>
          ))}
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
  offlineNotifications,
}: {
  offlineNotifications: OfflineNotification | undefined;
}) => {
  const { t } = useTranslation();
  const { refresh } = useOfflineNotification();
  const [editOpen, setEditOpen] = React.useState(false);
  const [editSaving, setEditSaving] = React.useState(false);

  return (
    <Flex gap="3" align="center" className="admin-card-actions admin-single-text-action w-full">
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
        <AppDialogContent>
          <Dialog.Title>{t("common.edit")}</Dialog.Title>
          <NotificationEditForm
            initialValues={{
              enable: offlineNotifications?.enable ?? false,
              cooldown: offlineNotifications?.cooldown ?? 1800,
              grace_period: offlineNotifications?.grace_period ?? 300,
            }}
            loading={editSaving}
            onSubmit={(values) => {
              setEditSaving(true);
              fetch("/api/admin/notification/offline/edit", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify([
                  {
                    client: offlineNotifications?.client,
                    ...values,
                  },
                ]),
              })
                .then((res) => {
                  if (!res.ok) {
                    toast.error(
                      "Failed to save offline notification settings: " +
                        res.statusText
                    );
                  }
                  toast.success(t("common.updated_successfully"));
                  return res.json();
                })
                .then(() => {
                  setEditOpen(false);
                  refresh();
                  setEditSaving(false);
                })
                .catch((error) => {
                  console.error(
                    "Error saving offline notification settings:",
                    error
                  );
                  toast.error(t("common.error", { message: error.message }));
                });
            }}
            onCancel={() => setEditOpen(false)}
          />
        </AppDialogContent>
      </Dialog.Root>
    </Flex>
  );
};

export default OfflinePage;

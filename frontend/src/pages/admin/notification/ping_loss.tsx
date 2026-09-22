import AppDialogContent from "@/components/AppDialogContent";
import Loading from "@/components/loading";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import AdminActiveFilter from "@/components/admin/AdminActiveFilter";
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
  type NodeDetail,
} from "@/contexts/NodeDetailsContext";
import {
  PingTaskProvider,
  usePingTask,
  type PingTask,
} from "@/contexts/PingTaskContext";
import {
  Badge,
  Box,
  Button,
  Dialog,
  Flex,
  IconButton,
  Select,
  Switch,
  Tabs,
  TextField,
} from "@radix-ui/themes";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Pencil,
  Plus,
  Search,
  Settings2,
  SlidersHorizontal,
  Trash2,
} from "lucide-react";
import React from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { toast } from "sonner";

type PingLossNotification = {
  id: number;
  client: string;
  task_id: number;
  enable: boolean;
  window_seconds: number;
  loss_threshold: number;
  minimum_samples: number;
  cooldown_seconds: number;
  last_notified?: string | null;
  alert_active?: boolean;
  task?: PingTask;
};

type AlertTarget = {
  key: string;
  client: string;
  clientName: string;
  serverOrder: number;
  taskId: number;
  task: PingTask;
  rule?: PingLossNotification;
};

type FormState = {
  enable: boolean;
  windowMinutes: number;
  lossThreshold: number;
  minimumSamples: number;
  cooldownMinutes: number;
};

type ViewMode = "task" | "server";

const defaultForm: FormState = {
  enable: true,
  windowMinutes: 1,
  lossThreshold: 5,
  minimumSamples: 1,
  cooldownMinutes: 5,
};

const isPingLossFormValid = (form: FormState) =>
  form.windowMinutes >= 1 &&
  form.windowMinutes <= 1440 &&
  form.lossThreshold > 0 &&
  form.lossThreshold <= 100 &&
  form.minimumSamples >= 1 &&
  form.minimumSamples <= 100000 &&
  form.cooldownMinutes >= 1 &&
  form.cooldownMinutes <= 10080;

const targetKey = (client: string, taskId: number) => `${client}:${taskId}`;

const parseResponse = async (response: Response) => {
  const data = await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(data?.message || "Request failed");
  }
  return data;
};

const buildAlertTargets = (
  tasks: PingTask[],
  nodes: NodeDetail[],
  rules: PingLossNotification[],
) => {
  const nodesById = new Map(
    nodes.map((node, index) => [node.uuid, { node, order: index }])
  );
  const tasksById = new Map(
    tasks
      .filter((task) => typeof task.id === "number")
      .map((task) => [task.id as number, task]),
  );
  const rulesByTarget = new Map<string, PingLossNotification>();
  for (const rule of rules) {
    const key = targetKey(rule.client, rule.task_id);
    if (!rulesByTarget.has(key)) rulesByTarget.set(key, rule);
  }

  const targets: AlertTarget[] = [];
  const seen = new Set<string>();
  for (const task of tasks) {
    if (typeof task.id !== "number") continue;
    for (const client of new Set(task.clients || [])) {
      const nodeEntry = nodesById.get(client);
      if (!nodeEntry) continue;
      const key = targetKey(client, task.id);
      seen.add(key);
      targets.push({
        key,
        client,
        clientName: nodeEntry.node.name || client,
        serverOrder: nodeEntry.order,
        taskId: task.id,
        task,
        rule: rulesByTarget.get(key),
      });
    }
  }

  for (const rule of rules) {
    const key = targetKey(rule.client, rule.task_id);
    if (seen.has(key)) continue;
    const nodeEntry = nodesById.get(rule.client);
    const task = tasksById.get(rule.task_id) || rule.task || {
      id: rule.task_id,
      name: `#${rule.task_id}`,
    };
    targets.push({
      key,
      client: rule.client,
      clientName: nodeEntry?.node.name || rule.client,
      serverOrder: nodeEntry?.order ?? Number.MAX_SAFE_INTEGER,
      taskId: rule.task_id,
      task,
      rule,
    });
  }

  return targets;
};

const sortTargets = (targets: AlertTarget[], view: ViewMode) => {
  const compareText = (a: string, b: string) =>
    a.localeCompare(b, undefined, { numeric: true, sensitivity: "base" });
  return [...targets].sort((a, b) => {
    if (view === "task") {
      const taskWeight = (a.task.weight ?? 0) - (b.task.weight ?? 0);
      if (taskWeight !== 0) return taskWeight;
      const taskName = compareText(a.task.name || "", b.task.name || "");
      if (taskName !== 0) return taskName;
      if (a.serverOrder !== b.serverOrder) {
        return a.serverOrder - b.serverOrder;
      }
      return compareText(a.clientName, b.clientName);
    }
    if (a.serverOrder !== b.serverOrder) {
      return a.serverOrder - b.serverOrder;
    }
    const serverName = compareText(a.clientName, b.clientName);
    if (serverName !== 0) return serverName;
    const taskWeight = (a.task.weight ?? 0) - (b.task.weight ?? 0);
    if (taskWeight !== 0) return taskWeight;
    return compareText(a.task.name || "", b.task.name || "");
  });
};

const PingLossPage = () => (
  <PingTaskProvider>
    <NodeDetailsProvider>
      <PingLossContent />
    </NodeDetailsProvider>
  </PingTaskProvider>
);

const PingLossContent = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const { nodeDetail, isLoading: nodesLoading, error: nodesError } =
    useNodeDetails();
  const { pingTasks, isLoading: tasksLoading, error: tasksError } =
    usePingTask();
  const [rules, setRules] = React.useState<PingLossNotification[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [search, setSearch] = React.useState("");
  const [view, setView] = React.useState<ViewMode>("task");
  const [selected, setSelected] = React.useState<string[]>([]);
  const routeState = searchParams.get("state")?.trim() || "";
  const routeNode = searchParams.get("node")?.trim() || "";
  const routeTask = Number(searchParams.get("task") || 0);

  const refresh = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch("/api/admin/notification/ping-loss/");
      const data = await parseResponse(response);
      setRules(Array.isArray(data?.data) ? data.data : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const targets = React.useMemo(
    () => buildAlertTargets(pingTasks || [], nodeDetail, rules),
    [pingTasks, nodeDetail, rules],
  );
  const routeFilteredTargets = React.useMemo(
    () => targets.filter((target) => (
      (!routeState || routeState !== "active" || target.rule?.alert_active === true)
      && (!routeNode || target.client === routeNode)
      && (!routeTask || target.taskId === routeTask)
    )),
    [routeNode, routeState, routeTask, targets],
  );

  React.useEffect(() => {
    const validKeys = new Set(targets.map((target) => target.key));
    setSelected((current) => current.filter((key) => validKeys.has(key)));
  }, [targets]);

  const filteredTargets = React.useMemo(() => {
    const keyword = search.trim().toLowerCase();
    const filtered = keyword
      ? routeFilteredTargets.filter((target) =>
          [
            target.clientName,
            target.client,
            target.task.name,
            target.task.target,
            target.task.type,
          ].some((value) => String(value || "").toLowerCase().includes(keyword)),
        )
      : routeFilteredTargets;
    return sortTargets(filtered, view);
  }, [routeFilteredTargets, search, view]);

  const activeFilterLabel = React.useMemo(() => {
    if (routeNode || routeTask) {
      const target = targets.find((item) => (
        (!routeNode || item.client === routeNode) && (!routeTask || item.taskId === routeTask)
      ));
      return [target?.clientName || routeNode, target?.task.name || (routeTask ? `#${routeTask}` : "")]
        .filter(Boolean)
        .join(" · ");
    }
    if (routeState === "active") {
      return t("admin_dashboard.alert_latency_loss");
    }
    return "";
  }, [routeNode, routeState, routeTask, t, targets]);

  const selectedTargets = React.useMemo(() => {
    const selectedSet = new Set(selected);
    return filteredTargets.filter((target) => selectedSet.has(target.key));
  }, [filteredTargets, selected]);
  const selectedFilteredCount = React.useMemo(() => {
    const selectedSet = new Set(selected);
    return filteredTargets.filter((target) => selectedSet.has(target.key)).length;
  }, [filteredTargets, selected]);
  const allFilteredSelected =
    filteredTargets.length > 0 && selectedFilteredCount === filteredTargets.length;
  const availableTargets = React.useMemo(
    () => sortTargets(targets.filter((target) => !target.rule), "task"),
    [targets],
  );

  if (loading || nodesLoading || tasksLoading) {
    return <Loading text={t("loading")} />;
  }
  if (error || nodesError || tasksError) {
    return (
      <div>
        {t("common.error")}: {error || nodesError || tasksError}
      </div>
    );
  }

  const handleBatchSaved = async () => {
    setSelected([]);
    await refresh();
  };

  const toggleSelectAll = () => {
    const filteredKeys = new Set(filteredTargets.map((target) => target.key));
    setSelected((current) =>
      allFilteredSelected
        ? current.filter((key) => !filteredKeys.has(key))
        : Array.from(new Set([...current, ...filteredKeys])),
    );
  };

  return (
    <div className="flex w-full min-w-0 max-w-full flex-col gap-4 p-0 md:p-4">
      <Flex className="w-full" justify="between" align="center" gap="3" wrap="wrap">
        <AdminPageTitle
          description={t(
            "notification.ping_loss.description",
            "根据延迟监测结果设置丢包阈值、统计窗口和冷却时间。",
          )}
        >
          {t("notification.ping_loss.full_title")}
        </AdminPageTitle>
      </Flex>

      {activeFilterLabel ? (
        <AdminActiveFilter label={activeFilterLabel} clearTo="/admin/notification/ping-loss" />
      ) : null}

      <Tabs.Root value={view} onValueChange={(value) => setView(value as ViewMode)}>
        <div className="w-full overflow-x-auto pb-1">
          <Tabs.List className="w-max min-w-full">
            <Tabs.Trigger value="task" className="min-w-[8rem] flex-1">
              {t("ping.task_view")}
            </Tabs.Trigger>
            <Tabs.Trigger value="server" className="min-w-[8rem] flex-1">
              {t("ping.server_view")}
            </Tabs.Trigger>
          </Tabs.List>
        </div>
        <div className="flex flex-col gap-3 pt-3">
          <Box>
            <Tabs.Content value="task">
              <AlertTable
                view="task"
                targets={filteredTargets}
                selected={selected}
                onSelectionChange={setSelected}
                onSaved={refresh}
                paginationSummary={
                  <AdminSelectionCount
                    count={selectedFilteredCount}
                    total={filteredTargets.length}
                    className="hidden md:inline-flex"
                  />
                }
              />
            </Tabs.Content>
            <Tabs.Content value="server">
              <AlertTable
                view="server"
                targets={filteredTargets}
                selected={selected}
                onSelectionChange={setSelected}
                onSaved={refresh}
                paginationSummary={
                  <AdminSelectionCount
                    count={selectedFilteredCount}
                    total={filteredTargets.length}
                    className="hidden md:inline-flex"
                  />
                }
              />
            </Tabs.Content>
          </Box>
          <div className="order-first flex min-w-0 flex-col gap-2 px-1 md:ml-auto md:w-fit md:self-end md:flex-row md:items-center md:justify-end">
            <AdminSelectionCount
              count={selectedFilteredCount}
              total={filteredTargets.length}
              className="order-last shrink-0 self-start text-sm text-muted-foreground md:hidden"
            />
            <div className="flex w-full min-w-0 flex-wrap items-center gap-2 md:w-auto md:justify-end">
              <Button
                type="button"
                variant="soft"
                className="shrink-0"
                disabled={filteredTargets.length === 0}
                onClick={toggleSelectAll}
              >
                {t(allFilteredSelected ? "common.deselect_all" : "common.select_all")}
              </Button>
              <ConfigurationDialog
                targets={selectedTargets}
                onSaved={handleBatchSaved}
                batch
              >
                <Button className="shrink-0" disabled={selectedTargets.length === 0}>
                  <SlidersHorizontal size={16} />
                  {t("notification.ping_loss.batch_edit")}
                </Button>
              </ConfigurationDialog>
              <TextField.Root
                className="order-last min-w-0 w-full basis-full sm:order-none sm:w-64 sm:basis-auto sm:flex-none"
                value={search}
                placeholder={t("common.search")}
                onChange={(event) => setSearch(event.target.value)}
              >
                <TextField.Slot>
                  <Search size={16} />
                </TextField.Slot>
              </TextField.Root>
              <div className="shrink-0">
                <PingLossDefaultDialog />
              </div>
              <ConfigurationDialog
                targets={[]}
                availableTargets={availableTargets}
                onSaved={refresh}
              >
                <Button className="shrink-0">
                  <Plus size={16} />
                  {t("common.add")}
                </Button>
              </ConfigurationDialog>
            </div>
          </div>
        </div>
      </Tabs.Root>
    </div>
  );
};

const AlertTable = ({
  view,
  targets,
  selected,
  onSelectionChange,
  onSaved,
  paginationSummary,
}: {
  view: ViewMode;
  targets: AlertTarget[];
  selected: string[];
  onSelectionChange: (keys: string[]) => void;
  onSaved: () => Promise<void>;
  paginationSummary?: React.ReactNode;
}) => {
  const { t } = useTranslation();
  const selectedSet = new Set(selected);
  const { page, setPage, pageItems, pageSize, setPageSize } =
    useAdminPagination(targets);
  return (
    <div className="admin-responsive-table-wrap w-full min-w-0 max-w-full overflow-hidden rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)]">
      <div className="overflow-x-auto">
      <Table className="admin-responsive-table admin-selection-table min-w-[1120px]">
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 px-3 text-center">
              <span className="sr-only">{t("common.select")}</span>
            </TableHead>
            <TableHead>
              {view === "task" ? t("ping.task") : t("common.server")}
            </TableHead>
            <TableHead>
              {view === "task" ? t("common.server") : t("ping.task")}
            </TableHead>
            <TableHead>{t("ping.target")}</TableHead>
            <TableHead>{t("common.status")}</TableHead>
            <TableHead>{t("notification.ping_loss.window")}</TableHead>
            <TableHead>{t("notification.ping_loss.threshold")}</TableHead>
            <TableHead>{t("notification.ping_loss.minimum_samples")}</TableHead>
            <TableHead>{t("notification.ping_loss.cooldown")}</TableHead>
            <TableHead>{t("notification.ping_loss.last_notified")}</TableHead>
            <TableHead className="text-center">{t("common.action")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {targets.length === 0 ? (
            <TableRow>
              <TableCell colSpan={11} className="py-8 text-center text-gray-500">
                {t("notification.ping_loss.empty")}
              </TableCell>
            </TableRow>
          ) : (
            pageItems.map((target) => (
              <AlertRow
                key={target.key}
                view={view}
                target={target}
                selected={selectedSet.has(target.key)}
                onSelectedChange={(checked) =>
                  onSelectionChange(
                    checked
                      ? Array.from(new Set([...selected, target.key]))
                      : selected.filter((key) => key !== target.key),
                  )
                }
                onSaved={onSaved}
              />
            ))
          )}
        </TableBody>
      </Table>
      </div>
      <AdminPagination
        page={page}
        total={targets.length}
        pageSize={pageSize}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
        summary={paginationSummary}
      />
    </div>
  );
};

const AlertRow = ({
  view,
  target,
  selected,
  onSelectedChange,
  onSaved,
}: {
  view: ViewMode;
  target: AlertTarget;
  selected: boolean;
  onSelectedChange: (checked: boolean) => void;
  onSaved: () => Promise<void>;
}) => {
  const { t } = useTranslation();
  const rule = target.rule;
  const taskName = target.task.name || `#${target.taskId}`;
  const primary = view === "task" ? taskName : target.clientName;
  const secondary = view === "task" ? target.clientName : taskName;

  return (
    <TableRow data-state={selected ? "selected" : undefined}>
      <TableCell className="w-12 px-3" data-label={t("common.select")}>
        <div className="flex items-center justify-center">
          <Checkbox
            checked={selected}
            aria-label={`${primary} - ${secondary}`}
            onCheckedChange={(checked) => onSelectedChange(checked === true)}
          />
        </div>
      </TableCell>
      <TableCell data-label={view === "task" ? t("ping.task") : t("common.server")}>{primary}</TableCell>
      <TableCell data-label={view === "task" ? t("common.server") : t("ping.task")}>{secondary}</TableCell>
      <TableCell data-label={t("ping.target")}>{target.task.target || "-"}</TableCell>
      <TableCell data-label={t("common.status")}>
        {rule ? (
          <Badge color={rule.enable ? "green" : "gray"}>
            {rule.enable ? t("common.enabled") : t("common.disabled")}
          </Badge>
        ) : (
          <Badge color="orange">{t("notification.ping_loss.not_configured")}</Badge>
        )}
      </TableCell>
      <TableCell data-label={t("notification.ping_loss.window")}>
        {rule
          ? t("notification.ping_loss.minutes", {
              count: rule.window_seconds / 60,
            })
          : "-"}
      </TableCell>
      <TableCell data-label={t("notification.ping_loss.threshold")}>{rule ? `${rule.loss_threshold.toFixed(1)}%` : "-"}</TableCell>
      <TableCell data-label={t("notification.ping_loss.minimum_samples")}>{rule?.minimum_samples ?? "-"}</TableCell>
      <TableCell data-label={t("notification.ping_loss.cooldown")}>
        {rule
          ? t("notification.ping_loss.minutes", {
              count: rule.cooldown_seconds / 60,
            })
          : "-"}
      </TableCell>
      <TableCell data-label={t("notification.ping_loss.last_notified")}>
        {rule?.last_notified
          ? new Date(rule.last_notified).toLocaleString()
          : t("notification.ping_loss.never")}
      </TableCell>
      <TableCell className="text-center" data-label={t("common.action")}>
        <Flex gap="3" align="center" className="admin-card-actions admin-ping-loss-actions w-full">
          <ConfigurationDialog targets={[target]} onSaved={onSaved}>
            <IconButton
              variant="ghost"
              title={rule ? t("common.edit") : t("notification.ping_loss.add")}
              aria-label={rule ? t("common.edit") : t("notification.ping_loss.add")}
            >
              <Pencil size={16} />
            </IconButton>
          </ConfigurationDialog>
          {rule ? <DeleteRuleButton rule={rule} onDeleted={onSaved} /> : null}
        </Flex>
      </TableCell>
    </TableRow>
  );
};

const PingLossConfigurationFields = ({
  form,
  onChange,
  enableLabel,
}: {
  form: FormState;
  onChange: React.Dispatch<React.SetStateAction<FormState>>;
  enableLabel?: string;
}) => {
  const { t } = useTranslation();
  const enableId = React.useId();
  return (
    <>
      <Flex justify="between" align="center">
        <label htmlFor={enableId}>{enableLabel ?? t("common.status")}</label>
        <Switch
          id={enableId}
          checked={form.enable}
          onCheckedChange={(enable) =>
            onChange((current) => ({ ...current, enable }))
          }
        />
      </Flex>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <NumberField
          label={t("notification.ping_loss.window_minutes")}
          value={form.windowMinutes}
          min={1}
          max={1440}
          onChange={(windowMinutes) =>
            onChange((current) => ({ ...current, windowMinutes }))
          }
        />
        <NumberField
          label={`${t("notification.ping_loss.threshold")} (%)`}
          value={form.lossThreshold}
          min={0.1}
          max={100}
          step={0.1}
          onChange={(lossThreshold) =>
            onChange((current) => ({ ...current, lossThreshold }))
          }
        />
        <NumberField
          label={t("notification.ping_loss.minimum_samples")}
          value={form.minimumSamples}
          min={1}
          max={100000}
          onChange={(minimumSamples) =>
            onChange((current) => ({ ...current, minimumSamples }))
          }
        />
        <NumberField
          label={t("notification.ping_loss.cooldown_minutes")}
          value={form.cooldownMinutes}
          min={1}
          max={10080}
          onChange={(cooldownMinutes) =>
            onChange((current) => ({ ...current, cooldownMinutes }))
          }
        />
      </div>
    </>
  );
};

const PingLossDefaultDialog = () => {
  const { t } = useTranslation();
  const [open, setOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [cached, setCached] = React.useState<FormState>(defaultForm);
  const [form, setForm] = React.useState<FormState>(defaultForm);

  React.useEffect(() => {
    let cancelled = false;
    fetch("/api/admin/notification/ping-loss/default", { cache: "no-store" })
      .then(parseResponse)
      .then((data) => {
        if (cancelled) return;
        const value = data?.data;
        setCached({
          enable: value?.enabled === true,
          windowMinutes: (Number(value?.window_seconds) || 60) / 60,
          lossThreshold: Number(value?.loss_threshold) || 5,
          minimumSamples: Number(value?.minimum_samples) || 1,
          cooldownMinutes: (Number(value?.cooldown_seconds) || 300) / 60,
        });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!isPingLossFormValid(form)) {
      toast.error(t("notification.ping_loss.invalid_form"));
      return;
    }
    setSaving(true);
    try {
      const response = await fetch("/api/admin/notification/ping-loss/default", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          enabled: form.enable,
          window_seconds: Math.round(form.windowMinutes * 60),
          loss_threshold: form.lossThreshold,
          minimum_samples: Math.round(form.minimumSamples),
          cooldown_seconds: Math.round(form.cooldownMinutes * 60),
        }),
      });
      await parseResponse(response);
      toast.success(t("common.updated_successfully"));
      setCached(form);
      setOpen(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger>
        <Button
          type="button"
          variant="soft"
          className="shrink-0"
          onClick={() => setForm(cached)}
        >
          <Settings2 size={16} />
          {t("notification.ping_loss.default_config")}
        </Button>
      </Dialog.Trigger>
      <AppDialogContent
        title={t("notification.ping_loss.default_config")}
        description={t("notification.ping_loss.default_config_description")}
        maxWidth="560px"
      >
        <form onSubmit={save} className="mt-4 flex flex-col gap-4">
          <PingLossConfigurationFields
            form={form}
            onChange={setForm}
            enableLabel={t("notification.ping_loss.default_config_enabled")}
          />
          <Flex gap="2" justify="end" className="mt-2">
            <Dialog.Close>
              <Button type="button" variant="soft" color="gray">
                {t("common.cancel")}
              </Button>
            </Dialog.Close>
            <Button type="submit" loading={saving}>
              {t("common.save")}
            </Button>
          </Flex>
        </form>
      </AppDialogContent>
    </Dialog.Root>
  );
};

const ConfigurationDialog = ({
  children,
  targets,
  availableTargets,
  onSaved,
  batch = false,
}: {
  children: React.ReactNode;
  targets: AlertTarget[];
  availableTargets?: AlertTarget[];
  onSaved: () => Promise<void>;
  batch?: boolean;
}) => {
  const { t } = useTranslation();
  const [open, setOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<FormState>(defaultForm);
  const [createTargetKey, setCreateTargetKey] = React.useState("");
  const createMode = availableTargets !== undefined;
  const availableTargetSignature = (availableTargets || [])
    .map((target) => target.key)
    .join("|");
  const activeTargets = createMode
    ? (availableTargets || []).filter(
        (target) => target.key === createTargetKey,
      )
    : targets;
  const targetSignature = activeTargets.map((target) => target.key).join("|");

  React.useEffect(() => {
    if (!open || !createMode) return;
    setCreateTargetKey((current) =>
      (availableTargets || []).some((target) => target.key === current)
        ? current
        : availableTargets?.[0]?.key || "",
    );
  }, [open, createMode, availableTargetSignature]);

  React.useEffect(() => {
    if (!open) return;
    const rule = activeTargets.find((target) => target.rule)?.rule;
    setForm(
      rule
        ? {
            enable: rule.enable,
            windowMinutes: rule.window_seconds / 60,
            lossThreshold: rule.loss_threshold,
            minimumSamples: rule.minimum_samples,
            cooldownMinutes: rule.cooldown_seconds / 60,
          }
        : defaultForm,
    );
  }, [open, targetSignature]);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (activeTargets.length === 0) {
      toast.error(t("notification.ping_loss.select_required"));
      return;
    }
    if (!isPingLossFormValid(form)) {
      toast.error(t("notification.ping_loss.invalid_form"));
      return;
    }

    const notifications = activeTargets.map((target) => ({
      ...(target.rule ? { id: target.rule.id } : {}),
      client: target.client,
      task_id: target.taskId,
      enable: form.enable,
      window_seconds: Math.round(form.windowMinutes * 60),
      loss_threshold: form.lossThreshold,
      minimum_samples: Math.round(form.minimumSamples),
      cooldown_seconds: Math.round(form.cooldownMinutes * 60),
    }));

    setSaving(true);
    try {
      const response = await fetch("/api/admin/notification/ping-loss/batch", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ notifications }),
      });
      await parseResponse(response);
      toast.success(t("common.updated_successfully"));
      setOpen(false);
      await onSaved();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("common.error"));
    } finally {
      setSaving(false);
    }
  };

  const firstRule = activeTargets.find((target) => target.rule)?.rule;
  const title = batch
    ? t("notification.ping_loss.batch_edit")
    : firstRule
      ? t("notification.ping_loss.edit")
      : t("notification.ping_loss.add");

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger>{children}</Dialog.Trigger>
      <AppDialogContent maxWidth="560px">
        <Dialog.Title>{title}</Dialog.Title>
        <Dialog.Description className="sr-only">{title}</Dialog.Description>
        {batch ? (
          <span className="text-sm text-muted-foreground">
            {t("common.selected", { count: targets.length })}
          </span>
        ) : null}
        <form onSubmit={submit} className="mt-4 flex flex-col gap-4">
          {createMode ? (
            <Field label={`${t("ping.task")} / ${t("common.server")}`}>
              <Select.Root
                value={createTargetKey}
                onValueChange={setCreateTargetKey}
                disabled={(availableTargets || []).length === 0}
              >
                <Select.Trigger placeholder={t("common.select")} />
                <Select.Content>
                  {(availableTargets || []).map((target) => (
                    <Select.Item key={target.key} value={target.key}>
                      {target.task.name || `#${target.taskId}`} / {target.clientName}
                    </Select.Item>
                  ))}
                </Select.Content>
              </Select.Root>
            </Field>
          ) : null}
          <PingLossConfigurationFields form={form} onChange={setForm} />

          <Flex gap="2" justify="end" className="mt-2">
            <Dialog.Close>
              <Button type="button" variant="soft" color="gray">
                {t("common.cancel")}
              </Button>
            </Dialog.Close>
            <Button type="submit" disabled={saving || activeTargets.length === 0}>
              {t("common.save")}
            </Button>
          </Flex>
        </form>
      </AppDialogContent>
    </Dialog.Root>
  );
};

const Field = ({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) => (
  <label className="flex min-w-0 flex-col gap-2">
    <span>{label}</span>
    {children}
  </label>
);

const NumberField = ({
  label,
  value,
  onChange,
  min,
  max,
  step = 1,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min: number;
  max: number;
  step?: number;
}) => (
  <Field label={label}>
    <TextField.Root
      type="number"
      value={String(value)}
      min={min}
      max={max}
      step={step}
      onChange={(event) => onChange(Number(event.target.value))}
    />
  </Field>
);

const DeleteRuleButton = ({
  rule,
  onDeleted,
}: {
  rule: PingLossNotification;
  onDeleted: () => Promise<void>;
}) => {
  const { t } = useTranslation();
  const [open, setOpen] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);

  const remove = async () => {
    setDeleting(true);
    try {
      const response = await fetch("/api/admin/notification/ping-loss/delete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: [rule.id] }),
      });
      await parseResponse(response);
      toast.success(t("common.deleted_successfully"));
      setOpen(false);
      await onDeleted();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("common.error"));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger>
        <IconButton
          variant="ghost"
          color="red"
          title={t("common.delete")}
          aria-label={t("common.delete")}
        >
          <Trash2 size={16} />
        </IconButton>
      </Dialog.Trigger>
      <AppDialogContent maxWidth="420px">
        <Dialog.Title>{t("notification.ping_loss.delete_title")}</Dialog.Title>
        <Dialog.Description className="sr-only">
          {t("notification.ping_loss.delete_title")}
        </Dialog.Description>
        <Flex gap="2" justify="end" className="mt-6">
          <Dialog.Close>
            <Button type="button" variant="soft" color="gray">
              {t("common.cancel")}
            </Button>
          </Dialog.Close>
          <Button color="red" onClick={remove} disabled={deleting}>
            {t("common.delete")}
          </Button>
        </Flex>
      </AppDialogContent>
    </Dialog.Root>
  );
};

export default PingLossPage;

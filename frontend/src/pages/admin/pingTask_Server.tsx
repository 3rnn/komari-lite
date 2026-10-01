import AppDialogContent from "@/components/AppDialogContent";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useNodeDetails } from "@/contexts/NodeDetailsContext";
import { type PingTask } from "@/contexts/PingTaskContext";
import { assignedDisplayTasks, resolveDisplayPingTaskIds } from "@/utils/pingDisplaySelection";
import { Checkbox } from "@/components/ui/checkbox";
import { Button, Dialog, Flex, IconButton } from "@radix-ui/themes";
import { MoreHorizontal } from "lucide-react";
import React from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  AdminPagination,
  useAdminPagination,
} from "@/components/admin/AdminPagination";

// This tab changes public presentation only. Probe assignments are edited in Probe Setup.
export const ServerView = ({
  pingTasks,
  search,
}: {
  pingTasks: PingTask[];
  search: string;
}) => {
  const { t } = useTranslation();
  const { nodeDetail, refresh } = useNodeDetails();
  const filteredNodes = React.useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return nodeDetail;
    return nodeDetail.filter((node) =>
      [node.name, ...assignedDisplayTasks(node.uuid, pingTasks).map((task) => task.name || task.target)]
        .some((value) => String(value || "").toLowerCase().includes(keyword)),
    );
  }, [nodeDetail, pingTasks, search]);
  const { page, setPage, pageItems, pageSize, setPageSize } =
    useAdminPagination(filteredNodes);

  React.useEffect(() => setPage(1), [search, setPage]);

  return (
    <div className="admin-responsive-table-wrap overflow-hidden rounded-md border border-[var(--gray-a5)]">
      <div className="overflow-x-auto">
        <Table className="admin-responsive-table min-w-[640px]">
          <TableHeader>
            <TableRow>
              <TableHead className="w-48">{t("common.server")}</TableHead>
              <TableHead>{t("ping.assigned_tasks")}</TableHead>
              <TableHead>{t("ping.display_task")}</TableHead>
              <TableHead className="w-16" aria-label={t("common.action")} />
            </TableRow>
          </TableHeader>
          <TableBody>
            {pageItems.map((node) => (
              <ServerRow
                key={node.uuid}
                nodeUuid={node.uuid}
                nodeName={node.name}
                displayPingTaskIDs={node.display_ping_task_ids}
                legacyDisplayPingTaskID={node.display_ping_task_id || 0}
                pingTasks={pingTasks}
                onSaved={refresh}
              />
            ))}
          </TableBody>
        </Table>
      </div>
      <AdminPagination
        page={page}
        total={filteredNodes.length}
        pageSize={pageSize}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
        summary={false}
      />
    </div>
  );
};

const ServerRow: React.FC<{
  nodeUuid: string;
  nodeName: string;
  displayPingTaskIDs?: number[];
  legacyDisplayPingTaskID: number;
  pingTasks: PingTask[];
  onSaved: () => void;
}> = ({ nodeUuid, nodeName, displayPingTaskIDs, legacyDisplayPingTaskID, pingTasks, onSaved }) => {
  const { t } = useTranslation();
  const [open, setOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const assigned = React.useMemo(
    () => assignedDisplayTasks(nodeUuid, pingTasks),
    [nodeUuid, pingTasks],
  );
  const currentIds = resolveDisplayPingTaskIds(nodeUuid, pingTasks, displayPingTaskIDs, legacyDisplayPingTaskID);
  const [selectedIds, setSelectedIds] = React.useState<number[]>(currentIds);
  const displayedTasks = assigned.filter((task) => currentIds.includes(task.id));

  const handleSave = async () => {
    if (selectedIds.length === currentIds.length && selectedIds.every((id) => currentIds.includes(id))) {
      setOpen(false);
      return;
    }
    setSaving(true);
    try {
      const response = await fetch(`/api/admin/client/${nodeUuid}/display-ping-tasks`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ task_ids: selectedIds }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result?.message || t("common.error"));
      }
      toast.success(t("common.updated_successfully"));
      setOpen(false);
      onSaved();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("common.error"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <TableRow>
      <TableCell data-label={t("common.server")}>{nodeName}</TableCell>
      <TableCell data-label={t("ping.assigned_tasks")} className="whitespace-normal break-words">
        {assigned.length ? assigned.map((task) => task.name).join(", ") : t("common.none")}
      </TableCell>
      <TableCell data-label={t("ping.display_task")}>
        {displayedTasks.length ? displayedTasks.map((task) => task.name).join(", ") : t("ping.theme_default")}
      </TableCell>
      <TableCell data-label={t("common.action")}>
        <Dialog.Root open={open} onOpenChange={(next) => {
          if (next) setSelectedIds(currentIds);
          setOpen(next);
        }}>
          <Dialog.Trigger>
            <IconButton variant="ghost" aria-label={t("ping.choose_display_task")}>
              <MoreHorizontal size={16} />
            </IconButton>
          </Dialog.Trigger>
          <AppDialogContent maxWidth="450px">
            <Dialog.Title>{t("ping.choose_display_task")} — {nodeName}</Dialog.Title>
            <p className="mb-3 text-sm text-gray-500">{t("ping.public_display_hint")}</p>
            <div role="group" aria-label={t("ping.display_task")} className="max-h-60 space-y-2 overflow-y-auto">
              {assigned.map((task) => (
                <label key={task.id} className="flex cursor-pointer items-center gap-2 rounded border border-[var(--gray-a5)] p-2">
                  <Checkbox
                    checked={selectedIds.includes(task.id)}
                    onCheckedChange={(checked) => setSelectedIds((previous) => checked
                      ? assigned.filter((item) => item.id === task.id || previous.includes(item.id)).map((item) => item.id)
                      : previous.filter((id) => id !== task.id))}
                  />
                  <span className="min-w-0 break-words text-sm">{task.name} ({task.type}/{task.interval}s)</span>
                </label>
              ))}
              {!assigned.length && <p className="text-sm text-gray-500">{t("common.none")}</p>}
            </div>
            <Button type="button" variant="soft" color="gray" className="mt-3" onClick={() => setSelectedIds([])}>
              {t("ping.theme_default")}
            </Button>
            <Flex gap="2" justify="end" className="mt-4">
              <Button variant="soft" color="gray" type="button" onClick={() => setOpen(false)}>
                {t("common.cancel")}
              </Button>
              <Button onClick={handleSave} disabled={saving}>
                {t("common.save")}
              </Button>
            </Flex>
          </AppDialogContent>
        </Dialog.Root>
      </TableCell>
    </TableRow>
  );
};

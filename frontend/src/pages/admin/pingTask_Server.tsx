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
import { assignedDisplayTasks, resolveDisplayPingTaskId } from "@/utils/pingDisplaySelection";
import { Button, Dialog, Flex, IconButton, Select } from "@radix-ui/themes";
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
                displayPingTaskID={node.display_ping_task_id || 0}
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
  displayPingTaskID: number;
  pingTasks: PingTask[];
  onSaved: () => void;
}> = ({ nodeUuid, nodeName, displayPingTaskID, pingTasks, onSaved }) => {
  const { t } = useTranslation();
  const [open, setOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const assigned = React.useMemo(
    () => assignedDisplayTasks(nodeUuid, pingTasks),
    [nodeUuid, pingTasks],
  );
  const currentId = resolveDisplayPingTaskId(nodeUuid, pingTasks, displayPingTaskID);
  const [selectedId, setSelectedId] = React.useState(String(currentId));
  const displayedTask = assigned.find((task) => task.id === currentId);

  const handleSave = async () => {
    const selectedTaskId = Number(selectedId);
    if (selectedTaskId === currentId) {
      setOpen(false);
      return;
    }
    setSaving(true);
    try {
      const response = await fetch(`/api/admin/client/${nodeUuid}/display-ping-task`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ task_id: selectedTaskId }),
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
        {displayedTask?.name || t("ping.theme_default")}
      </TableCell>
      <TableCell data-label={t("common.action")}>
        <Dialog.Root open={open} onOpenChange={(next) => {
          if (next) setSelectedId(String(currentId));
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
            <Select.Root value={selectedId} onValueChange={setSelectedId}>
              <Select.Trigger className="w-full" aria-label={t("ping.display_task")} />
              <Select.Content>
                <Select.Item value="0">{t("ping.theme_default")}</Select.Item>
                {assigned.map((task) => (
                  <Select.Item key={task.id} value={String(task.id)}>
                    {task.name} ({task.type}/{task.interval}s)
                  </Select.Item>
                ))}
              </Select.Content>
            </Select.Root>
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

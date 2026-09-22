import AppDialogContent from "@/components/AppDialogContent";
import Loading from "@/components/loading";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import NodeSelectorDialog from "@/components/NodeSelectorDialog";
import { useNodeDetails } from "@/contexts/NodeDetailsContext";
import { usePingTask, type PingTask } from "@/contexts/PingTaskContext";
import {
  Box,
  Button,
  Dialog,
  Flex,
  Select,
  Tabs,
  TextField,
} from "@radix-ui/themes";
import { Checkbox } from "@/components/ui/checkbox";
import React from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { CheckCircle2, Radar, Search, Server } from "lucide-react";
import { TaskView } from "./pingTask_Task";
import { ServerView } from "./pingTask_Server";

const PingTask = () => <InnerLayout />;

const InnerLayout = () => {
  const { pingTasks, isLoading, error } = usePingTask();
  const {
    nodeDetail,
    isLoading: nodeDetailLoading,
    error: nodeDetailError,
  } = useNodeDetails();
  const { t } = useTranslation();
  const [search, setSearch] = React.useState("");
  const taskList = React.useMemo(() => pingTasks ?? [], [pingTasks]);
  const serverNamesByUuid = React.useMemo(
    () =>
      new Map(
        nodeDetail.map((node) => [
          node.uuid,
          String(node.name || "").toLowerCase(),
        ]),
      ),
    [nodeDetail],
  );
  const filteredTasks = React.useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return taskList;

    return taskList.filter((task) =>
      [
        task.name,
        task.target,
        ...(task.clients || []).map(
          (uuid) => serverNamesByUuid.get(uuid) || uuid,
        ),
      ].some((value) =>
        String(value || "")
          .toLowerCase()
          .includes(keyword),
      ),
    );
  }, [search, serverNamesByUuid, taskList]);
  const linkedServers = React.useMemo(
    () =>
      new Set(
        taskList.flatMap((task) =>
          Array.isArray(task.clients) ? task.clients : [],
        ),
      ).size,
    [taskList],
  );
  const defaultTasks = taskList.filter((task) => task.default_on).length;

  if (isLoading || nodeDetailLoading) {
    return <Loading />;
  }
  if (error || nodeDetailError) {
    return <div>{error || nodeDetailError}</div>;
  }
  return (
    <Flex direction="column" gap="4" className="p-0 md:p-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <AdminPageTitle description={t("ping.description")}>
          {t("ping.title")}
        </AdminPageTitle>
      </div>
      <div className="grid grid-cols-3 gap-2 sm:gap-3">
        <PingSummary
          label={t("ping.task_count", "监测任务")}
          value={taskList.length}
          icon={<Radar size={20} />}
        />
        <PingSummary
          label={t("ping.linked_server_count", "关联服务器")}
          value={Math.min(linkedServers, nodeDetail.length)}
          icon={<Server size={20} />}
        />
        <PingSummary
          label={t("ping.default_task_count", "默认开启任务")}
          value={defaultTasks}
          tone="green"
          icon={<CheckCircle2 size={20} />}
        />
      </div>
      <Tabs.Root defaultValue="task">
        <div className="flex w-full flex-wrap items-end gap-3">
          <div className="min-w-[16rem] flex-[1_1_16rem] overflow-x-auto pb-1">
            <Tabs.List className="w-max min-w-full">
              <Tabs.Trigger value="task" className="min-w-[8rem] flex-1">
                {t("ping.task_view")}
              </Tabs.Trigger>
              <Tabs.Trigger value="server" className="min-w-[8rem] flex-1">
                {t("ping.server_view")}
              </Tabs.Trigger>
            </Tabs.List>
          </div>
          <div className="ml-auto flex min-w-0 items-center gap-2 pb-2">
            <TextField.Root
              className="w-36 sm:w-64"
              value={search}
              placeholder={t("common.search")}
              aria-label={t("common.search")}
              onChange={(event) => setSearch(event.target.value)}
            >
              <TextField.Slot>
                <Search size={16} />
              </TextField.Slot>
            </TextField.Root>
            <AddButton />
          </div>
        </div>
        <Box pt="3">
          <Tabs.Content value="task">
            <TaskView
              pingTasks={filteredTasks}
              reorderEnabled={!search.trim()}
            />
          </Tabs.Content>
          <Tabs.Content value="server">
            <ServerView pingTasks={taskList} search={search} />
          </Tabs.Content>
        </Box>
      </Tabs.Root>
    </Flex>
  );
};

function PingSummary({
  label,
  value,
  icon,
  tone = "gray",
}: {
  label: string;
  value: number;
  icon: React.ReactNode;
  tone?: "gray" | "green";
}) {
  const color = tone === "green" ? "text-green-600" : "text-gray-500";
  return (
    <div className="flex min-h-24 min-w-0 items-center justify-between gap-2 rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] px-3 py-4 sm:px-5">
      <div>
        <div className="text-sm text-gray-500">{label}</div>
        <div className="mt-1 text-2xl font-semibold tabular-nums">{value}</div>
      </div>
      <span className={color}>{icon}</span>
    </div>
  );
}

const AddButton: React.FC = () => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = React.useState(false);
  const [selected, setSelected] = React.useState<string[]>([]);
  const [defaultOn, setDefaultOn] = React.useState(false);
  const { refresh } = usePingTask();
  const [selectedType, setSelectedType] = React.useState<
    "icmp" | "tcp" | "http"
  >("icmp");
  const [saving, setSaving] = React.useState(false);
  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!defaultOn && selected.length === 0) {
      toast.error(t("ping.default_on_description"));
      return;
    }
    const payload = {
      name: e.currentTarget.ping_name.value,
      type: selectedType,
      target: e.currentTarget.ping_target.value,
      default_on: defaultOn,
      clients: selected,
      interval: parseInt(e.currentTarget.interval.value, 10),
    };
    setSaving(true);
    fetch("/api/admin/ping/add", {
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
          setSelectedType("icmp");
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
        console.error("Error adding ping task:", error);
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
        <Button className="w-full sm:w-auto">{t("common.add")}</Button>
      </Dialog.Trigger>
      <AppDialogContent>
        <Dialog.Title>{t("common.add")}</Dialog.Title>
        <form onSubmit={handleSubmit}>
          <Flex direction="column" justify="end" gap="2" className="font-bold">
            <label htmlFor="ping_name">{t("common.name")}</label>
            <TextField.Root id="ping_name" name="ping_name" />
            <label htmlFor="type">{t("ping.type")}</label>
            <Select.Root
              value={selectedType}
              onValueChange={(value) =>
                setSelectedType(value as "icmp" | "tcp" | "http")
              }
            >
              <Select.Trigger id="type" name="type" />
              <Select.Content>
                <Select.Item value="icmp">ICMP</Select.Item>
                <Select.Item value="tcp">TCP</Select.Item>
                <Select.Item value="http">HTTP</Select.Item>
              </Select.Content>
            </Select.Root>
            <label htmlFor="ping_target">{t("ping.target")}</label>
            <TextField.Root
              id="ping_target"
              name="ping_target"
              placeholder="1.1.1.1 | 1.1.1.1:80 | https://1.1.1.1"
            />
            <label htmlFor="ping_server">{t("common.server")}</label>
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-start gap-2">
                <NodeSelectorDialog value={selected} onChange={setSelected} />
                <label className="text-sm font-normal">
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
            </div>
            <label htmlFor="interval">
              {t("ping.interval")} ({t("time.second")})
            </label>
            <TextField.Root
              id="interval"
              name="interval"
              defaultValue={60}
              type="number"
              placeholder="60"
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

export default PingTask;

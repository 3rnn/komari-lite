import AppDialogContent from "@/components/AppDialogContent";
import React from "react";
import { Button, Dialog, Flex, Text } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import Selector from "@/components/Selector";
import { usePingTask } from "@/contexts/PingTaskContext";

type PingTaskSelectorDialogProps = {
  value: number[];
  onChange: (ids: number[]) => void;
  title?: React.ReactNode;
  children: React.ReactElement;
};

export default function PingTaskSelectorDialog({
  value,
  onChange,
  title,
  children,
}: PingTaskSelectorDialogProps) {
  const { t } = useTranslation();
  const { pingTasks } = usePingTask();
  const tasks = React.useMemo(
    () => (pingTasks ?? []).filter((task) => Number(task.id) > 0),
    [pingTasks],
  );
  const [open, setOpen] = React.useState(false);
  const [temporary, setTemporary] = React.useState(value.map(String));

  React.useEffect(() => {
    if (open) setTemporary(value.map(String));
  }, [open, value]);

  const save = () => {
    onChange(
      temporary
        .map(Number)
        .filter((id) => Number.isInteger(id) && id > 0),
    );
    setOpen(false);
  };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger>{children}</Dialog.Trigger>
      <AppDialogContent maxWidth="440px">
        <Dialog.Title>{title || t("common.select")}</Dialog.Title>
        <Flex direction="column" gap="3">
          <Text size="2" color="gray">
            {t("common.selected_total", {
              count: temporary.length,
              total: tasks.length,
            })}
          </Text>
          <Selector
            value={temporary}
            onChange={setTemporary}
            items={tasks}
            getId={(task) => String(task.id)}
            getLabel={(task) => task.name || task.target || String(task.id)}
            filterItem={(task, keyword) =>
              [task.name, task.target, task.type]
                .join(" ")
                .toLocaleLowerCase()
                .includes(keyword.toLocaleLowerCase())
            }
            searchPlaceholder={t("common.search")}
            headerLabel={t("ping.task")}
            showHeaderSelectAll={false}
            hiddenDescription
          />
          <Flex justify="end" gap="2">
            <Dialog.Close>
              <Button variant="soft" color="gray">
                {t("common.cancel")}
              </Button>
            </Dialog.Close>
            <Button onClick={save}>{t("common.done")}</Button>
          </Flex>
        </Flex>
      </AppDialogContent>
    </Dialog.Root>
  );
}

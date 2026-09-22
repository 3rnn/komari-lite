import AppDialogContent from "@/components/AppDialogContent";
import React from "react";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { Button, Dialog, Flex } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import Loading from "@/components/loading";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import {
  AdminPagination,
} from "@/components/admin/AdminPagination";
import { useAdminDefaultPageSize } from "@/hooks/useAdminDefaultPageSize";

interface Log {
  id: number;
  ip: string;
  uuid: string;
  message: string;
  msg_type: string;
  time: string;
}
const LogPage = () => {
  const defaultPageSize = useAdminDefaultPageSize();
  const [loading, setLoading] = React.useState<boolean>(true);
  const [logs, setLogs] = React.useState<Log[]>([]);
  const [error, setError] = React.useState<string | null>(null);
  const [page, setPage] = React.useState<number>(1);
  const [total, setTotal] = React.useState<number>(1);
  const [limit, setLimit] = React.useState<number>(defaultPageSize);
  const limitCustomized = React.useRef(false);
  const [t] = useTranslation();
  React.useEffect(() => {
    if (limitCustomized.current) return;
    setLimit(defaultPageSize);
    setPage(1);
  }, [defaultPageSize]);
  React.useEffect(() => {
    const fetchLogs = async () => {
      setLoading(true);
      try {
        const response = await fetch(
          `/api/admin/logs?limit=${limit}&page=${page}`
        );
        if (!response.ok) {
          throw new Error("Failed to fetch logs");
        }
        const data = await response.json();
        setLogs(data.data.logs);
        setTotal(data.data.total);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unknown error");
      } finally {
        setLoading(false);
      }
    };
    fetchLogs();
  }, [limit, page]);

  if (loading) {
    return <Loading />;
  }
  if (error) {
    return <div>Error: {error}</div>;
  }

  return (
    <div className="flex flex-col gap-4 p-0 md:p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <AdminPageTitle
          description={t(
            "logs.description",
            "查看后台操作与系统事件记录。",
          )}
        >
          {t("logs.title")}
        </AdminPageTitle>
      </div>
      <div className="overflow-hidden rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)]">
        <div className="overflow-x-auto">
        <Table className="min-w-[760px]">
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>IP</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Message</TableHead>
              <TableHead>Time</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.map((log) => (
              <TableRow key={log.id}>
                <TableCell>
                  <Dialog.Root>
                    <Dialog.Trigger>
                      <label className="hover:underline font-bold">
                        {log.id}
                      </label>
                    </Dialog.Trigger>
                    <AppDialogContent>
                      <Dialog.Title>{t("log.title")}</Dialog.Title>
                      <Flex direction="column" gap="1">
                        <label className="font-bold">ID</label>
                        <label className="text-sm">{log.id}</label>
                        <label className="font-bold">IP</label>
                        <label className="text-sm">{log.ip}</label>
                        <label className="font-bold">UUID</label>
                        <label className="text-sm">{log.uuid}</label>
                        <label className="font-bold">Type</label>
                        <label className="text-sm">{log.msg_type}</label>
                        <label className="font-bold">Message</label>
                        <label className="text-sm">{log.message}</label>
                        <label className="font-bold">Time</label>
                        <label className="text-sm">
                          {new Date(log.time).toLocaleString()}
                        </label>
                      </Flex>
                      <Flex justify={"end"}>
                        <Dialog.Close>
                          <Button variant="soft">{t("close")}</Button>
                        </Dialog.Close>
                      </Flex>
                    </AppDialogContent>
                  </Dialog.Root>
                </TableCell>
                <TableCell>{log.ip}</TableCell>
                <TableCell>{log.msg_type}</TableCell>
                <TableCell>
                  {log.message.length > 75
                    ? `${log.message.slice(0, 75)}...`
                    : log.message}
                </TableCell>
                <TableCell>{new Date(log.time).toLocaleString()}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        </div>
        <AdminPagination
          page={page}
          total={total}
          pageSize={limit}
          onPageChange={setPage}
          onPageSizeChange={(value) => {
            limitCustomized.current = true;
            setLimit(value);
            setPage(1);
          }}
          showSummary={false}
        />
      </div>
    </div>
  );
};
export default LogPage;

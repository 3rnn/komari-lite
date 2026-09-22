import { Button, Callout } from "@radix-ui/themes";
import { Filter, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";

export default function AdminActiveFilter({
  label,
  clearTo,
}: {
  label: string;
  clearTo: string;
}) {
  const { t } = useTranslation();
  return (
    <Callout.Root size="1" className="admin-active-filter w-full">
      <Callout.Icon><Filter size={15} /></Callout.Icon>
      <Callout.Text className="flex min-w-0 flex-1 items-center justify-between gap-3">
        <span className="min-w-0 break-words">
          {t("admin_filter.current", "当前筛选")}：<strong>{label}</strong>
        </span>
        <Button asChild size="1" variant="soft" color="gray" className="shrink-0">
          <Link to={clearTo}><X size={13} />{t("admin_filter.clear", "清除筛选")}</Link>
        </Button>
      </Callout.Text>
    </Callout.Root>
  );
}

import { useTranslation } from "react-i18next";

export const AdminSelectionCount = ({
  count,
  total,
  className = "",
}: {
  count: number;
  total: number;
  className?: string;
}) => {
  const { t } = useTranslation();
  const label = t("common.selected_total", { count, total });
  const parts = label.split(/(\d+)/).filter(Boolean);

  return (
    <span
      className={`inline-flex h-5 items-center whitespace-nowrap leading-none tabular-nums ${className}`}
    >
      {parts.map((part, index) => (
        <span
          key={`${part}-${index}`}
          className={`inline-flex h-5 items-center whitespace-pre leading-5 ${/^\d+$/.test(part) ? "tabular-nums" : ""}`}
        >
          {part}
        </span>
      ))}
    </span>
  );
};

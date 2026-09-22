import { useTranslation } from "react-i18next";
import { motion } from "motion/react";
import { useReduceMotionPreference } from "@/lib/api";

export type AdminNodeStatusFilter = "all" | "online" | "offline";

type AdminNodeStatusSummaryProps = {
  total: number;
  online: number;
  available?: boolean;
  value: AdminNodeStatusFilter;
  onValueChange: (filter: AdminNodeStatusFilter) => void;
};

export default function AdminNodeStatusSummary({
  total,
  online,
  available = true,
  value,
  onValueChange,
}: AdminNodeStatusSummaryProps) {
  const { t } = useTranslation();
  const reduceMotion = useReduceMotionPreference();
  const offline = Math.max(0, total - online);
  const items = [
    {
      label: t("admin.nodeTable.allNodes", "全部节点"),
      count: total,
      color: "var(--accent-9)",
      filter: "all" as const,
    },
    {
      label: t("nodeCard.online", "在线"),
      count: available ? online : "--",
      color: "var(--green-9)",
      filter: "online" as const,
    },
    {
      label: t("nodeCard.offline", "离线"),
      count: available ? offline : "--",
      color: "var(--red-9)",
      filter: "offline" as const,
    },
  ];

  return (
    <div
      data-testid="node-status-summary"
      className="grid w-full grid-cols-3 overflow-hidden rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] md:w-auto"
    >
      {items.map(({ label, count, color, filter }, index) => (
        <motion.button
          key={filter}
          type="button"
          className={`relative isolate flex h-10 items-center justify-center gap-2 px-3 text-left transition-[color,box-shadow] duration-150 hover:bg-[var(--accent-a2)] disabled:cursor-not-allowed disabled:opacity-60 md:min-w-28 md:px-4 ${
            index > 0 ? "border-l border-[var(--gray-a5)]" : ""
          }`}
          aria-pressed={value === filter}
          disabled={filter !== "all" && !available}
          onClick={() => onValueChange(filter)}
          whileTap={reduceMotion ? undefined : { scale: 0.985 }}
          transition={{ duration: reduceMotion ? 0 : 0.12 }}
        >
          {value === filter ? (
            <motion.span
              aria-hidden="true"
              className="absolute inset-0 -z-10 bg-[var(--accent-a3)] shadow-[inset_0_-2px_0_var(--accent-a7)]"
              layoutId={reduceMotion ? undefined : "admin-node-status-highlight"}
              initial={false}
              transition={
                reduceMotion
                  ? { duration: 0 }
                  : { type: "spring", stiffness: 420, damping: 34 }
              }
            />
          ) : null}
          <span
            className="size-1.5 shrink-0 rounded-full"
            style={{ backgroundColor: color }}
          />
          <span className="flex min-w-0 items-baseline gap-2 whitespace-nowrap">
            <strong className="shrink-0 text-sm font-medium leading-5 tabular-nums">
              {count}
            </strong>
            <span className="min-w-0 truncate text-sm leading-5 text-muted-foreground">
              {label}
            </span>
          </span>
        </motion.button>
      ))}
    </div>
  );
}

import React from "react";
import { useDroppable } from "@dnd-kit/core";
import { Flex, IconButton, Select, Text } from "@radix-ui/themes";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useAdminDefaultPageSize } from "@/hooks/useAdminDefaultPageSize";
import {
  ADMIN_LIST_PAGE_SIZE,
  adminPageSizeOptions,
  normalizeAdminPageSize,
} from "@/utils/adminPagination";

export { ADMIN_LIST_PAGE_SIZE } from "@/utils/adminPagination";

export const useAdminPagination = <T,>(
  items: readonly T[],
  initialPageSize?: number,
) => {
  const defaultPageSize = useAdminDefaultPageSize();
  const followsGlobalDefault = initialPageSize === undefined;
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSizeState] = React.useState(() =>
    normalizeAdminPageSize(initialPageSize ?? defaultPageSize),
  );
  const pageSizeCustomized = React.useRef(false);
  const totalPages = Math.max(1, Math.ceil(items.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pageStart = (currentPage - 1) * pageSize;
  const pageItems = items.slice(pageStart, pageStart + pageSize);

  React.useEffect(() => {
    setPage((value) => Math.min(value, totalPages));
  }, [totalPages]);

  React.useEffect(() => {
    if (!followsGlobalDefault || pageSizeCustomized.current) return;
    setPageSizeState(defaultPageSize);
    setPage(1);
  }, [defaultPageSize, followsGlobalDefault]);

  const setPageSize = React.useCallback((value: number) => {
    pageSizeCustomized.current = true;
    setPageSizeState(normalizeAdminPageSize(value));
    setPage(1);
  }, []);

  return {
    page: currentPage,
    setPage,
    totalPages,
    pageStart,
    pageItems,
    pageSize,
    setPageSize,
  };
};

type PageButtonProps = {
  direction: "previous" | "next";
  disabled: boolean;
  label: string;
  onClick: () => void;
};

const PageButton = ({ direction, disabled, label, onClick }: PageButtonProps) => {
  const Icon = direction === "previous" ? ChevronLeft : ChevronRight;
  return (
    <IconButton
      type="button"
      size="2"
      variant="soft"
      color="gray"
      disabled={disabled}
      title={label}
      aria-label={label}
      onClick={onClick}
    >
      <Icon size={16} />
    </IconButton>
  );
};

const PageDropButton = ({
  id,
  dragging,
  ...props
}: PageButtonProps & { id: string; dragging: boolean }) => {
  const { isOver, setNodeRef } = useDroppable({
    id,
    disabled: props.disabled,
  });
  const Icon = props.direction === "previous" ? ChevronLeft : ChevronRight;
  return (
    <IconButton
      ref={setNodeRef}
      type="button"
      size="2"
      variant={isOver ? "solid" : "soft"}
      color={isOver ? undefined : "gray"}
      disabled={props.disabled}
      className={
        dragging && !props.disabled ? "ring-1 ring-[var(--accent-a7)]" : undefined
      }
      title={props.label}
      aria-label={props.label}
      onClick={props.onClick}
    >
      <Icon size={16} />
    </IconButton>
  );
};

export const AdminPagination = ({
  page,
  total,
  onPageChange,
  pageSize = ADMIN_LIST_PAGE_SIZE,
  onPageSizeChange,
  previousDropId,
  nextDropId,
  dragging = false,
  summary,
  showSummary = true,
}: {
  page: number;
  total: number;
  onPageChange: (page: number) => void;
  pageSize?: number;
  onPageSizeChange?: (pageSize: number) => void;
  previousDropId?: string;
  nextDropId?: string;
  dragging?: boolean;
  summary?: React.ReactNode;
  showSummary?: boolean;
}) => {
  const { t } = useTranslation();
  if (total === 0) return null;

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(Math.max(page, 1), totalPages);
  const pageStart = (currentPage - 1) * pageSize;
  const pageSizeOptions = adminPageSizeOptions();
  const usesCustomPageSize = !pageSizeOptions.includes(pageSize);
  const previousProps: PageButtonProps = {
    direction: "previous",
    disabled: currentPage <= 1,
    label: t("admin.nodeTable.previousPage", "上一页"),
    onClick: () => onPageChange(Math.max(1, currentPage - 1)),
  };
  const nextProps: PageButtonProps = {
    direction: "next",
    disabled: currentPage >= totalPages,
    label: t("admin.nodeTable.nextPage", "下一页"),
    onClick: () => onPageChange(Math.min(totalPages, currentPage + 1)),
  };

  return (
    <div className={`admin-pagination flex flex-wrap items-center gap-3 border-t border-[var(--gray-a5)] bg-[var(--color-panel-solid)] px-4 py-3 ${showSummary ? "justify-between" : "justify-end"}`}>
      {showSummary ? (
        <Text size="2" color="gray">
          {summary ??
            t("admin.nodeTable.pageSummary", {
              start: pageStart + 1,
              end: Math.min(pageStart + pageSize, total),
              total,
              defaultValue: "显示 {{start}}-{{end}}，共 {{total}} 台",
            })}
        </Text>
      ) : null}
      <Flex align="center" gap="2">
        {onPageSizeChange ? (
          <Select.Root
            value={String(pageSize)}
            onValueChange={(value) => onPageSizeChange(Number(value))}
          >
            <Select.Trigger aria-label={t("admin.nodeTable.pageSize", "每页条数")} />
            <Select.Content>
              {usesCustomPageSize ? (
                <Select.Item
                  value={String(pageSize)}
                  className="hidden"
                  aria-hidden="true"
                >
                  {pageSize} {t("admin.nodeTable.itemsPerPage", "条/页")}
                </Select.Item>
              ) : null}
              {pageSizeOptions.map((option) => (
                <Select.Item key={option} value={String(option)}>
                  {option} {t("admin.nodeTable.itemsPerPage", "条/页")}
                </Select.Item>
              ))}
            </Select.Content>
          </Select.Root>
        ) : null}
        {previousDropId ? (
          <PageDropButton id={previousDropId} dragging={dragging} {...previousProps} />
        ) : (
          <PageButton {...previousProps} />
        )}
        <Text size="2" className="min-w-16 text-center tabular-nums">
          {currentPage} / {totalPages}
        </Text>
        {nextDropId ? (
          <PageDropButton id={nextDropId} dragging={dragging} {...nextProps} />
        ) : (
          <PageButton {...nextProps} />
        )}
      </Flex>
    </div>
  );
};

import React from "react";
import { TextField } from "@radix-ui/themes";
import { Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Checkbox } from "./ui/checkbox";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "./ui/table";

/**
 * Generic multiselect list with search, select-all, indeterminate state, and orphaned values.
 * Pass items and getId/getLabel callbacks to define identity and display labels.
 */
export interface SelectorProps<T> {
  className?: string;
  hiddenDescription?: boolean;
  /** Selected ID list */
  value: string[];
  /** Selection change callback */
  onChange: (ids: string[]) => void;
  /** Data source */
  items: T[];
  /** Extract the unique ID */
  getId: (item: T) => string;
  /** Get the display label (cell content) */
  getLabel: (item: T) => React.ReactNode;
  /** Optional custom sorting */
  sortItems?: (a: T, b: T) => number;
  /** Custom search filter; return true to retain */
  filterItem?: (item: T, keyword: string) => boolean;
  /** Search placeholder */
  searchPlaceholder?: string;
  /** Header title for the second column */
  headerLabel?: React.ReactNode;
  /** Show a select-all checkbox in the header */
  showHeaderSelectAll?: boolean;
}

function SelectorInner<T>(props: SelectorProps<T>) {
  const {
    className = "",
    hiddenDescription = false,
    value: externalValue,
    onChange,
    items,
    getId,
    getLabel,
    sortItems,
    filterItem,
    searchPlaceholder,
    headerLabel,
    showHeaderSelectAll = true,
  } = props;
  const { t } = useTranslation();

  const value = externalValue ?? [];
  const [search, setSearch] = React.useState("");

  // Sort and search.
  const processed = React.useMemo(() => {
    let arr = [...items];
    if (sortItems) arr.sort(sortItems);
    if (search.trim()) {
      const kw = search.toLowerCase();
      arr = arr.filter((it) =>
        filterItem
          ? filterItem(it, search)
          : String(getLabel(it)).toLowerCase().includes(kw)
      );
    }
    return arr;
  }, [items, sortItems, filterItem, search, getLabel]);

  const allIds = processed.map(getId);

  // Indeterminate selection state.
  const allChecked =
    allIds.length > 0 && allIds.every((id) => value.includes(id));
  const isIndeterminate =
    value.length > 0 && value.some((id) => allIds.includes(id)) && !allChecked;
  const checkAllState = allChecked ? true : isIndeterminate ? "indeterminate" : false;

  // IDs that are selected but no longer in items.
  const orphanIds = value.filter((id) => !items.some((it) => getId(it) === id));

  const resolvedSearchPlaceholder =
    searchPlaceholder ?? t("common.search");
  const resolvedHeaderLabel = headerLabel ?? t("common.content");

  const handleCheckAll = (checked: boolean) => {
    if (checked) {
      onChange(Array.from(new Set([...value, ...allIds])));
    } else {
      onChange(value.filter((id) => !allIds.includes(id)));
    }
  };

  const handleCheck = (id: string, checked: boolean) => {
    if (checked) {
      onChange(Array.from(new Set([...value, id])));
    } else {
      onChange(value.filter((v) => v !== id));
    }
  };

  return (
    <div className={`flex flex-col ${className}`}>
      <TextField.Root
        className="mb-2 flex items-center gap-1"
        placeholder={resolvedSearchPlaceholder}
        value={search}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
          setSearch(e.target.value)
        }
      >
        <TextField.Slot>
          <Search size="16" />
        </TextField.Slot>
      </TextField.Root>
      <div className="selector rounded-md overflow-hidden">
        <Table>
          <TableHeader>
            <TableHead>
              {showHeaderSelectAll ? (
                <Checkbox
                  checked={checkAllState}
                  onClick={(event) => event.stopPropagation()}
                  onCheckedChange={(checked) => handleCheckAll(checked === true)}
                  aria-label={t("common.select_all")}
                />
              ) : null}
            </TableHead>
            <TableHead>{resolvedHeaderLabel}</TableHead>
          </TableHeader>
          <TableBody>
            {processed.map((it) => {
              const id = getId(it);
              return (
                <TableRow
                  key={id}
                  onClick={() => {
                    handleCheck(id, !value.includes(id));
                  }}
                >
                  <TableCell>
                    <Checkbox
                      checked={value.includes(id)}
                      onClick={(event) => event.stopPropagation()}
                      onCheckedChange={(checked) => handleCheck(id, checked === true)}
                      aria-label={`${t("common.select")} ${id}`}
                    />
                  </TableCell>
                  <TableCell>{getLabel(it)}</TableCell>
                </TableRow>
              );
            })}
            {orphanIds.map((id) => (
              <TableRow
                key={id}
                onClick={() => {
                  handleCheck(id, !value.includes(id));
                }}
              >
                <TableCell>
                  <Checkbox
                    checked={value.includes(id)}
                    onClick={(event) => event.stopPropagation()}
                    onCheckedChange={(checked) => handleCheck(id, checked === true)}
                    aria-label={`${t("common.select")} ${id}`}
                  />
                </TableCell>
                <TableCell>{id}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {!hiddenDescription && (
        <label className="text-sm text-gray-500">
          {t("common.selected", { count: value.length })}
        </label>
      )}
    </div>
  );
}

/** Export the generic component. */
export function Selector<T>(props: SelectorProps<T>) {
  return <SelectorInner {...props} />;
}

export default Selector;

import { useSettings } from "@/lib/api";
import { normalizeAdminPageSize } from "@/utils/adminPagination";

export function useAdminDefaultPageSize(): number {
  const { settings } = useSettings();
  return normalizeAdminPageSize(settings.admin_default_page_size);
}

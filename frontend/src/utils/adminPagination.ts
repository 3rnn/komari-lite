export const ADMIN_LIST_PAGE_SIZE = 10;
export const ADMIN_LIST_PAGE_SIZE_MIN = 5;
export const ADMIN_LIST_PAGE_SIZE_MAX = 100;

export function isValidAdminPageSize(value: unknown): value is number {
  return (
    typeof value === "number" &&
    Number.isInteger(value) &&
    value >= ADMIN_LIST_PAGE_SIZE_MIN &&
    value <= ADMIN_LIST_PAGE_SIZE_MAX
  );
}

export function normalizeAdminPageSize(
  value: unknown,
  fallback = ADMIN_LIST_PAGE_SIZE,
): number {
  const numericValue = typeof value === "string" ? Number(value) : value;
  return isValidAdminPageSize(numericValue) ? numericValue : fallback;
}

export function adminPageSizeOptions(): number[] {
  return [10, 50, 100];
}

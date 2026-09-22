import type { MenuItem } from "@/types/menu";

export const APPEARANCE_MENU_PATH = "/admin/appearance";

export function buildAdminMenuItems(
  baseItems: MenuItem[],
  themeItems: MenuItem[],
): MenuItem[] {
  return baseItems.map((item) =>
    item.path === APPEARANCE_MENU_PATH
      ? {
          ...item,
          children: [...(item.children ?? []), ...themeItems],
        }
      : item,
  );
}

export function toggleSingleSubMenu(
  current: Record<string, boolean>,
  path: string,
): Record<string, boolean> {
  return current[path] ? {} : { [path]: true };
}

export function syncSubMenuForLocation(
  current: Record<string, boolean>,
  items: MenuItem[],
  pathname: string,
): Record<string, boolean> {
  const activeGroup = items.find((item) =>
    item.children?.some(
      (child) =>
        pathname === child.path || pathname.startsWith(`${child.path}/`),
    ),
  );

  if (!activeGroup) return current;
  if (
    current[activeGroup.path] &&
    Object.keys(current).every((path) => path === activeGroup.path)
  ) {
    return current;
  }
  return { [activeGroup.path]: true };
}

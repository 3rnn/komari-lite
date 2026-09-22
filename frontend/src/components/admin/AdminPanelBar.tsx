import { Cross1Icon, ExitIcon } from "@radix-ui/react-icons";
import { Callout, Flex, Grid, IconButton, Text } from "@radix-ui/themes";
import { AnimatePresence, motion } from "motion/react";
import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router-dom";
import ColorSwitch from "../ColorSwitch";
import LanguageSwitch from "../Language";
import ThemeSwitch from "../ThemeSwitch";
import KomariLiteBrand from "../KomariLiteBrand";
import { useIsMobile } from "@/hooks/use-mobile";
import menuConfig from "../../config/menuConfig.json";
import type { MenuItem } from "../../types/menu";
import { iconMap } from "../../utils/iconHelper";
import { ChevronDownIcon } from "@radix-ui/react-icons";
import { TablerMenu2 } from "../Icones/Tabler";
import {
  syncSubMenuForLocation,
  toggleSingleSubMenu,
} from "@/utils/adminMenu";
import { useSettings } from "@/lib/api";
import { preloadAdminRoute } from "@/routes";

// 将JSON配置转换为类型安全的菜单项数组 (基础静态菜单)
const parsedMenuConfig = menuConfig as {
  menu: MenuItem[];
  footer?: MenuItem[];
};
const baseMenuItems = parsedMenuConfig.menu;
const footerMenuItems = parsedMenuConfig.footer ?? [];
const DESKTOP_SIDEBAR_WIDTH = 212;
const MOBILE_SIDEBAR_WIDTH = "clamp(184px, 42vw, 244px)";
const MOBILE_SIDEBAR_OPEN_TRANSITION = {
  duration: 0.22,
  ease: [0.22, 1, 0.36, 1],
} as const;
const MOBILE_SIDEBAR_CLOSE_TRANSITION = {
  duration: 0.18,
  ease: [0.4, 0, 1, 1],
} as const;
interface AdminPanelBarProps {
  content: ReactNode;
}

const AdminPanelBar = ({ content }: AdminPanelBarProps) => {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [openSubMenus, setOpenSubMenus] = useState<{ [key: string]: boolean }>({
    // 默认所有子菜单关闭
  });
  const isMobile = useIsMobile();
  const ishttps = window.location.protocol === "https:";
  const { t } = useTranslation();
  const location = useLocation();
  const { settings } = useSettings();
  const reduceMotion = Boolean(settings.reduce_motion);
  const menuItems = baseMenuItems;

  useEffect(() => {
    document.documentElement.dataset.reduceMotion = reduceMotion ? "true" : "false";
    document.documentElement.dataset.adminShellActive = "true";
    return () => {
      delete document.documentElement.dataset.reduceMotion;
      delete document.documentElement.dataset.adminShellActive;
    };
  }, [reduceMotion]);

  useEffect(() => {
    const shell = document.querySelector<HTMLElement>("[data-admin-shell]");
    if (!shell || reduceMotion) return;

    const tabLists = new Set<HTMLElement>();
    const scheduledFrames = new Map<HTMLElement, number>();

    const updateIndicator = (list: HTMLElement) => {
      scheduledFrames.delete(list);
      const activeTab = list.querySelector<HTMLElement>(
        '.rt-TabsTrigger[data-state="active"], [role="tab"][aria-selected="true"]',
      );
      if (!activeTab) return;

      const listRect = list.getBoundingClientRect();
      const tabRect = activeTab.getBoundingClientRect();
      list.style.setProperty(
        "--admin-tab-highlight-x",
        `${tabRect.left - listRect.left + list.scrollLeft}px`,
      );
      list.style.setProperty("--admin-tab-highlight-width", `${tabRect.width}px`);
      if (!list.hasAttribute("data-admin-tab-motion-ready")) {
        window.requestAnimationFrame(() => {
          if (list.isConnected) list.setAttribute("data-admin-tab-motion-ready", "true");
        });
      }
    };

    const scheduleIndicator = (list: HTMLElement) => {
      const currentFrame = scheduledFrames.get(list);
      if (currentFrame) window.cancelAnimationFrame(currentFrame);
      scheduledFrames.set(
        list,
        window.requestAnimationFrame(() => updateIndicator(list)),
      );
    };

    const resizeObserver = new ResizeObserver((entries) => {
      entries.forEach((entry) => scheduleIndicator(entry.target as HTMLElement));
    });

    const registerTabList = (list: HTMLElement) => {
      if (tabLists.has(list)) return;
      tabLists.add(list);
      resizeObserver.observe(list);
      list.addEventListener("scroll", handleTabListScroll, { passive: true });
      updateIndicator(list);
    };

    function handleTabListScroll(event: Event) {
      scheduleIndicator(event.currentTarget as HTMLElement);
    }

    const registerTabListsWithin = (root: ParentNode) => {
      root.querySelectorAll<HTMLElement>(".rt-TabsList").forEach(registerTabList);
    };

    registerTabListsWithin(shell);
    const mutationObserver = new MutationObserver((records) => {
      records.forEach((record) => {
        if (record.type === "attributes") {
          const list = (record.target as HTMLElement).closest<HTMLElement>(".rt-TabsList");
          if (list) scheduleIndicator(list);
          return;
        }
        record.addedNodes.forEach((node) => {
          if (!(node instanceof HTMLElement)) return;
          if (node.matches(".rt-TabsList")) registerTabList(node);
          registerTabListsWithin(node);
        });
      });
    });
    mutationObserver.observe(shell, {
      attributes: true,
      attributeFilter: ["data-state", "aria-selected"],
      childList: true,
      subtree: true,
    });

    const handleResize = () => tabLists.forEach(scheduleIndicator);
    window.addEventListener("resize", handleResize, { passive: true });
    return () => {
      mutationObserver.disconnect();
      resizeObserver.disconnect();
      window.removeEventListener("resize", handleResize);
      scheduledFrames.forEach((frame) => window.cancelAnimationFrame(frame));
      tabLists.forEach((list) => {
        list.removeEventListener("scroll", handleTabListScroll);
        list.removeAttribute("data-admin-tab-motion-ready");
        list.style.removeProperty("--admin-tab-highlight-x");
        list.style.removeProperty("--admin-tab-highlight-width");
      });
    };
  }, [reduceMotion]);

  const getAdminLinkTarget = (target: EventTarget | null) => {
    if (!(target instanceof Element)) return null;
    const anchor = target.closest<HTMLAnchorElement>("a[href]");
    if (!anchor || anchor.target === "_blank" || anchor.hasAttribute("download")) {
      return null;
    }
    if (anchor.dataset.adminReloadDocument === "true") return null;
    const url = new URL(anchor.href, window.location.href);
    if (url.origin !== window.location.origin) return null;
    if (url.pathname !== "/admin" && !url.pathname.startsWith("/admin/")) {
      return null;
    }
    return `${url.pathname}${url.search}${url.hash}`;
  };

  const preloadAdminLink = (target: EventTarget | null) => {
    const href = getAdminLinkTarget(target);
    if (href) void preloadAdminRoute(href);
  };

  // Handle responsive behavior
  useEffect(() => {
    const handleResize = () => setSidebarOpen(!isMobile);
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [isMobile]);

  // 根据路径自动展开子菜单（包含动态扩展项）
  useEffect(() => {
    setOpenSubMenus((current) =>
      syncSubMenuForLocation(current, menuItems, location.pathname),
    );
  }, [location.pathname, menuItems]);

  // 侧边栏动画变体
  const sidebarVariants = reduceMotion
    ? {
        open: { x: 0, opacity: 1, transition: { duration: 0 } },
        closed: {
          x: isMobile ? "-100%" : 0,
          opacity: 1,
          transition: { duration: 0 },
        },
      }
    : isMobile
    ? {
        open: {
          x: 0,
          transition: MOBILE_SIDEBAR_OPEN_TRANSITION,
        },
        closed: {
          x: "-100%",
          transition: MOBILE_SIDEBAR_CLOSE_TRANSITION,
        },
      }
    : {
        open: {
          x: 0,
          opacity: 1,
          transition: {
            type: "spring",
            stiffness: 300,
            damping: 30,
          },
        },
        closed: {
          x: 0,
          opacity: 1,
          transition: {
            type: "spring",
            stiffness: 300,
            damping: 30,
          },
        },
      } as const;

  // 内容区域动画变体
  const contentVariants = {
    open: {
      opacity: 1,
      x: 0,
      transition: {
        duration: reduceMotion ? 0 : 0.3,
      },
    },
    closed: {
      opacity: 1,
      x: 0,
      transition: {
        duration: reduceMotion ? 0 : 0.3,
      },
    },
  };

  function logout() {
    window.open("/api/logout", "_self");
  }

  const renderIcon = (
    icon: string,
    labelKey: string,
    className?: string,
    active?: boolean,
  ) => {
    const link = /^(https?:\/\/|\/|\.\/|\.\.\/)/.test(icon);
    if (link) {
      return (
        <img
          src={icon}
          alt={t(labelKey)}
          style={{
            width: 16,
            height: 16,
            objectFit: "contain",
            opacity: active ? 1 : 0.7,
            filter: active ? "none" : "grayscale(20%)",
          }}
          className={className}
          loading="lazy"
        />
      );
    }
    const Icon = iconMap[icon];
    if (Icon) {
      return (
        <Icon
          className={className}
          style={{
            color: active ? "var(--accent-10)" : "var(--gray11)",
          }}
        />
      );
    }
    return (
      <span
        className={className}
        style={{
          width: 16,
          height: 16,
          display: "inline-block",
          borderRadius: 4,
          background: "var(--accent-8)",
        }}
      />
    );
  };

  const renderMenuItems = (items: MenuItem[]) =>
    items.map((item) => {
      const isOpen = openSubMenus[item.path];
      if (item.children?.length) {
        const submenu = (
          <Flex direction="column" className="ml-4 gap-1">
            {item.children.map((child) => (
              <SidebarItem
                key={child.path}
                to={child.path}
                icon={renderIcon(
                  child.icon,
                  child.labelKey,
                  "flex w-4 h-5 items-center justify-center",
                )}
                onClick={() => isMobile && setSidebarOpen(false)}
                newTab={child.newTab}
                reloadDocument={child.reloadDocument}
              >
                {child.rawLabel || t(child.labelKey)}
              </SidebarItem>
            ))}
          </Flex>
        );

        return (
          <div key={item.path}>
            <Flex
              className="p-2 gap-2 border-l-[4px] border-transparent cursor-pointer hover:bg-accent-3 rounded-md"
              align="center"
              onClick={() => {
                setOpenSubMenus((current) =>
                  toggleSingleSubMenu(current, item.path),
                );
              }}
            >
              {renderIcon(
                item.icon,
                item.labelKey,
                "flex w-4 h-5 items-center justify-center",
              )}
              <Text className="text-base" weight="medium" style={{ flex: 1 }}>
                {item.rawLabel || t(item.labelKey)}
              </Text>
              <ChevronDownIcon
                style={{
                  transform: isOpen ? "rotate(180deg)" : "rotate(0deg)",
                  transition: "transform 0.2s",
                }}
              />
            </Flex>
            <motion.div
              initial={{ height: 0, opacity: 0 }}
              animate={
                isOpen
                  ? { height: "auto", opacity: 1 }
                  : { height: 0, opacity: 0 }
              }
              transition={reduceMotion ? { duration: 0 } : { duration: 0.14 }}
              style={{ overflow: "hidden" }}
            >
              {submenu}
            </motion.div>
          </div>
        );
      }
      return (
        <SidebarItem
          key={item.path}
          to={item.path}
          icon={renderIcon(
            item.icon,
            item.labelKey,
            "flex w-4 h-5 items-center justify-center",
          )}
          onClick={() => isMobile && setSidebarOpen(false)}
          newTab={item.newTab}
          reloadDocument={item.reloadDocument}
        >
          {item.rawLabel || t(item.labelKey)}
        </SidebarItem>
      );
    });

  return (
    <>
      <Grid
        data-admin-shell
        onPointerOverCapture={(event) => preloadAdminLink(event.target)}
        onFocusCapture={(event) => preloadAdminLink(event.target)}
        onTouchStartCapture={(event) => preloadAdminLink(event.target)}
        columns={{
          initial: "1fr",
          md: sidebarOpen
            ? `${DESKTOP_SIDEBAR_WIDTH}px 1fr`
            : "0px 1fr",
        }} // 动态调整网格列
        rows={{
          initial: "auto minmax(0, 1fr)",
          md: "auto minmax(0, 1fr)",
        }}
        style={{
          height: "var(--app-viewport-height, 100vh)",
          width: "100%",
          overflow: "hidden",
          overscrollBehavior: "none",
          backgroundColor: "var(--accent-1)",
          position: "relative",
        }}
      >
        {/* Navbar */}
        <motion.nav
          className="md:col-span-2"
          initial={{ y: 0 }}
          animate={{ y: 0 }}
          transition={{ duration: 0.5, ease: "easeOut" }}
        >
          <Flex
            gap={isMobile ? "1" : "3"}
            p="2"
            justify="between"
            align="center"
            className="border-b-1"
          >
            <Flex
              gap={isMobile ? "2" : "3"}
              align="end"
              style={{ minHeight: "calc(32px * var(--scaling))" }}
            >
              <IconButton
                size="2"
                variant="ghost"
                data-testid="mobile-sidebar-trigger"
                aria-label={t("navigation.open")}
                onClick={() => setSidebarOpen(!sidebarOpen)}
                className="shrink-0"
                style={{
                  display: isMobile && sidebarOpen ? "none" : "flex",
                  color: "var(--gray-11)",
                }}
              >
                <TablerMenu2 className="h-6 w-6" />
              </IconButton>
              <a
                href="/"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-end leading-none"
              >
                <KomariLiteBrand size={isMobile ? "sm" : "md"} />
              </a>
            </Flex>
            <Flex
              gap={isMobile ? "1" : "3"}
              align="center"
              overflowX="auto"
              className="shrink-0 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
            >
              <ThemeSwitch />
              <ColorSwitch />
              <LanguageSwitch />
              <IconButton variant="soft" color="orange" onClick={logout}>
                <ExitIcon />
              </IconButton>
            </Flex>
          </Flex>
        </motion.nav>

        {/* Sidebar */}
        <AnimatePresence>
          {isMobile && sidebarOpen && (
            <motion.button
              key="mobile-sidebar-backdrop"
              type="button"
              aria-label={t("close", "关闭导航")}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: reduceMotion ? 0 : 0.2 }}
              onClick={() => setSidebarOpen(false)}
              className="absolute inset-0 z-[49] touch-none cursor-default border-0 bg-[var(--black-a6)] p-0"
            />
          )}
          <motion.div
            key="admin-sidebar"
            variants={sidebarVariants}
            initial={false}
            animate={sidebarOpen ? "open" : "closed"}
            exit="closed"
            style={{
              backgroundColor: "var(--accent-1)",
              width: isMobile
                ? MOBILE_SIDEBAR_WIDTH
                : sidebarOpen
                  ? `${DESKTOP_SIDEBAR_WIDTH}px`
                  : "0px",
              height: "100%",
              position: isMobile ? "absolute" : "relative",
              top: isMobile ? 0 : undefined,
              left: isMobile ? 0 : undefined,
              zIndex: isMobile ? 50 : 1,
              overflowY: "auto",
              overflowX: "hidden",
              overscrollBehaviorY: "contain",
              WebkitOverflowScrolling: "touch",
              willChange: isMobile ? "transform" : undefined,
              backfaceVisibility: isMobile ? "hidden" : undefined,
              pointerEvents: isMobile && !sidebarOpen ? "none" : "auto",
            }}
          >
            <Flex
              gap="3"
              className="p-2 border-r-1"
              direction="column"
              justify="start"
              align="start"
              style={{
                height: "100%",
                minWidth: isMobile ? "100%" : `${DESKTOP_SIDEBAR_WIDTH}px`,
              }}
            >
              {/* 关闭按钮 */}
              <IconButton
                variant="soft"
                data-testid="mobile-sidebar-close"
                aria-label={t("close", "关闭导航")}
                style={{
                  display: isMobile ? "flex" : "none",
                  margin: "8px 0px 0px 8px",
                }}
                onClick={() => setSidebarOpen(false)}
              >
                <Cross1Icon />
              </IconButton>
              {/* 侧边连链接 */}
              <Flex
                direction="column"
                gap="1"
                className="h-full md:mt-0 mt-6"
                style={{ width: "100%" }}
              >
                <Flex direction="column" gap="1" style={{ width: "100%" }}>
                  {renderMenuItems(menuItems)}
                </Flex>
                <Flex
                  direction="column"
                  gap="1"
                  className="mt-auto border-t border-[var(--gray-a5)] pt-2"
                  style={{ width: "100%" }}
                >
                  {renderMenuItems(footerMenuItems)}
                </Flex>
              </Flex>
            </Flex>
          </motion.div>
        </AnimatePresence>

        {/* Main Content */}
        <motion.div
          variants={contentVariants}
          animate={sidebarOpen ? "open" : "closed"}
          style={{
            backgroundColor: "var(--accent-3)",
            display: "block",
            height: "100%", // Ensure the container takes full height
            minHeight: 0,
            minWidth: 0,
            maxWidth: "100%",
            overflow: "hidden", // Prevent this container from scrolling
          }}
        >
          <div
            data-admin-scroll-container
            style={{
              backgroundColor: "var(--accent-1)",
              height: "100%",
              borderRadius: "0",
              padding: isMobile ? "8px" : "16px",
              overflowY: isMobile && sidebarOpen ? "hidden" : "auto",
              overscrollBehaviorY: "contain",
              WebkitOverflowScrolling: "touch",
              boxSizing: "border-box",
            }}
          >
            <Callout.Root mb="2" hidden={ishttps} color="red">
              <Callout.Icon>
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="24"
                  viewBox="0 0 24 24"
                >
                  <path
                    fill="currentColor"
                    d="M10.03 3.659c.856-1.548 3.081-1.548 3.937 0l7.746 14.001c.83 1.5-.255 3.34-1.969 3.34H4.254c-1.715 0-2.8-1.84-1.97-3.34zM12.997 17A.999.999 0 1 0 11 17a.999.999 0 0 0 1.997 0m-.259-7.853a.75.75 0 0 0-1.493.103l.004 4.501l.007.102a.75.75 0 0 0 1.493-.103l-.004-4.502z"
                  />
                </svg>
              </Callout.Icon>
              <Callout.Text>
                <Text size="2" weight="medium">
                  {t("warn_https")}
                </Text>
              </Callout.Text>
            </Callout.Root>
            <div data-admin-page-content style={{ minHeight: "100%" }}>
              {content}
            </div>
          </div>
        </motion.div>
      </Grid>
    </>
  );
};

export default AdminPanelBar;

// 侧边栏项目组件
const SidebarItem = ({
  to,
  onClick,
  icon,
  children,
  newTab,
  reloadDocument,
}: {
  to: string;
  onClick: () => void;
  icon: ReactNode;
  children: ReactNode;
  newTab?: boolean;
  reloadDocument?: boolean;
}) => {
  const location = useLocation();
  const isExternalLink = to.startsWith("http://") || to.startsWith("https://");
  const isActive =
    !isExternalLink &&
    to !== "/" &&
    (location.pathname === to ||
      (to !== "/admin" && location.pathname.startsWith(to)));
  const openInNewTab = newTab === true || (isExternalLink && newTab !== false);
  const preload = () => {
    if (!isExternalLink && !reloadDocument) void preloadAdminRoute(to);
  };

  if (openInNewTab || reloadDocument) {
    return (
      <a
        href={to}
        data-admin-reload-document={reloadDocument ? "true" : undefined}
        onClick={onClick}
        target={openInNewTab ? "_blank" : undefined}
        rel={openInNewTab ? "noopener noreferrer" : undefined}
        className="group transition-colors duration-200 hover:bg-accent-3 rounded-md"
      >
        <Flex
          className="p-2 gap-2 h-full"
          align="center"
          style={{
            borderLeft: "4px solid transparent",
            borderRadius: "6px",
            backgroundColor: "transparent",
            color: "inherit",
            transition: "background-color 0.2s, border-color 0.2s",
          }}
        >
          <span
            style={{
              color: "inherit",
              opacity: 0.7,
            }}
            className="flex w-4 h-5 items-center justify-center"
          >
            {icon}
          </span>
          <Text className="text-base" weight="medium" style={{ flex: 1 }}>
            {children}
          </Text>
        </Flex>
      </a>
    );
  }

  return (
    <Link
      to={to}
      onPointerEnter={preload}
      onFocus={preload}
      onTouchStart={preload}
      onClick={onClick}
      className="group transition-colors duration-200 hover:bg-accent-3 rounded-md"
    >
      <Flex
        className="p-2 gap-2"
        align="center"
        style={{
          borderLeft: isActive
            ? "4px solid var(--accent-8)"
            : "4px solid transparent",
          borderRadius: "6px",
          backgroundColor: isActive ? "var(--accent-4)" : "transparent",
          color: isActive ? "var(--accent-10)" : "inherit",
          transition: "background-color 0.2s, border-color 0.2s",
        }}
      >
        <span
          style={{
            color: isActive ? "var(--accent-10)" : "inherit",
            opacity: isActive ? 1 : 0.7,
          }}
          className="flex w-4 h-5 items-center justify-center"
        >
          {icon}
        </span>
        <Text
          className="text-base"
          weight={isActive ? "bold" : "medium"}
          style={{ flex: 1 }}
        >
          {children}
        </Text>
      </Flex>
    </Link>
  );
};

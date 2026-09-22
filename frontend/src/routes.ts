// routes.js
import { lazy } from "react";
import { Navigate, type RouteObject } from "react-router-dom";
import React from "react";
import {
  expandAdminPreloadTargets,
  LIKELY_ADMIN_ROUTES,
  normalizeAdminPathname,
} from "./utils/adminPreload";

const importAdminLayout = () => import("./pages/admin/_layout");
const importAdminDashboard = () => import("./pages/admin/dashboard");
const importAdminServers = () => import("./pages/admin");
const importAdminPing = () => import("./pages/admin/pingTask");
const importAdminSettingsLayout = () =>
  import("./pages/admin/settings/_layout");
let adminLayoutModule: ReturnType<typeof importAdminLayout> | undefined;
let adminDashboardModule: ReturnType<typeof importAdminDashboard> | undefined;
let adminServersModule: ReturnType<typeof importAdminServers> | undefined;
let adminPingModule: ReturnType<typeof importAdminPing> | undefined;
let adminSettingsLayoutModule: ReturnType<
  typeof importAdminSettingsLayout
> | undefined;
const loadAdminLayout = () => (adminLayoutModule ??= importAdminLayout());
const loadAdminDashboard = () =>
  (adminDashboardModule ??= importAdminDashboard());
const loadAdminServers = () => (adminServersModule ??= importAdminServers());
const loadAdminPing = () => (adminPingModule ??= importAdminPing());
const loadAdminSettingsLayout = () =>
  (adminSettingsLayoutModule ??= importAdminSettingsLayout());

export const preloadAdminEntry = (pathname: string) => {
  void loadAdminLayout();
  if (pathname === "/admin") void loadAdminDashboard();
};

const adminRoutePreloaders: Record<string, () => Promise<unknown>> = {
  "/admin": loadAdminDashboard,
  "/admin/servers": loadAdminServers,
  "/admin/ping": loadAdminPing,
  "/admin/logs": () => import("./pages/admin/log"),
  "/admin/theme_managed": () => import("./pages/admin/theme_managed.tsx"),
  "/admin/settings": loadAdminSettingsLayout,
  "/admin/settings/site": () => import("./pages/admin/settings/site"),
  "/admin/settings/dashboard": () => import("./pages/admin/settings/dashboard"),
  "/admin/settings/custom": () => import("./pages/admin/settings/custom"),
  "/admin/settings/notification": () => import("./pages/admin/settings/notification"),
  "/admin/settings/general": () => import("./pages/admin/settings/general"),
  "/admin/settings/metrics": () => import("./pages/admin/settings/metrics"),
  "/admin/settings/account-security": () => import("./pages/admin/settings/account-security"),
  "/admin/notification/offline": () => import("./pages/admin/notification/offline"),
  "/admin/notification/load": () => import("./pages/admin/notification/load"),
  "/admin/notification/general": () => import("./pages/admin/notification/general"),
  "/admin/notification/traffic-report": () => import("./pages/admin/notification/traffic_report"),
  "/admin/notification/ping-loss": () => import("./pages/admin/notification/ping_loss"),
};

export const preloadAdminRoute = async (target: string): Promise<void> => {
  const pathname = normalizeAdminPathname(target);
  await Promise.all(
    expandAdminPreloadTargets(pathname).map((path) => {
      const preload = adminRoutePreloaders[path];
      return preload ? preload() : Promise.resolve();
    }),
  );
};

export const preloadAdminRoutes = async (
  targets: readonly string[] = LIKELY_ADMIN_ROUTES,
): Promise<void> => {
  for (const target of targets) {
    await preloadAdminRoute(target);
  }
};

const AdminLayout = lazy(loadAdminLayout);
const AdminDashboard = lazy(loadAdminDashboard);
const AdminServers = lazy(loadAdminServers);
const AdminPing = lazy(loadAdminPing);
const AdminSettingsLayout = lazy(loadAdminSettingsLayout);
const NotFound = lazy(() => import("./pages/404"));

export const routes: RouteObject[] = [
  {
    path: "/admin/update/1.2.7",
    element: React.createElement(
      lazy(() => import("./pages/admin/update_1_2_7"))
    ),
  },
  {
    path: "/admin/update/storage-v4",
    element: React.createElement(
      lazy(() => import("./pages/admin/update_storage_v4"))
    ),
  },
  {
    path: "/install",
    element: React.createElement(lazy(() => import("./pages/install"))),
  },
  {
    path: "/admin",
    element: React.createElement(AdminLayout),
    children: [
      { index: true, element: React.createElement(AdminDashboard) },
      {
        path: "servers",
        element: React.createElement(AdminServers),
      },
      {
        path: "theme_managed",
        element: React.createElement(
          lazy(() => import("./pages/admin/theme_managed.tsx"))
        ),
      },
      {
        path: "sessions",
        element: React.createElement(Navigate, {
          replace: true,
          to: "/admin/settings/account-security?tab=sessions",
        }),
      },
      {
        path: "account",
        element: React.createElement(Navigate, {
          replace: true,
          to: "/admin/settings/account-security?tab=account",
        }),
      },
      {
        path: "settings",
        element: React.createElement(AdminSettingsLayout),
        children: [
          {
            path: "site",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/site"))
            ),
          },
          {
            path: "dashboard",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/dashboard"))
            ),
          },
          {
            path: "custom",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/custom"))
            ),
          },
          {
            path: "notification",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/notification"))
            ),
          },
          {
            path: "general",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/general"))
            ),
          },
          {
            path: "metrics",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/metrics"))
            ),
          },
          {
            path: "account-security",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/account-security"))
            ),
          },
        ],
      },
      {
        path: "notification",
        children: [
          {
            path: "offline",
            element: React.createElement(
              lazy(() => import("./pages/admin/notification/offline"))
            ),
          },
          {
            path: "load",
            element: React.createElement(
              lazy(() => import("./pages/admin/notification/load"))
            ),
          },
          {
            path: "general",
            element: React.createElement(
              lazy(() => import("./pages/admin/notification/general"))
            ),
          },
          {
            path: "traffic-report",
            element: React.createElement(
              lazy(() => import("./pages/admin/notification/traffic_report"))
            ),
          },
          {
            path: "ping-loss",
            element: React.createElement(
              lazy(() => import("./pages/admin/notification/ping_loss"))
            ),
          },
        ],
      },
      {
        path: "ping",
        element: React.createElement(AdminPing),
      },
      {
        path: "logs",
        element: React.createElement(lazy(() => import("./pages/admin/log"))),
      },
    ],
  },
  {
    path: "/manage/*",
    element: React.createElement(lazy(() => import("./pages/disabled"))),
  },
  // Catch-all 404 route
  { path: "*", element: React.createElement(NotFound) },
];

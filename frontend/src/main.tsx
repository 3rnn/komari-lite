import React, { StrictMode, useMemo } from "react";
import { createRoot } from "react-dom/client";
import "./global.css";
import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import {
  ThemeContext,
  THEME_DEFAULTS,
  type Appearance,
  type Colors,
} from "./contexts/ThemeContext";
import { useLocalStorage } from "./hooks/useLocalStorage";
import { useSystemTheme } from "./hooks/useSystemTheme";
import { BrowserRouter } from "react-router-dom";
// Ensure i18n is initialized before any component renders
import "./i18n/config";
import ErrorBoundary from "./components/ErrorBoundary";
import { Suspense } from "react";
import { useRoutes } from "react-router-dom";
import { preloadAdminEntry, preloadAdminRoute, routes } from "./routes";
import Loading from "./components/loading";
import { PublicInfoProvider } from "./contexts/PublicInfoContext";
import { OfflineIndicator } from "./components/OfflineIndicator";
import { Toaster } from "./components/ui/sonner";
import { RPC2Provider } from "./contexts/RPC2Context";
import { NodeListProvider } from "./contexts/NodeListContext";
import { AccountProvider } from "./contexts/AccountContext";
import { useAccount } from "./contexts/AccountContext";
import FullPageLoading from "./components/FullPageLoading";
import DocumentTitle from "./components/DocumentTitle";
import AccountPreferenceSync from "./components/AccountPreferenceSync";
import {
  getIdleAdminWarmupTargets,
  scheduleIdleAdminWarmup,
} from "./utils/adminPreload";
import { prefetchAdminDashboard } from "./utils/dashboardPrefetch";

const AdminRoutePreloader = () => {
  const { account } = useAccount();

  React.useEffect(() => {
    if (!account?.logged_in) return;
    const accountKey = account.uuid || account.username || "authenticated";
    const pathname = window.location.pathname.replace(/\/$/, "") || "/";
    if (pathname === "/admin") {
      void prefetchAdminDashboard(accountKey).catch(() => undefined);
    }
    const connection = (
      navigator as Navigator & {
        connection?: { saveData?: boolean; effectiveType?: string };
      }
    ).connection;
    const targets = getIdleAdminWarmupTargets(
      window.location.pathname,
      connection,
    );
    if (targets.length === 0) return;

    let stopWarmup: (() => void) | undefined;
    const startWarmup = () => {
      stopWarmup = scheduleIdleAdminWarmup({
        targets,
        preload: preloadAdminRoute,
        timers: {
          requestIdleCallback: window.requestIdleCallback?.bind(window),
          cancelIdleCallback: window.cancelIdleCallback?.bind(window),
          setTimeout: (callback, delay) =>
            Number(globalThis.setTimeout(callback, delay)),
          clearTimeout: (handle) => globalThis.clearTimeout(handle),
        },
      });
    };

    if (document.readyState === "complete") {
      startWarmup();
    } else {
      window.addEventListener("load", startWarmup, { once: true });
    }

    return () => {
      window.removeEventListener("load", startWarmup);
      stopWarmup?.();
    };
  }, [account?.logged_in]);

  return null;
};

const App = () => {
	const currentPath = window.location.pathname.replace(/\/$/, "");
	const isAdminRoute = currentPath === "/admin" || currentPath.startsWith("/admin/");
	const isUpgradeRoute =
		currentPath === "/admin/update/1.2.7" ||
		currentPath === "/admin/update/storage-v4";
	const isRestrictedGuideRoute =
		isUpgradeRoute ||
		currentPath === "/install" ||
		currentPath === "/database-recovery";
	if (isAdminRoute) {
		preloadAdminEntry(currentPath);
	}
  React.useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const tempKey = params.get("temp_key");

    if (tempKey) {
      document.cookie = `temp_key=${tempKey}; path=/; max-age=${60 * 60 * 24 * 365 * 100}`;
      params.delete("temp_key");
      window.history.replaceState(
        {},
        document.title,
        `${window.location.pathname}${params.toString() ? "?" + params.toString() : ""}`,
      );
    }
  }, []);
  const [appearance, setAppearance] = useLocalStorage<Appearance>(
    "appearance",
    THEME_DEFAULTS.appearance,
  );
  const [color, setColor] = useLocalStorage<Colors>(
    "color",
    THEME_DEFAULTS.color,
  );

  // Use the system theme hook to resolve "system" to actual theme
  const resolvedAppearance = useSystemTheme(appearance);

  React.useEffect(() => {
    const isDark = resolvedAppearance === "dark";
    document.documentElement.classList.toggle("dark", isDark);
  }, [resolvedAppearance]);

  const themeContextValue = useMemo(
    () => ({
      appearance,
      setAppearance,
      color,
      setColor,
    }),
    [appearance, setAppearance, color, setColor],
  );
  const routing = useRoutes(routes);
  return (
    <ThemeContext.Provider value={themeContextValue}>
      <Theme
          appearance={resolvedAppearance}
          accentColor={color}
          scaling="110%"
          className="theme-root"
          style={{
            backgroundColor: "transparent",
            minHeight: "var(--app-viewport-height, 100vh)",
          }}
        >
		{isRestrictedGuideRoute ? (
		  <PublicInfoProvider>
			<DocumentTitle />
			<Toaster />
			<Suspense fallback={<Loading />}>{routing}</Suspense>
		  </PublicInfoProvider>
		) : (
		  <AccountProvider>
			<AccountPreferenceSync />
			<AdminRoutePreloader />
			<RPC2Provider>
			  <PublicInfoProvider>
				<DocumentTitle />
				<NodeListProvider>
				  <Toaster />
				  <OfflineIndicator />
				  <Suspense
					fallback={isAdminRoute ? <FullPageLoading /> : <Loading />}
				  >
					{routing}
				  </Suspense>
				</NodeListProvider>
			  </PublicInfoProvider>
			</RPC2Provider>
		  </AccountProvider>
		)}
      </Theme>
    </ThemeContext.Provider>
  );
};

createRoot(document.getElementById("root")!).render(
  <ErrorBoundary>
    <StrictMode>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </StrictMode>
  </ErrorBoundary>,
);

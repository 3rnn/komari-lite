import React from "react";
import { useLocation } from "react-router-dom";
import {
  ADMIN_ROUTE_PROGRESS_EXIT_MS,
  ADMIN_ROUTE_PROGRESS_SHOW_DELAY_MS,
  getAdminRouteViewKey,
  getAdminRouteProgressHideDelay,
  isAdminRouteViewReady,
  promoteAdminRouteView,
  stageAdminRouteView,
  type RouteViewportState,
} from "@/utils/adminRouteViewport";

const isRouteViewReady = (element: HTMLElement) =>
  isAdminRouteViewReady({
    hasPendingMarker:
      element.querySelector('[data-admin-route-pending="true"]') !== null,
    childElementCount: element.childElementCount,
    textContent: element.textContent,
  });

const AdminRouteViewport = ({
  fallback,
  outlet,
}: {
  fallback: React.ReactNode;
  outlet: React.ReactNode;
}) => {
  const location = useLocation();
  const incomingKey = getAdminRouteViewKey(location);
  const viewElements = React.useRef(new Map<string, HTMLDivElement>());
  const [progressState, setProgressState] = React.useState<"hidden" | "visible" | "leaving">("hidden");
  const progressVisibleAt = React.useRef<number | null>(null);
  const [state, setState] = React.useState<RouteViewportState<React.ReactNode>>(() => ({
    activeKey: incomingKey,
    pendingKey: null,
    views: [{ key: incomingKey, outlet }],
  }));

  React.useLayoutEffect(() => {
    setState((current) => stageAdminRouteView(current, incomingKey, outlet));
  }, [incomingKey, outlet]);

  React.useEffect(() => {
    if (!state.pendingKey) {
      const delay = getAdminRouteProgressHideDelay({
        becameVisibleAt: progressVisibleAt.current,
        now: window.performance.now(),
      });
      const timer = window.setTimeout(() => {
        progressVisibleAt.current = null;
        setProgressState((current) => (current === "hidden" ? current : "leaving"));
      }, delay);
      return () => window.clearTimeout(timer);
    }
    if (progressState === "visible") return;
    if (progressState === "leaving") {
      progressVisibleAt.current = window.performance.now();
      setProgressState("visible");
      return;
    }
    const timer = window.setTimeout(() => {
      progressVisibleAt.current = window.performance.now();
      setProgressState("visible");
    }, ADMIN_ROUTE_PROGRESS_SHOW_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [progressState, state.pendingKey]);

  React.useEffect(() => {
    if (progressState !== "leaving") return;
    const timer = window.setTimeout(() => {
      setProgressState("hidden");
    }, ADMIN_ROUTE_PROGRESS_EXIT_MS);
    return () => window.clearTimeout(timer);
  }, [progressState]);

  React.useLayoutEffect(() => {
    const pendingKey = state.pendingKey;
    if (!pendingKey) return;
    const element = viewElements.current.get(pendingKey);
    if (!element) return;

    let animationFrame = 0;
    let readyFrames = 0;
    let stopped = false;
    const promote = () => {
      setState((current) => promoteAdminRouteView(current, pendingKey));
    };
    const check = () => {
      if (stopped) return;
      if (!isRouteViewReady(element)) {
        readyFrames = 0;
        return;
      }
      readyFrames += 1;
      if (readyFrames >= 2) {
        promote();
        return;
      }
      animationFrame = window.requestAnimationFrame(check);
    };
    const observer = new MutationObserver(() => {
      readyFrames = 0;
      window.cancelAnimationFrame(animationFrame);
      animationFrame = window.requestAnimationFrame(check);
    });
    observer.observe(element, {
      attributes: true,
      attributeFilter: ["data-admin-route-pending"],
      childList: true,
      characterData: true,
      subtree: true,
    });
    animationFrame = window.requestAnimationFrame(check);

    return () => {
      stopped = true;
      observer.disconnect();
      window.cancelAnimationFrame(animationFrame);
    };
  }, [state.pendingKey]);

  return (
    <div className="admin-route-viewport">
      {progressState !== "hidden" ? (
        <div
          aria-label="页面载入中"
          className="admin-route-progress-track"
          data-progress-state={progressState}
          role="status"
        >
          <span className="admin-route-progress-indicator" />
        </div>
      ) : null}
      {state.views.map((view) => {
        const active = view.key === state.activeKey;
        return (
          <div
            key={view.key}
            ref={(element) => {
              if (element) viewElements.current.set(view.key, element);
              else viewElements.current.delete(view.key);
            }}
            aria-hidden={active ? undefined : true}
            className="admin-route-view"
            data-admin-route-active={active ? "true" : "false"}
            inert={active ? undefined : true}
          >
            <React.Suspense fallback={fallback}>{view.outlet}</React.Suspense>
          </div>
        );
      })}
    </div>
  );
};

export default AdminRouteViewport;

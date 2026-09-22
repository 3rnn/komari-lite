import { useEffect, useRef, useState } from "react";
import { useRPC2Call } from "@/contexts/RPC2Context";
import type { LiveDataResponse } from "@/types/LiveData";
import { mergeLatestStatus } from "@/utils/liveData";

export const ADMIN_NODE_LIVE_INTERVAL_MS = 5000;

export function useAdminNodeLiveData() {
  const { call } = useRPC2Call();
  const [liveData, setLiveData] = useState<LiveDataResponse | null>(null);
  const [available, setAvailable] = useState(false);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<number | null>(null);
  const liveDataRef = useRef<LiveDataResponse | null>(null);

  useEffect(() => {
    let timer: number | undefined;
    let stopped = false;
    let running = false;

    const clearTimer = () => {
      if (timer !== undefined) {
        window.clearTimeout(timer);
        timer = undefined;
      }
    };

    const scheduleNext = () => {
      clearTimer();
      if (!stopped && !document.hidden) {
        timer = window.setTimeout(fetchLatest, ADMIN_NODE_LIVE_INTERVAL_MS);
      }
    };

    const fetchLatest = async () => {
      if (running || stopped || document.hidden) return;
      running = true;
      try {
        const result = await call<
          { compact: true },
          Record<string, unknown>
        >("common:getNodesLatestStatus", { compact: true });
        if (stopped) return;
        const next = mergeLatestStatus(result, liveDataRef.current);
        if (next !== liveDataRef.current) {
          liveDataRef.current = next;
          setLiveData(next);
        }
        setAvailable(true);
        setLastUpdatedAt(Date.now());
      } catch {
        if (!stopped) setAvailable(false);
      } finally {
        running = false;
        scheduleNext();
      }
    };

    const handleVisibilityChange = () => {
      if (document.hidden) {
        clearTimer();
      } else if (!running) {
        void fetchLatest();
      }
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    if (!document.hidden) void fetchLatest();

    return () => {
      stopped = true;
      clearTimer();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [call]);

  return { liveData, available, lastUpdatedAt };
}

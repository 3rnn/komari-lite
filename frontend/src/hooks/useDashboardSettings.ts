import React from "react";

import {
  DEFAULT_DASHBOARD_SETTINGS,
  sanitizeDashboardSettings,
  type DashboardSettings,
} from "@/utils/dashboardSettings";
import { readDashboardSession, writeDashboardSession } from "@/utils/dashboardSession";
import { adminRequestInvalidated, getAdminRevocationGeneration, registerAdminCacheClearer, revokeAdminSession } from "@/utils/adminRevocation";

const dashboardSettingsSnapshots = new Map<string, DashboardSettings>();
const pendingDashboardSettingsRequests = new Map<string, Promise<DashboardSettings>>();
registerAdminCacheClearer(() => {
  dashboardSettingsSnapshots.clear();
  pendingDashboardSettingsRequests.clear();
});

function normalizedAccountKey(accountKey?: string): string {
  return accountKey?.trim() || "authenticated";
}

function cachedDashboardSettings(accountKey: string): DashboardSettings | null {
  const memory = dashboardSettingsSnapshots.get(accountKey);
  if (memory) return memory;
  const stored = readDashboardSession<DashboardSettings>("settings", accountKey, "active");
  if (!stored) return null;
  const settings = sanitizeDashboardSettings(stored);
  dashboardSettingsSnapshots.set(accountKey, settings);
  return settings;
}

function readEnvelope(value: unknown): { data?: unknown; message?: unknown; status?: unknown } {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as { data?: unknown; message?: unknown; status?: unknown }
    : {};
}

async function responseError(response: Response, payload: unknown): Promise<Error> {
  const envelope = readEnvelope(payload);
  const message = typeof envelope.message === "string" && envelope.message.trim()
    ? envelope.message
    : `HTTP ${response.status}`;
  return new Error(message);
}

export async function fetchDashboardSettings(options?: {
  signal?: AbortSignal;
  force?: boolean;
  accountKey?: string;
}): Promise<DashboardSettings> {
  const accountKey = normalizedAccountKey(options?.accountKey);
  const snapshot = cachedDashboardSettings(accountKey);
  if (!options?.force && snapshot) return snapshot;
  const pending = pendingDashboardSettingsRequests.get(accountKey);
  if (pending) return pending;

  const generation = getAdminRevocationGeneration();
  const request = fetch("/api/admin/settings/dashboard", {
    cache: "no-store",
    signal: options?.signal,
  }).then(async (response) => {
    adminRequestInvalidated(generation);
    if (response.status === 401) revokeAdminSession();
    let payload: unknown = null;
    try {
      payload = await response.json();
    } catch {
      // The HTTP status below remains the fallback.
    }
    if (!response.ok) throw await responseError(response, payload);
    adminRequestInvalidated(generation);
    const envelope = readEnvelope(payload);
    if (envelope.status !== "success") throw await responseError(response, payload);
    const settings = sanitizeDashboardSettings(envelope.data);
    dashboardSettingsSnapshots.set(accountKey, settings);
    writeDashboardSession("settings", accountKey, "active", settings);
    return settings;
  });

  let tracked: Promise<DashboardSettings>;
  tracked = request.finally(() => {
    if (pendingDashboardSettingsRequests.get(accountKey) === tracked) pendingDashboardSettingsRequests.delete(accountKey);
  });
  pendingDashboardSettingsRequests.set(accountKey, tracked);
  return tracked;
}

export async function saveDashboardSettings(
  settings: DashboardSettings,
  options?: { signal?: AbortSignal; accountKey?: string },
): Promise<DashboardSettings> {
  const accountKey = normalizedAccountKey(options?.accountKey);
  const normalized = sanitizeDashboardSettings(settings);
  const generation = getAdminRevocationGeneration();
  const response = await fetch("/api/admin/settings/dashboard", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(normalized),
    signal: options?.signal,
  });
  adminRequestInvalidated(generation);
  if (response.status === 401) revokeAdminSession();
  let payload: unknown = null;
  try {
    payload = await response.json();
  } catch {
    // The HTTP status below remains the fallback.
  }
  if (!response.ok) throw await responseError(response, payload);
  adminRequestInvalidated(generation);
  const envelope = readEnvelope(payload);
  if (envelope.status !== "success") throw await responseError(response, payload);
  const confirmed = sanitizeDashboardSettings(envelope.data);
  dashboardSettingsSnapshots.set(accountKey, confirmed);
  writeDashboardSession("settings", accountKey, "active", confirmed);
  return confirmed;
}

export function getDashboardSettingsSnapshot(accountKey?: string): DashboardSettings | null {
  return cachedDashboardSettings(normalizedAccountKey(accountKey));
}

export function useDashboardSettings(accountKeyInput?: string) {
  const accountKey = normalizedAccountKey(accountKeyInput);
  const [settings, setSettings] = React.useState<DashboardSettings>(
    () => getDashboardSettingsSnapshot(accountKey) ?? DEFAULT_DASHBOARD_SETTINGS,
  );
  const [loading, setLoading] = React.useState(getDashboardSettingsSnapshot(accountKey) === null);
  const [error, setError] = React.useState<Error | null>(null);
  const activeKey = React.useRef(accountKey);
  activeKey.current = accountKey;

  const refetch = React.useCallback(async (force = false) => {
    const generation = getAdminRevocationGeneration();
    setLoading(true);
    try {
      const next = await fetchDashboardSettings({ force, accountKey });
      if (activeKey.current !== accountKey || generation !== getAdminRevocationGeneration()) return next;
      setSettings(next);
      setError(null);
      return next;
    } catch (reason) {
      const nextError = reason instanceof Error ? reason : new Error(String(reason));
      if (activeKey.current === accountKey && generation === getAdminRevocationGeneration()) setError(nextError);
      throw nextError;
    } finally {
      if (activeKey.current === accountKey && generation === getAdminRevocationGeneration()) setLoading(false);
    }
  }, [accountKey]);

  React.useEffect(() => {
    let cancelled = false;
    const generation = getAdminRevocationGeneration();
    const current = () => !cancelled && activeKey.current === accountKey && generation === getAdminRevocationGeneration();
    const cached = getDashboardSettingsSnapshot(accountKey);
    setSettings(cached ?? DEFAULT_DASHBOARD_SETTINGS);
    setError(null);
    setLoading(cached === null);
    if (!cached) {
      void refetch().catch(() => {});
    } else {
      void fetchDashboardSettings({ force: true, accountKey })
        .then((next) => {
          if (!current()) return;
          setSettings(next);
          setError(null);
        })
        .catch((reason) => {
          if (current()) setError(reason instanceof Error ? reason : new Error(String(reason)));
        });
    }
    return () => { cancelled = true; };
  }, [accountKey, refetch]);

  return { settings, loading, error, refetch };
}

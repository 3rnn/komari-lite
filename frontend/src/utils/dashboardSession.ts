export type DashboardSessionKind = "settings" | "summary" | "charts" | "view";

export type DashboardViewState = {
  scrollTop: number;
  moduleId?: string;
  moduleOffset?: number;
};

type DashboardSessionRecord<T> = {
  key: string;
  savedAt: number;
  data: T;
};

export type DashboardSessionStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export const DASHBOARD_SESSION_MAX_AGE_MS = 10 * 60 * 1000;

function activeSessionStorage(): DashboardSessionStorage | null {
  if (typeof window === "undefined") return null;
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

function storageKey(kind: DashboardSessionKind, accountKey: string): string {
  return `komari:admin-dashboard:v1:${encodeURIComponent(accountKey || "authenticated")}:${kind}`;
}

export function readDashboardSession<T>(
  kind: DashboardSessionKind,
  accountKey: string,
  dataKey: string,
  options?: {
    now?: number;
    maxAgeMs?: number;
    storage?: DashboardSessionStorage | null;
  },
): T | null {
  const storage = options?.storage === undefined ? activeSessionStorage() : options.storage;
  if (!storage) return null;
  const key = storageKey(kind, accountKey);
  try {
    const raw = storage.getItem(key);
    if (!raw) return null;
    const record = JSON.parse(raw) as Partial<DashboardSessionRecord<T>>;
    const now = options?.now ?? Date.now();
    const maxAgeMs = options?.maxAgeMs ?? DASHBOARD_SESSION_MAX_AGE_MS;
    if (
      record.key !== dataKey
      || typeof record.savedAt !== "number"
      || !Number.isFinite(record.savedAt)
      || now - record.savedAt < 0
      || now - record.savedAt > maxAgeMs
      || !("data" in record)
    ) {
      storage.removeItem(key);
      return null;
    }
    return record.data as T;
  } catch {
    storage.removeItem(key);
    return null;
  }
}

export function writeDashboardSession<T>(
  kind: DashboardSessionKind,
  accountKey: string,
  dataKey: string,
  data: T,
  options?: { now?: number; storage?: DashboardSessionStorage | null },
): void {
  const storage = options?.storage === undefined ? activeSessionStorage() : options.storage;
  if (!storage) return;
  try {
    storage.setItem(storageKey(kind, accountKey), JSON.stringify({
      key: dataKey,
      savedAt: options?.now ?? Date.now(),
      data,
    } satisfies DashboardSessionRecord<T>));
  } catch {
    // A full or disabled session store must not block dashboard rendering.
  }
}

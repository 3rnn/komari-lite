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

export type DashboardSessionStorage = Pick<Storage, "getItem" | "setItem" | "removeItem"> & Partial<Pick<Storage, "key" | "length">>;

export const DASHBOARD_SESSION_MAX_AGE_MS = 10 * 60 * 1000;
const DASHBOARD_SESSION_PREFIX = "komari:admin-dashboard:v1:";
let sessionEnded = false;

function activeSessionStorage(): DashboardSessionStorage | null {
  if (typeof window === "undefined") return null;
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

function storageKey(kind: DashboardSessionKind, accountKey: string): string {
  return `${DASHBOARD_SESSION_PREFIX}${encodeURIComponent(accountKey || "authenticated")}:${kind}`;
}

export function purgeLegacyDashboardResponses(options?: { storage?: DashboardSessionStorage | null }): void {
  const storage = options?.storage === undefined ? activeSessionStorage() : options.storage;
  if (!storage) return;
  try {
    if (!storage.key || typeof storage.length !== "number") return;
    for (let index = storage.length - 1; index >= 0; index--) {
      const key = storage.key(index);
      if (key?.startsWith(DASHBOARD_SESSION_PREFIX) && (key.endsWith(":summary") || key.endsWith(":charts"))) {
        storage.removeItem(key);
      }
    }
  } catch {
    // Storage may be unavailable in private browsing or restricted contexts.
  }
}

// The entry bundle imports this module even on the login page: purge prior-version payloads eagerly.
purgeLegacyDashboardResponses();

export function clearDashboardSession(options?: { storage?: DashboardSessionStorage | null }): void {
  if (!options) sessionEnded = true;
  const storage = options?.storage === undefined ? activeSessionStorage() : options.storage;
  if (!storage) return;
  try {
    if (!storage.key || typeof storage.length !== "number") return;
    for (let index = storage.length - 1; index >= 0; index--) {
      const key = storage.key(index);
      if (key?.startsWith(DASHBOARD_SESSION_PREFIX)) storage.removeItem(key);
    }
  } catch {
    // Disabled storage must not prevent logout.
  }
}

export function isDashboardUnauthorized(reason: unknown): boolean {
  return typeof reason === "object" && reason !== null && "status" in reason && reason.status === 401;
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
  let removalAttempted = false;
  const remove = () => {
    removalAttempted = true;
    storage.removeItem(key);
  };
  try {
    if (sessionEnded || kind === "summary" || kind === "charts") {
      remove(); // Drop legacy responses, even if their old TTL is still valid.
      return null;
    }
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
      remove();
      return null;
    }
    return record.data as T;
  } catch {
    if (!removalAttempted) {
      try { remove(); } catch { /* Restricted storage must not interrupt rendering. */ }
    }
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
  if (!storage || sessionEnded) return;
  try {
    if (kind === "summary" || kind === "charts") {
      storage.removeItem(storageKey(kind, accountKey));
      return;
    }
    storage.setItem(storageKey(kind, accountKey), JSON.stringify({
      key: dataKey,
      savedAt: options?.now ?? Date.now(),
      data,
    } satisfies DashboardSessionRecord<T>));
  } catch {
    // A full or disabled session store must not block dashboard rendering.
  }
}

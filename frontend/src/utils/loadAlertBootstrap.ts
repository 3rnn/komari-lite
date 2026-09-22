export interface LoadAlertConfiguration {
  id?: number;
  name?: string;
  clients?: string[];
  default_on?: boolean;
  metric?: "cpu" | "ram" | "disk" | "net_in" | "net_out";
  threshold?: number;
  ratio?: number;
  interval?: number;
  last_notified?: string;
  [property: string]: unknown;
}

export interface LoadAlertBootstrapSnapshot {
  data: LoadAlertConfiguration[] | null;
  error: string | null;
  fetchedAt: number;
}

const LOAD_ALERT_BOOTSTRAP_TTL_MS = 30_000;

export async function fetchLoadAlertConfigurations(): Promise<LoadAlertConfiguration[]> {
  const response = await fetch("/api/admin/notification/load", {
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error("Failed to fetch notification tasks");
  }

  const payload = await response.json();
  return payload && Array.isArray(payload.data) ? payload.data : [];
}

export const createLoadAlertBootstrapResource = (
  loader: () => Promise<LoadAlertConfiguration[]> = fetchLoadAlertConfigurations,
) => {
  const snapshots = new Map<string, LoadAlertBootstrapSnapshot>();
  const pendingRequests = new Map<string, Promise<void>>();

  const start = (accountKey: string): Promise<void> => {
    const pending = pendingRequests.get(accountKey);
    if (pending) return pending;

    const request = loader()
      .then((data) => {
        snapshots.set(accountKey, {
          data,
          error: null,
          fetchedAt: Date.now(),
        });
      })
      .catch((reason: unknown) => {
        snapshots.set(accountKey, {
          data: null,
          error:
            reason instanceof Error
              ? reason.message
              : "An error occurred while fetching load alerts",
          fetchedAt: Date.now(),
        });
      })
      .finally(() => {
        pendingRequests.delete(accountKey);
      });
    pendingRequests.set(accountKey, request);
    return request;
  };

  return {
    read(accountKey: string): LoadAlertBootstrapSnapshot {
      const snapshot = snapshots.get(accountKey);
      if (snapshot) return snapshot;
      throw start(accountKey);
    },
    update(
      accountKey: string,
      data: LoadAlertConfiguration[] | null,
      error: string | null = null,
    ): LoadAlertBootstrapSnapshot {
      const snapshot = { data, error, fetchedAt: Date.now() };
      snapshots.set(accountKey, snapshot);
      return snapshot;
    },
    isStale(snapshot: LoadAlertBootstrapSnapshot, now = Date.now()): boolean {
      return now - snapshot.fetchedAt >= LOAD_ALERT_BOOTSTRAP_TTL_MS;
    },
  };
};

export const loadAlertBootstrapResource = createLoadAlertBootstrapResource();

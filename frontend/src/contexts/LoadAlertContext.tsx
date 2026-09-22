import React from "react";
import { useAccount } from "@/contexts/AccountContext";
import {
  fetchLoadAlertConfigurations,
  loadAlertBootstrapResource,
  type LoadAlertBootstrapSnapshot,
  type LoadAlertConfiguration,
} from "@/utils/loadAlertBootstrap";

export type LoadAlert = LoadAlertConfiguration;

export interface CurrentLoadAlert {
  notification_id: number;
  notification_name: string;
  client: string;
  client_name: string;
  metric: string;
  threshold: number;
  ratio: number;
  interval: number;
  active_since?: string | null;
  last_evaluated_at: string;
  latest_value: number;
  matched_samples: number;
  total_samples: number;
  silenced: boolean;
  silenced_until?: string | null;
  silenced_forever: boolean;
}

interface Response {
  data: LoadAlert[];
  message: string;
  status: string;
  [property: string]: any;
}

interface LoadAlertContextType {
  loadAlerts: LoadAlert[] | null;
  currentAlerts: CurrentLoadAlert[] | null;
  isLoading: boolean;
  currentLoading: boolean;
  error: string | null;
  currentError: string | null;
  refresh: () => Promise<void>;
  refreshCurrent: () => Promise<void>;
}

const LoadAlertContext = React.createContext<LoadAlertContextType | undefined>(
  undefined
);

export const LoadAlertProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const { account } = useAccount();
  const accountKey = account?.uuid || "__authenticated__";
  const initialSnapshot = loadAlertBootstrapResource.read(accountKey);
  const initialSnapshotRef = React.useRef<LoadAlertBootstrapSnapshot>(initialSnapshot);
  const [loadAlerts, setLoadAlerts] = React.useState<LoadAlert[] | null>(
    initialSnapshot.data,
  );
  const loadAlertsRef = React.useRef<LoadAlert[] | null>(initialSnapshot.data);
  const [currentAlerts, setCurrentAlerts] = React.useState<CurrentLoadAlert[] | null>(null);
  const [isLoading, setIsLoading] = React.useState<boolean>(false);
  const [currentLoading, setCurrentLoading] = React.useState<boolean>(false);
  const [error, setError] = React.useState<string | null>(initialSnapshot.error);
  const [currentError, setCurrentError] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await fetchLoadAlertConfigurations();
      initialSnapshotRef.current = loadAlertBootstrapResource.update(accountKey, data);
      loadAlertsRef.current = data;
      setLoadAlerts(data);
    } catch (reason) {
      const message =
        reason instanceof Error
          ? reason.message
          : "An error occurred while fetching load alerts";
      initialSnapshotRef.current = loadAlertBootstrapResource.update(
        accountKey,
        loadAlertsRef.current,
        message,
      );
      setError(message);
    } finally {
      setIsLoading(false);
    }
  }, [accountKey]);

  const refreshCurrent = React.useCallback(async () => {
    setCurrentLoading(true);
    setCurrentError(null);
    try {
      const response = await fetch("/api/admin/notification/load/current", {
        cache: "no-store",
      });
      if (!response.ok) throw new Error("Failed to fetch current load alerts");
      const resp: Response & { data: CurrentLoadAlert[] } = await response.json();
      setCurrentAlerts(resp && Array.isArray(resp.data) ? resp.data : []);
    } catch (reason) {
      const message =
        reason instanceof Error
          ? reason.message
          : "An error occurred while fetching current load alerts";
      setCurrentError(message);
      throw reason instanceof Error ? reason : new Error(message);
    } finally {
      setCurrentLoading(false);
    }
  }, []);

  React.useEffect(() => {
    if (loadAlertBootstrapResource.isStale(initialSnapshotRef.current)) {
      void refresh();
    }
  }, [refresh]);

  return (
    <LoadAlertContext.Provider value={{
      loadAlerts,
      currentAlerts,
      isLoading,
      currentLoading,
      error,
      currentError,
      refresh,
      refreshCurrent,
    }}>
      {children}
    </LoadAlertContext.Provider>
  );
};

export const useLoadAlert = () => {
  const context = React.useContext(LoadAlertContext);
  if (!context) {
    throw new Error("useLoadAlert must be used within a LoadAlertProvider");
  }
  return context;
};

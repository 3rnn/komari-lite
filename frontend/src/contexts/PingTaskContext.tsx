import React from "react";

export interface PingTask {
  clients?: string[];
  default_on?: boolean;
  id?: number;
  interval?: number;
  target?: string;
  type?: string;
  [property: string]: any;
}

interface Response {
  data: PingTask[];
  message: string;
  status: string;
  [property: string]: any;
}

interface PingTaskContextType {
  pingTasks: PingTask[] | null;
  isLoading: boolean;
  error: string | null;
  refresh: () => void;
}

const PingTaskContext = React.createContext<PingTaskContextType | undefined>(
  undefined
);

const PingTaskProviderValue: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [pingTasks, setPingTasks] = React.useState<PingTask[] | null>(null);
  const [isLoading, setIsLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string | null>(null);

  const refresh = React.useCallback(() => {
    setIsLoading(true);
    setError(null);
    fetch("/api/admin/ping")
      .then((response) => {
        if (!response.ok) {
          throw new Error("Failed to fetch ping tasks");
        }
        return response.json();
      })
      .then((resp: Response) => {
        if (resp && Array.isArray(resp.data)) {
          setPingTasks(resp.data);
        } else {
          setPingTasks([]);
        }
      })
      .catch((err) => {
        setError(err.message || "An error occurred while fetching ping tasks");
      })
      .finally(() => {
        setIsLoading(false);
      });
  }, []);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  const value = React.useMemo(
    () => ({ pingTasks, isLoading, error, refresh }),
    [error, isLoading, pingTasks, refresh],
  );

  return (
    <PingTaskContext.Provider value={value}>
      {children}
    </PingTaskContext.Provider>
  );
};

export const PingTaskProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const inherited = React.useContext(PingTaskContext);
  return inherited ? <>{children}</> : (
    <PingTaskProviderValue>{children}</PingTaskProviderValue>
  );
};

export const usePingTask = () => {
  const context = React.useContext(PingTaskContext);
  if (!context) {
    throw new Error("usePingTask must be used within a PingTaskProvider");
  }
  return context;
};

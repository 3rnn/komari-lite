import React from "react";
import {
  fetchAccount,
  saveAccountPreferences,
  type Account,
  type AccountPreferences,
} from "@/utils/adminAuth";

// Context
export interface AccountContextType {
  account: Account | null;
  loading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
  updatePreferences: (preferences: AccountPreferences) => Promise<void>;
}

// 创建Context

const AccountContext = React.createContext<AccountContextType | undefined>(
  undefined,
);

// Provider组件
export const AccountProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [account, setAccount] = React.useState<Account | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<Error | null>(null);

  const refresh = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setAccount(await fetchAccount());
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const updatePreferences = React.useCallback(
    async (preferences: AccountPreferences) => {
      await saveAccountPreferences(preferences);
      setAccount((current) =>
        current?.logged_in
          ? {
              ...current,
              ...(preferences.language
                ? { language: preferences.language }
                : {}),
              ...(preferences.color ? { color: preferences.color } : {}),
            }
          : current,
      );
    },
    [],
  );

  return (
    <AccountContext.Provider
      value={{ account, loading, error, refresh, updatePreferences }}
    >
      {children}
    </AccountContext.Provider>
  );
};

export const useOptionalAccount = () => React.useContext(AccountContext);

// 自定义Hook
export const useAccount = () => {
  const context = useOptionalAccount();
  if (!context) {
    throw new Error("useAccount must be used within an AccountProvider");
  }
  return context;
};

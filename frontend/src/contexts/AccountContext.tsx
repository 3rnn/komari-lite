import React from "react";
import {
  fetchAccount,
  saveAccountPreferences,
  type Account,
  type AccountPreferences,
} from "@/utils/adminAuth";
import { getAdminRevocationGeneration, isAdminLogoutPending, revokeAdminSession, subscribeAdminRevocation } from "@/utils/adminRevocation";

// Context
export interface AccountContextType {
  account: Account | null;
  loading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
  updatePreferences: (preferences: AccountPreferences) => Promise<void>;
}

// Create the context.

const AccountContext = React.createContext<AccountContextType | undefined>(
  undefined,
);

// Account provider.
export const AccountProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [account, setAccount] = React.useState<Account | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<Error | null>(null);

  const requestSequence = React.useRef(0);
  const accountIdentity = React.useRef<string | null>(null);

  React.useEffect(() => subscribeAdminRevocation(() => {
    requestSequence.current++;
    accountIdentity.current = null;
    setAccount({ logged_in: false, uuid: "", username: "", sso_id: "", sso_type: "", "2fa_enabled": false });
    setError(null);
    setLoading(false);
  }), []);

  const refresh = React.useCallback(async () => {
    if (isAdminLogoutPending()) return;
    const sequence = ++requestSequence.current;
    const generation = getAdminRevocationGeneration();
    setLoading(true);
    setError(null);
    try {
      const next = await fetchAccount(fetch, () => sequence === requestSequence.current);
      if (sequence === requestSequence.current && generation === getAdminRevocationGeneration() && !isAdminLogoutPending()) {
        const nextIdentity = next.logged_in ? next.uuid || next.username || "authenticated" : null;
        if (accountIdentity.current !== nextIdentity && (accountIdentity.current !== null || !nextIdentity)) {
          // /api/me can replace an authenticated session without returning 401.
          revokeAdminSession();
        }
        accountIdentity.current = nextIdentity;
        setAccount(next);
        setError(null);
        setLoading(false);
      }
    } catch (err) {
      if (sequence === requestSequence.current && generation === getAdminRevocationGeneration() && !isAdminLogoutPending()) {
        setError(err instanceof Error ? err : new Error(String(err)));
      }
    } finally {
      if (sequence === requestSequence.current && generation === getAdminRevocationGeneration()) setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const updatePreferences = React.useCallback(
    async (preferences: AccountPreferences) => {
      const generation = getAdminRevocationGeneration();
      await saveAccountPreferences(preferences);
      if (generation !== getAdminRevocationGeneration()) return;
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

// Custom hook.
export const useAccount = () => {
  const context = useOptionalAccount();
  if (!context) {
    throw new Error("useAccount must be used within an AccountProvider");
  }
  return context;
};

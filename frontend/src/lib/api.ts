import React from "react";
import { toast } from "sonner";

/**
 * API utility functions for settings management
 */

export interface SettingsResponse {
  sitename: string;
  description: string;
  admin_default_page_size: number;
  reduce_motion: boolean;
  cors_origin_check_enabled: boolean;
  auto_order_new_clients_by_region: boolean;
  geo_ip_enabled: boolean;
  geo_ip_provider: string;
  o_auth_provider: string;
  o_auth_enabled: boolean;
  custom_head: string;
  CreatedAt: string;
  UpdatedAt: string;
  [key: string]: any;
}

const createDefaultSettings = (): SettingsResponse => ({
  sitename: "",
  description: "",
  admin_default_page_size: 10,
  reduce_motion: false,
  cors_origin_check_enabled: true,
  auto_order_new_clients_by_region: false,
  geo_ip_enabled: false,
  geo_ip_provider: "",
  o_auth_provider: "",
  o_auth_enabled: false,
  custom_head: "",
  CreatedAt: "",
  UpdatedAt: "",
});

let pendingSettingsRequest: Promise<SettingsResponse> | null = null;

function getSettingsDeduplicated(): Promise<SettingsResponse> {
  if (pendingSettingsRequest) return pendingSettingsRequest;

  pendingSettingsRequest = getSettings().finally(() => {
    pendingSettingsRequest = null;
  });
  return pendingSettingsRequest;
}

/**
 * Fetch settings from the API
 * @returns Promise containing the settings data
 */
export async function getSettings(): Promise<SettingsResponse> {
  try {
    const response = await fetch("/api/admin/settings");

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    const settingsPayload = data["data"];

    if (
      typeof settingsPayload !== "object" ||
      settingsPayload === null ||
      Array.isArray(settingsPayload)
    ) {
      throw new Error("Invalid settings response payload");
    }

    // Remove database metadata fields that are not needed for UI
    const settings = Object.fromEntries(
      Object.entries(settingsPayload).filter(
        ([key]) => !["CreatedAt", "UpdatedAt", "id"].includes(key),
      ),
    );

    return settings as SettingsResponse;
  } catch (error) {
    console.error("Failed to fetch settings:", error);
    throw error;
  }
}

/**
 * Update settings via the API
 * @param settings - The settings object to update
 * @returns Promise containing the response
 */
export async function updateSettings(
  settings: Partial<SettingsResponse>
): Promise<void> {
  const response = await fetch("/api/admin/settings", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(settings),
  });

  if (!response.ok) {
    let message = `HTTP error! status: ${response.status}`;

    try {
      const errorData = await response.json();
      if (errorData?.message) {
        message = String(errorData.message);
      }
    } catch {
      // Keep the fallback HTTP status message.
    }

    console.error("Failed to update settings:", message);
    throw new Error(message);
  }
}
export async function updateSettingsWithToast(
  settings: Partial<SettingsResponse>,
  t: (key: string) => string
): Promise<void> {
  try {
    await updateSettings(settings);
    toast.success(t("settings.settings_saved"));
  } catch (error) {
    toast.error(t("settings.settings_save_failed") + ": " + error);
    throw error;
  }
}

/**
 * Update a single setting field
 * @param key - The setting key to update
 * @param value - The new value for the setting
 * @param currentSettings - The current settings object (to merge with)
 * @returns Promise containing the response
 */
export async function updateSingleSetting<K extends keyof SettingsResponse>(
  key: K,
  value: SettingsResponse[K],
  currentSettings: SettingsResponse
): Promise<void> {
  const updatedSettings = { ...currentSettings, [key]: value };
  return updateSettings(updatedSettings);
}

/**
 * Hook for managing settings state and API calls
 */
function useSettingsController() {
  const [settings, setSettings] = React.useState<SettingsResponse>(
    createDefaultSettings,
  );

  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  // Fetch settings on mount
  React.useEffect(() => {
    let cancelled = false;

    const fetchSettings = async () => {
      setLoading(true);
      setError(null);
      try {
        const data = await getSettingsDeduplicated();
        if (cancelled) return;
        setSettings(data);
      } catch (err) {
        if (cancelled) return;
        setError(
          err instanceof Error ? err.message : "Failed to fetch settings"
        );
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void fetchSettings();

    return () => {
      cancelled = true;
    };
  }, []);

  // Update a single setting
  const updateSetting = async <K extends keyof SettingsResponse>(
    key: K,
    value: SettingsResponse[K]
  ) => {
    try {
      await updateSingleSetting(key, value, settings);
      setSettings((prev) => ({ ...prev, [key]: value }));
    } catch (err) {
      setError(
        err instanceof Error ? err.message : `Failed to update ${String(key)}`
      );
      throw err;
    }
  };

  // Update multiple settings
  const updateMultipleSettings = async (
    newSettings: Partial<SettingsResponse>
  ) => {
    try {
      const updatedSettings = { ...settings, ...newSettings };
      await updateSettings(updatedSettings);
      setSettings(updatedSettings);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to update settings"
      );
      throw err;
    }
  };

  const refetch = React.useCallback(async () => {
    const data = await getSettings();
    setSettings(data);
    setError(null);
  }, []);

  return {
    settings,
    loading,
    error,
    setSettings,
    updateSetting,
    updateMultipleSettings,
    refetch,
  };
}

type SettingsContextValue = ReturnType<typeof useSettingsController>;

const SettingsContext = React.createContext<SettingsContextValue | null>(null);

export function useReduceMotionPreference(): boolean {
  const context = React.useContext(SettingsContext);
  return Boolean(context?.settings.reduce_motion);
}

export function SettingsProvider({ children }: { children: React.ReactNode }) {
  const value = useSettingsController();
  return React.createElement(SettingsContext.Provider, { value }, children);
}

export function useSettings(): SettingsContextValue {
  const context = React.useContext(SettingsContext);
  if (!context) {
    throw new Error("useSettings must be used within SettingsProvider");
  }
  return context;
}

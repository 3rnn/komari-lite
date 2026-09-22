import React from "react";
import { useAccount } from "@/contexts/AccountContext";
import { ThemeContext } from "@/contexts/ThemeContext";
import i18n from "@/i18n/config";
import {
  normalizeAccountPreferenceColor,
  normalizeAccountPreferenceLanguage,
  type AccountPreferences,
} from "@/utils/adminAuth";
import { readStoredLanguage, writeLanguageCookie } from "@/utils/language";

const AccountPreferenceSync = () => {
  const { account, updatePreferences } = useAccount();
  const { color, setColor } = React.useContext(ThemeContext);
  const syncedAccount = React.useRef<string | null>(null);

  React.useLayoutEffect(() => {
    if (!account?.logged_in || !account.uuid) {
      syncedAccount.current = null;
      return;
    }
    if (syncedAccount.current === account.uuid) return;
    syncedAccount.current = account.uuid;

    const savedLanguage = normalizeAccountPreferenceLanguage(account.language);
    const localLanguage =
      normalizeAccountPreferenceLanguage(readStoredLanguage()) ||
      normalizeAccountPreferenceLanguage(i18n.resolvedLanguage) ||
      normalizeAccountPreferenceLanguage(i18n.language) ||
      "en-US";
    const savedColor = normalizeAccountPreferenceColor(account.color);

    if (savedLanguage) {
      void i18n.changeLanguage(savedLanguage);
      writeLanguageCookie(savedLanguage);
    }
    if (savedColor) setColor(savedColor);

    const initialPreferences: AccountPreferences = {};
    if (!savedLanguage) initialPreferences.language = localLanguage;
    if (!savedColor) initialPreferences.color = color;
    if (initialPreferences.language || initialPreferences.color) {
      void updatePreferences(initialPreferences).catch((error) => {
        console.warn("Failed to initialize account preferences:", error);
      });
    }
  }, [account, color, setColor, updatePreferences]);

  return null;
};

export default AccountPreferenceSync;

import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import en from "./locales/en.json";
import zh_CN from "./locales/zh_CN.json";
import {
  LANGUAGE_STORAGE_KEY,
  readStoredLanguage,
  writeLanguageCookie,
} from "@/utils/language";

// Legacy locale identifiers remain available for stored preferences, but only
// English is offered to new users through the language switcher.
const resources = {
  en: {
    translation: en,
  },
  "en-US": {
    translation: en,
    name: "English",
  },
  "en-GB": {
    translation: en,
  },
  "en-CA": {
    translation: en,
  },
  "en-AU": {
    translation: en,
  },
  "zh-CN": {
    translation: zh_CN,
  },
  "zh-SG": {
    translation: zh_CN,
  },
  "zh-HK": {
    translation: zh_CN,
  },
  "zh-MO": {
    translation: zh_CN,
  },
};

writeLanguageCookie(readStoredLanguage());

i18next.on("languageChanged", (language) => {
  writeLanguageCookie(language);
});

const i18n = i18next;

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: "en-US",
    lng: readStoredLanguage() || "en-US",
    interpolation: {
      escapeValue: false, // React handles XSS
    },
    detection: {
      // Do not detect browser language; use the explicit selection or the existing default.
      order: ["localStorage"],
      caches: ["localStorage"],
      lookupLocalStorage: LANGUAGE_STORAGE_KEY,
    },
  });

export default i18n;
export { resources };

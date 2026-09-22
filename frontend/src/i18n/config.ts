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

// 不添加 name 字段的语言将不会在语言切换菜单中显示
// not adding the name field will hide the language from the language switcher menu
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
    name: "简体中文",
  },
  "zh-SG": {
    translation: zh_CN,  // Singapore uses Simplified Chinese
  },
  "zh-HK": {
    translation: zh_CN,  // 繁体已移除，港澳回落到简体中文
  },
  "zh-MO": {
    translation: zh_CN,  // 同上
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
    // 面板只提供英文与简体中文，默认简体中文
    fallbackLng: "zh-CN",
    lng: readStoredLanguage() || "zh-CN",
    interpolation: {
      escapeValue: false, // React handles XSS
    },
    detection: {
      // 不再按浏览器语言自动切换：只认用户显式选择，其余一律简体中文
      order: ["localStorage"],
      caches: ["localStorage"],
      lookupLocalStorage: LANGUAGE_STORAGE_KEY,
    },
  });

export default i18n;
export { resources };

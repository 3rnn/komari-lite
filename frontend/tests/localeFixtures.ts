import { readdirSync, readFileSync } from "node:fs";

/** 面板当前提供的语言包（直接从目录发现，避免在测试里写死语言清单）。 */
export const AVAILABLE_LOCALES: string[] = readdirSync("src/i18n/locales")
  .filter((name) => name.endsWith(".json"))
  .map((name) => name.replace(/\.json$/, ""))
  .sort();

export function readLocale(name: string): Record<string, unknown> {
  return JSON.parse(readFileSync(`src/i18n/locales/${name}.json`, "utf8"));
}

export const SOURCE_LOCALE = "zh_CN";

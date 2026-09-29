import { readdirSync, readFileSync } from "node:fs";

/** Discover offered locale bundles from the directory instead of hard-coding the list in tests. */
export const AVAILABLE_LOCALES: string[] = readdirSync("src/i18n/locales")
  .filter((name) => name.endsWith(".json"))
  .map((name) => name.replace(/\.json$/, ""))
  .sort();

export function readLocale(name: string): Record<string, unknown> {
  return JSON.parse(readFileSync(`src/i18n/locales/${name}.json`, "utf8"));
}

export const SOURCE_LOCALE = "zh_CN";

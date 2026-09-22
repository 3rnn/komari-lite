export type I18nText = string | Record<string, string>;

export function resolveI18nText(
  text: I18nText | undefined,
  language: string,
): string | undefined {
  if (text === undefined || text === null) return undefined;
  if (typeof text === "string") return text;

  const entries = Object.entries(text).filter(
    ([, value]) => typeof value === "string" && value.trim() !== "",
  );
  if (entries.length === 0) return undefined;

  const normalize = (value: string) =>
    value.trim().replace(/_/g, "-").toLowerCase();
  const languageKey = normalize(language || "");
  const base = languageKey.split("-")[0];
  const candidates = [languageKey, base, "en", "en-us"].filter(Boolean);

  for (const candidate of candidates) {
    const match = entries.find(([key]) => normalize(key) === candidate);
    if (match) return match[1];
  }
  return entries[0][1];
}

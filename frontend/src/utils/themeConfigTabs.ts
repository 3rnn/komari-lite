import type { I18nText } from "@/utils/i18nText";

export interface ThemeConfigTabField {
  key?: string;
  name?: I18nText;
  help?: I18nText;
  type?:
    | "title"
    | "switch"
    | "select"
    | "number"
    | "string"
    | "richtext"
    | "nodes"
    | "pingtasks";
  default?: unknown;
  options?: string;
  optionLabels?: Record<string, I18nText>;
  required?: boolean;
}

export interface ThemeConfigGroup {
  title?: I18nText;
  items: ThemeConfigTabField[];
}

export function groupThemeConfigFields(
  fields: ThemeConfigTabField[],
): ThemeConfigGroup[] {
  const groups: ThemeConfigGroup[] = [];
  let current: ThemeConfigGroup | null = null;

  const commitCurrent = () => {
    if (current && current.items.length > 0) groups.push(current);
  };

  for (const field of fields) {
    if (field.type === "title") {
      commitCurrent();
      current = { title: field.name, items: [] };
      continue;
    }
    if (!field.key) continue;
    if (!current) current = { items: [] };
    current.items.push(field);
  }

  commitCurrent();
  return groups;
}

export function resolveActiveThemeConfigGroup(
  sectionTops: number[],
  activationLine: number,
  atBottom = false,
): number {
  if (sectionTops.length === 0) return 0;
  if (atBottom) return sectionTops.length - 1;

  let active = 0;
  sectionTops.forEach((top, index) => {
    if (top <= activationLine) active = index;
  });
  return active;
}

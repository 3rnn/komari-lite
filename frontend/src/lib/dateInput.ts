const DATE_INPUT_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

export function dateInputToISOString(value: string): string | null {
  const normalized = value.trim();
  if (!normalized) return null;

  const match = DATE_INPUT_PATTERN.exec(normalized);
  if (!match) throw new Error("Invalid date input value");

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(0);
  date.setFullYear(year, month - 1, day);
  date.setHours(0, 0, 0, 0);

  if (
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    throw new Error("Invalid date input value");
  }

  return date.toISOString();
}

export function timestampToDateInput(value: string | number | Date): string {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return "";

  const year = String(date.getFullYear()).padStart(4, "0");
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

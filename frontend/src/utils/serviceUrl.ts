export function normalizeOptionalServiceUrl(
  value: string,
  pageProtocol = window.location.protocol,
): string {
  const trimmed = value.trim().replace(/\/+$/, "");
  if (!trimmed) return "";
  if (/^https?:\/\//i.test(trimmed)) return trimmed;
  const protocol = pageProtocol === "https:" ? "https" : "http";
  return `${protocol}://${trimmed}`;
}

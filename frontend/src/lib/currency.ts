export function currencyForDisplay(currency: string): string {
  const normalized = currency.trim().toUpperCase();
  return normalized === "CAD" || normalized === "CA$" || normalized === "C$"
    ? "C$"
    : currency;
}

export function currencyForStorage(currency: string): string {
  const normalized = currency.trim();
  const upper = normalized.toUpperCase();
  return upper === "CAD" || upper === "CA$" || upper === "C$"
    ? "CAD"
    : normalized;
}

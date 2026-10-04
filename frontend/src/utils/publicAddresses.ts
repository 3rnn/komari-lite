export type PublicIPAddress = {
  address: string;
  family: "ipv4" | "ipv6";
  interface?: string;
  source?: string;
  primary?: boolean;
};

// Older panels/Agents only supply ipv4 and ipv6. The optional structured
// inventory includes primaries, so the UI filters those out before expansion.
export function additionalPublicAddresses(
  addresses: PublicIPAddress[] | undefined | null,
  primaryIPv4: string | undefined,
  primaryIPv6: string | undefined,
): PublicIPAddress[] {
  if (!Array.isArray(addresses)) return [];
  const seen = new Set([primaryIPv4, primaryIPv6].filter(Boolean).map((ip) => ip!.trim().toLowerCase()));
  return addresses.filter((entry) => {
    if (!entry || (entry.family !== "ipv4" && entry.family !== "ipv6") || typeof entry.address !== "string") return false;
    const key = entry.address.trim().toLowerCase();
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return entry.primary !== true;
  });
}

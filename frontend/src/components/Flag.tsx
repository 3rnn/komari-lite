import * as React from "react";
import { Box } from "@radix-ui/themes";
import { getRegionCode } from "@/utils/regionHelper";
import { getAppAssetUrl } from "@/utils/assetUrl";

interface FlagProps {
  flag?: string | null; // Region code (e.g. "SG", "US") or flag emoji (e.g. "🇸🇬", "🇺🇳")
  size?: string; // Optional size prop for future use
  compact?: boolean;
}

/**
 * Convert a two-regional-indicator flag emoji into a two-letter country code.
 * Example: 🇸🇬 (two regional indicators) -> SG
 * @param emoji Flag emoji string
 * @returns Two-letter country code (e.g. "SG") or null for invalid flags.
 */
const getCountryCodeFromFlagEmoji = (emoji?: string | null): string | null => {
  // Array.from handles Unicode surrogate pairs as single characters.
  // National flags contain exactly two regional indicators.
  const chars = Array.from(emoji ?? "");

  // A flag must consist of exactly two regional-indicator characters.
  if (chars.length !== 2) {
    return null;
  }

  // Read the Unicode code points of both indicators.
  const codePoint1 = chars[0].codePointAt(0)!;
  const codePoint2 = chars[1].codePointAt(0)!;

  // Regional indicators range from U+1F1E6 (🇦) to U+1F1FF (🇿).
  const REGIONAL_INDICATOR_START = 0x1F1E6; // Unicode code point for 🇦
  const ASCII_ALPHA_START = 0x41; // ASCII code point for A

  // Check both code points are regional indicators.
  if (
    codePoint1 >= REGIONAL_INDICATOR_START && codePoint1 <= 0x1F1FF &&
    codePoint2 >= REGIONAL_INDICATOR_START && codePoint2 <= 0x1F1FF
  ) {
    // Convert each indicator offset relative to A into its ASCII letter.
    const letter1 = String.fromCodePoint(codePoint1 - REGIONAL_INDICATOR_START + ASCII_ALPHA_START);
    const letter2 = String.fromCodePoint(codePoint2 - REGIONAL_INDICATOR_START + ASCII_ALPHA_START);
    return `${letter1}${letter2}`;
  }

  return null;
};

const Flag = React.memo(({ flag, size, compact = false }: FlagProps) => {
  const resolvedFlagFileName = getCountryCodeFromFlagEmoji(flag) ?? getRegionCode(flag);
  const imgSrc = getAppAssetUrl(`assets/flags/${resolvedFlagFileName}.svg`);
  const altText = `Region flag: ${resolvedFlagFileName}`;

  return (
    <Box
      as="span"
      className={
        compact
          ? "shrink-0 self-center"
          : `m-2 self-center ${size ? `w-${size} h-${size}` : "w-6 h-6"}`
      }
      style={{
        display: "inline-flex",
        alignItems: "center",
        ...(compact ? { width: 20, height: 15 } : {}),
      }}
      aria-label={altText}
    >
      <img
        src={imgSrc}
        alt={altText}
        style={{ width: "100%", height: "100%", objectFit: "contain" }}
      />
    </Box>
  );
});

// Set displayName for easier identification in React DevTools.
Flag.displayName = "Flag";

export default Flag;

import * as React from "react";
import { Box } from "@radix-ui/themes";
import { getRegionCode } from "@/utils/regionHelper";
import { getAppAssetUrl } from "@/utils/assetUrl";

interface FlagProps {
  flag?: string | null; // Region code (e.g. "SG", "US") or flag emoji (e.g. "🇸🇬", "🇺🇳")
  size?: string; // Optional size prop for future use
  compact?: boolean;
}

const Flag = React.memo(({ flag, size, compact = false }: FlagProps) => {
  const resolvedFlagFileName = getRegionCode(flag);
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

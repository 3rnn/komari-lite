/**
 * Convert a size string (e.g. '1.5MB' or '128*1024gb') to bytes.
 * @param str Input string.
 * @returns Byte count, or 0 if parsing fails.
 * @example
 * stringToBytes('1MB');        // 1048576
 * stringToBytes('1 MB');        // 1048576
 * stringToBytes('5.4MB');      // 5662310.4
 * stringToBytes('6,222,765 MB'); // 6525139624935
 * stringToBytes('128*1024gb'); // 140737488355328
 * stringToBytes('1e3kb');       // 1024000 (1000 * 1024)
 * stringToBytes('0.2gb');       // 214748364.8
 * stringToBytes('1024');        // 1024 (bytes by default)
 * stringToBytes('1tb');         // 1099511627776
 */
export function stringToBytes(str: string): number {
  if (typeof str !== "string" || str.length === 0) {
    return 0;
  }
  // Define unit multipliers based on 1024.
  const units: { [key: string]: number } = {
    b: 1,
    byte: 1,
    bytes: 1,
    k: 1024,
    kb: 1024,
    kib: 1024,
    kilobyte: 1024,
    m: 1024 ** 2,
    mb: 1024 ** 2,
    mib: 1024 ** 2,
    megabyte: 1024 ** 2,
    g: 1024 ** 3,
    gb: 1024 ** 3,
    gib: 1024 ** 3,
    gigabyte: 1024 ** 3,
    t: 1024 ** 4,
    tb: 1024 ** 4,
    tib: 1024 ** 4,
    terabyte: 1024 ** 4,
    p: 1024 ** 5,
    pb: 1024 ** 5,
    pib: 1024 ** 5,
    petabyte: 1024 ** 5,
  };

  // 1. Normalize case and remove commas and spaces.
  const cleanStr = str.toLowerCase().replace(/,/g, "").replace(/\s/g, "");

  // 2. Separate the unit from the number.
  // Sort units longest-first so 'kb' matches before 'b'.
  const unitKeys = Object.keys(units).sort((a, b) => b.length - a.length);
  const unitRegex = new RegExp(`(${unitKeys.join("|")})$`);

  let unit = "b"; // Default to bytes.
  let numericPart = cleanStr;

  const match = cleanStr.match(unitRegex);
  if (match) {
    unit = match[1];
    // Remove the unit to obtain the numeric part.
    numericPart = cleanStr.substring(0, cleanStr.length - unit.length);
  }

  // An empty numeric part (e.g. "kb") means 1.
  if (numericPart === "") {
    numericPart = "1";
  }

  // Only accept a numeric literal (including scientific notation). Never
  // evaluate administrator input as JavaScript.
  const value = Number(numericPart.trim());
  if (!Number.isFinite(value) || value < 0) {
    return 0;
  }

  // 4. Multiply by the unit's byte multiplier.
  const multiplier = units[unit];
  return Math.round(value * multiplier);
}

export function formatBytes(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  let size = bytes;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  if (unitIndex === 0) {
    // Bytes have no decimal places.
    return `${Math.round(size)} ${units[unitIndex]}`;
  } else if (unitIndex >= 2 && bytes >= 1024**3) {
    return `${size.toFixed(2)} ${units[unitIndex]}`;
  } else if (size > 99.99) {
    return `${size.toFixed(1)} ${units[unitIndex]}`;
  } else {
    // Small values use two decimal places.
    return `${size.toFixed(2)} ${units[unitIndex]}`;
  }
}

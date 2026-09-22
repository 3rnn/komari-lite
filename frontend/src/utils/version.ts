export function publicVersion(version?: string | null): string {
  const publicValue = version?.split("+", 1)[0] ?? "";
  const snapshot = publicValue.match(
    /^Snapshot-(\d+\.\d+\.\d+(?:\.\d+)?)(?:-\d{10,12})?$/i,
  );
  return snapshot ? `${snapshot[1]} Snapshot` : publicValue;
}

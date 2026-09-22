export type ThemePreviewStatus = "loading" | "loaded" | "error";

type PreviewImageSnapshot = {
  complete: boolean;
  naturalWidth: number;
};

export function resolveThemePreviewStatus(
  src: string | null | undefined,
  image: PreviewImageSnapshot | null,
): ThemePreviewStatus {
  if (!src) return "error";
  if (!image || !image.complete) return "loading";
  return image.naturalWidth > 0 ? "loaded" : "error";
}

export function themePreviewSrc(
  src: string | null | undefined,
  options?: { card?: boolean; version?: string },
): string | undefined {
  if (!src) return undefined;
  const params = new URLSearchParams();
  if (options?.card) params.set("card", "1");
  if (options?.version) params.set("v", options.version);
  const query = params.toString();
  return query ? `${src}${src.includes("?") ? "&" : "?"}${query}` : src;
}

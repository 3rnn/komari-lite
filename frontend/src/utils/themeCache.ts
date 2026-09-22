export const isKomariThemeCacheEntry = (
  rawUrl: string,
  currentOrigin: string,
  mode?: RequestMode,
): boolean => {
  let url: URL;
  try {
    url = new URL(rawUrl, currentOrigin);
  } catch {
    return false;
  }
  if (url.origin !== currentOrigin) return false;
  if (mode === "navigate") return true;
  return (
    url.pathname === "/" ||
    url.pathname === "/index.html" ||
    url.pathname === "/manifest.json" ||
    url.pathname === "/favicon.ico" ||
    url.pathname.startsWith("/themes/") ||
    url.pathname.startsWith("/system-assets/")
  );
};

export async function clearThemeNavigationCache(): Promise<void> {
  try {
    if ("serviceWorker" in navigator) {
      const registration = await navigator.serviceWorker.getRegistration(
        window.location.href,
      );
      await registration?.unregister();
    }
  } catch (reason) {
    console.warn("Unable to unregister the current Komari service worker", reason);
  }

  try {
    if (!("caches" in window)) return;
    const currentOrigin = window.location.origin;
    for (const cacheName of await caches.keys()) {
      const cache = await caches.open(cacheName);
      const requests = await cache.keys();
      await Promise.all(
        requests
          .filter((request) =>
            isKomariThemeCacheEntry(request.url, currentOrigin, request.mode),
          )
          .map((request) => cache.delete(request)),
      );
    }
  } catch (reason) {
    console.warn("Unable to clear cached Komari theme documents", reason);
  }
}

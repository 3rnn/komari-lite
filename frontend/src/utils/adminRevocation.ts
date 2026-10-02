import { clearDashboardSession } from "./dashboardSession.ts";

let generation = 0;
let logoutPending = false;

export function isAdminLogoutPending(): boolean {
  return logoutPending;
}
const cacheClearers = new Set<() => void>();
const subscribers = new Set<() => void>();

export function getAdminRevocationGeneration(): number {
  return generation;
}

export function registerAdminCacheClearer(clear: () => void): () => void {
  cacheClearers.add(clear);
  return () => cacheClearers.delete(clear);
}

export function subscribeAdminRevocation(listener: () => void): () => void {
  subscribers.add(listener);
  return () => subscribers.delete(listener);
}

export function revokeAdminSession(): void {
  generation++;
  for (const clear of cacheClearers) clear();
  clearDashboardSession();
  for (const subscriber of subscribers) subscriber();
}

/** Revoke the view immediately, then delete the HttpOnly session on the server before navigation. */
export async function logoutAdminSession(navigate: () => void): Promise<void> {
  logoutPending = true;
  revokeAdminSession();
  // The same-origin POST route deletes the HttpOnly cookie and server session.
  const response = await fetch("/api/logout", { method: "POST", credentials: "same-origin", cache: "no-store" });
  if (!response.ok) throw new Error(`Logout failed (HTTP ${response.status}); retry before closing this tab`);
  logoutPending = false;
  try {
    navigate();
  } catch {
    // The server cookie is already invalid; keep the mounted view revoked.
  }
}

export function adminRequestInvalidated(generationAtStart: number): void {
  if (generationAtStart !== generation) {
    throw new DOMException("Admin request invalidated", "AbortError");
  }
}

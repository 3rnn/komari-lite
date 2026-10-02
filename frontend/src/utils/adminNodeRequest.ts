import { adminRequestInvalidated, getAdminRevocationGeneration, revokeAdminSession } from "./adminRevocation.ts";

/** Guard both headers and asynchronously consumed bodies against session replacement. */
function guardedResponse(response: Response, generation: number): Response {
  return new Proxy(response, {
    get(target, property) {
      if (property === "clone") return () => {
        adminRequestInvalidated(generation);
        return guardedResponse(target.clone(), generation);
      };
      if (["json", "text", "blob", "arrayBuffer", "formData"].includes(String(property))) {
        return async (...args: unknown[]) => {
          adminRequestInvalidated(generation);
          const method = Reflect.get(target, property) as (...args: unknown[]) => Promise<unknown>;
          const value = await method.apply(target, args);
          adminRequestInvalidated(generation);
          return value;
        };
      }
      const value: unknown = Reflect.get(target, property, target);
      return typeof value === "function" ? value.bind(target) : value;
    },
  });
}

export async function adminNodeRequest(url: string, init: RequestInit, sensitive2FA = false): Promise<Response> {
  const generation = getAdminRevocationGeneration();
  const response = await fetch(url, init);
  adminRequestInvalidated(generation);
  if (response.status === 401) {
    // Credential rotation also returns 401 for a missing or invalid one-time code.
    if (sensitive2FA) {
      const payload = await response.clone().json().catch(() => null);
      adminRequestInvalidated(generation);
      if (payload?.message === "Invalid 2FA code" || payload?.message === "2FA code is required") return response;
    }
    revokeAdminSession();
    throw Object.assign(new Error("HTTP 401"), { status: 401 });
  }
  return guardedResponse(response, generation);
}

/** Keep node mutations tied to the session that issued them. */
export async function postAdminNode(url: string, body?: unknown): Promise<Response> {
  return adminNodeRequest(url, {
    method: "POST",
    ...(body === undefined ? {} : {
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  });
}

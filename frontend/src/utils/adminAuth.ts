import { allowedColors, type Colors } from "../contexts/ThemeContext.ts";

export type Account = {
  logged_in: boolean;
  sso_id: string;
  sso_type: string;
  username: string;
  uuid: string;
  "2fa_enabled": boolean;
  language?: string;
  color?: string;
};

export type AccountPreferences = {
  language?: string;
  color?: Colors;
};

const supportedAccountLanguages = new Set([
  "en-US",
  "zh-CN",
]);

export function normalizeAccountPreferenceLanguage(language?: string | null) {
  const normalized = (language ?? "").trim().replace(/_/g, "-");
  if (supportedAccountLanguages.has(normalized)) return normalized;

  const lower = normalized.toLowerCase();
  if (lower === "en" || lower.startsWith("en-")) return "en-US";
  // 面板只保留英文与简体中文：繁中/日文/印尼文等历史偏好统一回落到简体中文
  if (lower === "zh" || lower.startsWith("zh-")) return "zh-CN";
  return "";
}

export function normalizeAccountPreferenceColor(
  color?: string | null,
): Colors | "" {
  return allowedColors.includes(color as Colors) ? (color as Colors) : "";
}

export type AdminAuthView = "loading" | "error" | "login" | "admin";

type AuthState = {
  account: Pick<Account, "logged_in"> | null;
  loading: boolean;
  error: Error | null;
};

type Fetcher = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export function resolveAdminAuthView({
  account,
  loading,
  error,
}: AuthState): AdminAuthView {
  if (loading) return "loading";
  if (error || !account) return "error";
  return account.logged_in ? "admin" : "login";
}

export function isAdminNodeBootstrapLoading(
  accountLoading: boolean,
  accountKey: string | null,
  loadedAccount: string | null,
  hasPreauthenticatedNodeData = false,
) {
  return (
    accountLoading ||
    Boolean(
      accountKey &&
        loadedAccount !== accountKey &&
        !hasPreauthenticatedNodeData,
    )
  );
}

export async function fetchAccount(
  fetcher: Fetcher = fetch,
): Promise<Account> {
  const response = await fetcher("/api/me", { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`Failed to fetch account data (${response.status})`);
  }
  return response.json() as Promise<Account>;
}

export async function saveAccountPreferences(
  preferences: AccountPreferences,
  fetcher: Fetcher = fetch,
): Promise<void> {
  const response = await fetcher("/api/rpc2", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: 1,
      method: "admin:updateAccountPreferences",
      params: preferences,
    }),
  });
  const payload = (await response.json().catch(() => ({}))) as {
    error?: { message?: string };
  };
  if (!response.ok || payload.error) {
    throw new Error(
      payload.error?.message ||
        `Failed to save account preferences (${response.status})`,
    );
  }
}

type PasswordLoginInput = {
  username: string;
  password: string;
  twoFactorCode?: string;
  fetcher?: Fetcher;
  refreshAccount: () => Promise<void>;
};

export type PasswordLoginResult =
  | { ok: true }
  | { ok: false; message: string; requiresTwoFactor: boolean };

export async function submitPasswordLogin({
  username,
  password,
  twoFactorCode,
  fetcher = fetch,
  refreshAccount,
}: PasswordLoginInput): Promise<PasswordLoginResult> {
  const response = await fetcher("/api/login", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      username,
      password,
      // 只要用户填了动态口令就随请求发送：服务端仅在账号开启 2FA 时校验它。
      // 旧的判断在账号已开启 2FA 时会把口令丢掉，导致验证永远不过，故移除该分支。
      ...(twoFactorCode ? { "2fa_code": twoFactorCode } : {}),
    }),
  });
  const data = (await response.json().catch(() => ({}))) as {
    message?: string;
  };

  if (!response.ok) {
    return {
      ok: false,
      message: data.message || `Login failed (${response.status})`,
      requiresTwoFactor: data.message === "2FA code is required",
    };
  }

  await refreshAccount();
  return { ok: true };
}

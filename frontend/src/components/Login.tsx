import AppDialogContent from "@/components/AppDialogContent";
import * as React from "react";
import {
  Button,
  Callout,
  Dialog,
  Flex,
  IconButton,
  Text,
  TextField,
} from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import { CircleAlert, LoaderCircle, LogIn } from "lucide-react";
import { TablerSettings } from "./Icones/Tabler";
import LanguageSwitch from "./Language";
import LoginIdentityHeader from "./LoginIdentityHeader";
import ThemeSwitch from "./ThemeSwitch";
import {
  AccountProvider,
  useAccount,
  useOptionalAccount,
} from "@/contexts/AccountContext";
import { usePublicInfo } from "@/contexts/PublicInfoContext";
import { submitPasswordLogin } from "@/utils/adminAuth";
import { nextLoginStepState } from "../utils/loginFlow";

type LoginDialogProps = {
  trigger?: React.ReactNode | string;
  autoOpen?: boolean;
  showSettings?: boolean;
  info?: string | React.ReactNode;
  onLoginSuccess?: () => void | Promise<void>;
  redirectAfterLogin?: boolean;
  standalone?: boolean;
};

// 服务端在动态口令错误时返回的原文（web/api/AuthSensitive.go: err2FAInvalid）。
const INVALID_2FA_MESSAGE = "Invalid 2FA code";

const LoginDialogContent = ({
  trigger,
  autoOpen = false,
  showSettings = true,
  info,
  onLoginSuccess,
  redirectAfterLogin = true,
  standalone = false,
}: LoginDialogProps) => {
  const { account, loading, error, refresh } = useAccount();
  const { t } = useTranslation();
  const { publicInfo } = usePublicInfo();
  const [username, setUsername] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [twoFac, setTwoFac] = React.useState("");
  const [errorMsg, setErrorMsg] = React.useState("");
  const [isLoading, setIsLoading] = React.useState(false);
  const [require2FA, setRequire2FA] = React.useState(false);
  const [open, setOpen] = React.useState(autoOpen);

  const passwordLoginEnabled = !publicInfo?.disable_password_login;
  const twoFactorDigits = twoFac.replace(/\D/g, "").slice(0, 6);
  const resetTwoFactorStep = () => {
    setRequire2FA(false);
    setTwoFac("");
    setErrorMsg("");
  };
  const isFormValid =
    passwordLoginEnabled && username.trim() !== "" && password.trim() !== "";

  React.useEffect(() => {
    if (autoOpen) setOpen(true);
  }, [autoOpen]);

  const handleLogin = async () => {
    if (!isFormValid) {
      setErrorMsg(t("login.required"));
      return;
    }
    setErrorMsg("");
    setIsLoading(true);
    try {
      const result = await submitPasswordLogin({
        username,
        password,
        twoFactorCode: twoFac,
        refreshAccount: refresh,
      });
      if (result.ok) {
        await onLoginSuccess?.();
        if (!onLoginSuccess && redirectAfterLogin) {
          window.location.assign("/admin");
        }
        return;
      }
      // 「需要动态口令」是流程信号而不是错误，交给 loginFlow 决定界面状态
      const next = nextLoginStepState({ require2FA, errorMsg }, result);
      setRequire2FA(next.require2FA);
      setErrorMsg(next.errorMsg);
    } catch (err) {
      console.error(err);
      setErrorMsg(t("login.network_error"));
    } finally {
      setIsLoading(false);
    }
  };

  if (loading) {
    return standalone ? (
      <StandaloneShell publicName={publicInfo?.sitename}>
        <Flex align="center" justify="center" gap="2" py="8">
          <LoaderCircle size={18} className="animate-spin" />
          <Text size="2" color="gray">{t("loading")}</Text>
        </Flex>
      </StandaloneShell>
    ) : (
      <Button disabled>{t("loading")}</Button>
    );
  }

  if (error || !account) {
    const retry = (
      <Flex direction="column" gap="3">
        <Callout.Root color="red" role="alert">
          <Callout.Icon><CircleAlert size={16} /></Callout.Icon>
          <Callout.Text>{t("login.account_status_failed")}</Callout.Text>
        </Callout.Root>
        <Button variant="soft" onClick={() => void refresh()}>
          {t("common.retry")}
        </Button>
      </Flex>
    );
    return standalone ? (
      <StandaloneShell publicName={publicInfo?.sitename}>{retry}</StandaloneShell>
    ) : retry;
  }

  if (account.logged_in) {
    if (!showSettings) return null;
    return (
      <a href="/admin" target="_blank" rel="noreferrer">
        <IconButton><TablerSettings /></IconButton>
      </a>
    );
  }

  const loginForm = (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        if (!isLoading) void handleLogin();
      }}
    >
      <Flex direction="column" gap="3">
        {passwordLoginEnabled && require2FA ? (
          <Flex direction="column" gap="1" role="status">
            <Text as="div" size="2" weight="medium">
              {t("login.two_factor_account", { account: username })}
            </Text>
          </Flex>
        ) : null}
        {passwordLoginEnabled && !require2FA ? (
          <>
            <label>
              <Text as="div" size="2" mb="1" weight="medium">
                {t("login.username")}
              </Text>
              <TextField.Root
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                placeholder={t("login.username_placeholder")}
                disabled={isLoading}
                autoComplete="username"
                autoFocus
                size="3"
                className="text-[15px]"
              />
            </label>
            <label>
              <Text as="div" size="2" mb="1" weight="medium">
                {t("login.password")}
              </Text>
              <TextField.Root
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                type="password"
                placeholder={t("login.password_placeholder")}
                disabled={isLoading}
                autoComplete="current-password"
                size="3"
                className="text-[15px]"
              />
            </label>
          </>
        ) : null}
        {passwordLoginEnabled && require2FA ? (
          <Flex direction="column" gap="2">
            <Text as="div" size="2" weight="medium">
              {t("login.two_factor_title", "两步验证")}
            </Text>
            <Text as="div" size="1" color="gray">
              {t(
                "login.two_factor_hint",
                "请打开身份验证器 App，输入当前显示的 6 位动态口令。",
              )}
            </Text>
            <label>
              <Text as="div" size="2" mb="1" weight="medium">
                {t("login.two_factor_code_label", "动态口令")}
              </Text>
              <TextField.Root
                value={twoFactorDigits}
                onChange={(event) =>
                  setTwoFac(event.target.value.replace(/\D/g, "").slice(0, 6))
                }
                inputMode="numeric"
                autoComplete="one-time-code"
                autoFocus
                maxLength={6}
                placeholder="123456"
                disabled={isLoading}
                size="3"
                className="text-center text-[20px] tracking-[0.4em] font-mono"
              />
            </label>
            <Text as="div" size="1" color="gray">
              {t(
                "login.two_factor_trouble",
                "提示：口令 30 秒更新一次，请确认设备时间准确；连续失败会被临时限流。",
              )}
            </Text>
          </Flex>
        ) : null}
        {errorMsg ? (
          <Text size="2" color="red" role="alert">
            {require2FA && errorMsg === INVALID_2FA_MESSAGE
              ? t(
                  "login.two_factor_invalid_hint",
                  "口令不正确或已过期，请输入验证器当前显示的 6 位口令后重试。",
                )
              : errorMsg}
          </Text>
        ) : null}
        <Button
          type="submit"
          size="3"
          disabled={
            isLoading ||
            !isFormValid ||
            (require2FA && twoFactorDigits.length !== 6)
          }
        >
          {isLoading ? (
            <LoaderCircle size={16} className="animate-spin" />
          ) : (
            <LogIn size={16} />
          )}
          {isLoading
            ? t("login.logging_in")
            : require2FA
              ? t("login.two_factor_verify", "验证并登录")
              : t("login.title")}
        </Button>
        {require2FA ? (
          <Button
            type="button"
            variant="soft"
            color="gray"
            size="2"
            disabled={isLoading}
            onClick={resetTwoFactorStep}
          >
            {t("login.two_factor_back", "返回上一步")}
          </Button>
        ) : null}
      </Flex>
    </form>
  );

  if (standalone) {
    return (
      <StandaloneShell publicName={publicInfo?.sitename} info={info}>
        {loginForm}
      </StandaloneShell>
    );
  }

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger>
        {trigger || <Button>{t("login.title")}</Button>}
      </Dialog.Trigger>
      <AppDialogContent maxWidth="420px">
        <LoginIdentityHeader dialog />
        {info ? <Text as="div" size="2" color="gray" mb="4">{info}</Text> : null}
        {loginForm}
      </AppDialogContent>
    </Dialog.Root>
  );
};

const StandaloneShell = ({
  info,
  children,
}: {
  publicName?: string;
  info?: React.ReactNode;
  children: React.ReactNode;
}) => {
  return (
    <div className="w-full max-w-[420px] px-4 py-8 sm:px-0">
      <div className="fixed right-4 top-4 z-10 flex gap-2 sm:right-6 sm:top-6">
        <LanguageSwitch />
        <ThemeSwitch />
      </div>
      <section className="rounded-lg border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] p-5 shadow-lg sm:p-7">
        <LoginIdentityHeader />
        {info ? <Text as="div" size="2" color="gray" mb="4">{info}</Text> : null}
        {children}
      </section>
    </div>
  );
};

const LoginDialog = (props: LoginDialogProps) => {
  const inheritedAccount = useOptionalAccount();
  if (inheritedAccount) return <LoginDialogContent {...props} />;
  return (
    <AccountProvider>
      <LoginDialogContent {...props} />
    </AccountProvider>
  );
};

export default LoginDialog;

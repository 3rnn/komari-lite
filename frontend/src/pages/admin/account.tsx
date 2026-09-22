import AppDialogContent from "@/components/AppDialogContent";
import React from "react";

import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { useAccount } from "@/contexts/AccountContext";
import {
  Button,
  Dialog,
  Flex,
  Skeleton,
  TextField,
} from "@radix-ui/themes";
import Loading from "@/components/loading";
import { SettingCardLabel } from "@/components/admin/SettingCard";
import { AdminSectionTitle } from "@/components/admin/AdminPageTitle";

const Account = () => <InnerLayout />;

const InnerLayout = () => {
  const { t } = useTranslation();
  const { account, loading, error } = useAccount();
  const [usernameSaving, setUsernameSaving] = React.useState(false);
  const [passwordSaving, setPasswordSaving] = React.useState(false);
  const [passwordTwoFa, setPasswordTwoFa] = React.useState("");
  if (loading) {
    return <Loading />;
  }
  if (error) {
    return <div>{error.message}</div>;
  }

  function handleSubmitUsernameChange(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setUsernameSaving(true);
    fetch("/api/admin/update/user", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        uuid: account?.uuid,
        username: (event.currentTarget as HTMLFormElement).username.value,
      }),
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error("Failed to update username");
        }
        return response.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setUsernameSaving(false);
      });
  }
  function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget as HTMLFormElement;
    const password = form.password.value;
    const password_repeat = form.password_repeat.value;
    if (!password || !password_repeat) {
      toast.error(t("account.password_empty_error"));
      return;
    }
    if (password !== password_repeat) {
      toast.error(t("account.password_mismatch_error"));
      return;
    }
    if (password.length < 8) {
      toast.error(t("account.password_too_short_error"));
      return;
    }
    if (!/(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/.test(password)) {
      toast.error(t("account.password_strength_error"));
      return;
    }
    if (account?.["2fa_enabled"] && !passwordTwoFa) {
      toast.error(t("account.otp_empty_error"));
      return;
    }
    setPasswordSaving(true);
    fetch("/api/admin/update/user", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        uuid: account?.uuid,
        password: password,
        "2fa_code": passwordTwoFa,
      }),
    })
      .then(async (response) => {
        if (!response.ok) {
          const data = await response.json();
          throw new Error(data.message || "Failed to update password");
        }
        return response.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
        setPasswordTwoFa("");
        setTimeout(() => {
          window.location.href = "/";
        }, 2000);
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setPasswordSaving(false);
      });
  }
  
  // SSO 辅助函数
  return (
    <Flex gap="4" direction="column" align="stretch">
      <Flex gap="4" direction="column" className="w-full">
        <Flex gap="3" direction="column" className="w-full">
          <AdminSectionTitle>{t("account.title")}</AdminSectionTitle>
          <p className="text-sm leading-6 text-muted-foreground">
            {t("account.greeting", { username: account?.username })}
          </p>
          <form
            className="flex flex-col gap-3 rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] p-4"
            onSubmit={handleSubmitUsernameChange}
          >
            <label className="text-base font-semibold leading-6" htmlFor="username">
              {t("account.change_username_title")}
            </label>

            <TextField.Root
              className="w-full"
              id="username"
              name="username"
              defaultValue={account?.username}
            ></TextField.Root>
            <div>
              <Button disabled={usernameSaving} type="submit">
                {t("account.change_username_button")}
              </Button>
            </div>
          </form>
          <form
            onSubmit={changePassword}
            className="flex flex-col gap-3 rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] p-4"
          >
            <label className="text-base font-semibold leading-6" htmlFor="old_password">
              {t("account.change_password_title")}
            </label>
            <label className="text-sm font-medium" htmlFor="password">
              {t("account.new_password")}
            </label>
            <TextField.Root
              className="w-full"
              id="password"
              name="password"
              type="password"
            ></TextField.Root>
            <label className="text-sm font-medium" htmlFor="password_repeat">
              {t("account.new_password_repeat")}
            </label>
            <TextField.Root
              className="w-full"
              id="password_repeat"
              name="password_repeat"
              type="password"
            ></TextField.Root>
            {account?.["2fa_enabled"] ? (
              <>
                <label className="text-sm font-medium" htmlFor="password_2fa">
                  {t("account.2fa_otp_input_prompt")}
                </label>
                <TextField.Root
                  className="w-full"
                  id="password_2fa"
                  name="password_2fa"
                  type="number"
                  placeholder="000000"
                  value={passwordTwoFa}
                  onChange={(e) =>
                    setPasswordTwoFa((e.target as HTMLInputElement).value)
                  }
                />
              </>
            ) : null}
            <div>
              <Button disabled={passwordSaving} type="submit">
                {t("account.change_password_button")}
              </Button>
            </div>
          </form>
        </Flex>
        <Flex direction="column" className="gap-3">
          <SettingCardLabel>2FA</SettingCardLabel>
          {account?.["2fa_enabled"] ? (
            <TwoFactorEnabled />
          ) : (
            <TwoFactorDisabled></TwoFactorDisabled>
          )}
          <Flex gap="4" align="center" justify="start">
            <label className="text-muted-foreground text-sm">
              {t("account_settings.looking_for_backup")}
            </label>
          </Flex>
        </Flex>
      </Flex>
    </Flex>
  );
};
const TwoFactorDisabled = () => {
  const { t } = useTranslation();
  const { refresh } = useAccount();
  const [saving, setSaving] = React.useState(false);
  const [isOpen, setIsOpen] = React.useState(false);
  const [isLoading, setIsLoading] = React.useState(true);
  const [qrcode, setQRCode] = React.useState<string | null>(null);
  const [code, setCode] = React.useState<string>("");

  React.useEffect(() => {
    if (isOpen) {
      setIsLoading(true);
      fetch("/api/admin/2fa/generate")
        .then((response) => {
          if (!response.ok) {
            throw new Error(t("account.qr_fetch_error"));
          }
          return response.blob();
        })
        .then((blob) => {
          const url = URL.createObjectURL(blob);
          setQRCode(url);
        })
        .catch((err) => toast.error(err.message))
        .finally(() => setIsLoading(false));
    }
  }, [isOpen]);

  const handleEnable2fa = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!code) {
      toast.error(t("account.otp_empty_error"));
      return;
    }
    setSaving(true);
    fetch(`/api/admin/2fa/enable?code=${encodeURIComponent(code)}`, {
      method: "POST",
    })
      .then(async (res) => {
        if (!res.ok) {
          const data = await res.json();
          throw new Error(
            data.message || `Failed to enable 2FA (${res.status})`
          );
        }
        return res.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
        setIsOpen(false);
        refresh();
      })
      .catch((err) => toast.error(err.message))
      .finally(() => setSaving(false));
  };

  return (
    <Flex
      direction="column"
      gap="3"
      className="rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] p-4"
    >
      <label className="text-base font-semibold leading-6">
        {t("account.2fa_disabled")}
      </label>
      <Dialog.Root open={isOpen} onOpenChange={setIsOpen}>
        <Dialog.Trigger>
          <div>
            <Button className="w-full sm:w-auto">{t("account.enable_2fa")}</Button>
          </div>
        </Dialog.Trigger>
        <AppDialogContent>
          <Dialog.Title>{t("account.enable_2fa")}</Dialog.Title>
          <Flex direction="column" gap="2">
            <label>{t("account.2fa_qr_code_hint")}</label>
            <div className="flex justify-center">
              {isLoading ? (
                <Skeleton width="200px" height="200px" />
              ) : (
                <img src={qrcode!} alt="2FA QR Code" width={200} height={200} />
              )}
            </div>
            <label>{t("account.2fa_otp_input_prompt")}</label>
            <form className="flex flex-col gap-2" onSubmit={handleEnable2fa}>
              <TextField.Root
                type="number"
                name="code"
                placeholder="000000"
                value={code}
                onChange={(e) => setCode((e.target as HTMLInputElement).value)}
              />
              <Button disabled={saving} type="submit">
                {t("account.enable_2fa")}
              </Button>
            </form>
          </Flex>
        </AppDialogContent>
      </Dialog.Root>
    </Flex>
  );
};

const TwoFactorEnabled = () => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [code, setCode] = React.useState("");
  const { refresh } = useAccount();
  const disable2fa = () => {
    if (!code) {
      toast.error(t("account.otp_empty_error"));
      return;
    }
    setSaving(true);
    fetch(`/api/admin/2fa/disable?2fa_code=${encodeURIComponent(code)}`, {
      method: "POST",
    })
      .then(async (response) => {
        if (!response.ok) {
          const data = await response.json();
          throw new Error(data.message || "Failed to disable 2FA");
        }
        return response.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
        setIsOpen(false);
        setCode("");
        refresh();
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setSaving(false);
      });
  };
  return (
    <Flex
      direction="column"
      gap="3"
      className="rounded-md border border-[var(--gray-a5)] bg-[var(--color-panel-solid)] p-4"
    >
      <label className="text-base font-semibold leading-6">
        {t("account.2fa_enabled")}
      </label>
      <div>
        <Dialog.Root open={isOpen} onOpenChange={setIsOpen}>
          <Dialog.Trigger>
            <Button color="red">
              {t("account.disable_2fa")}
            </Button>
          </Dialog.Trigger>
          <AppDialogContent>
            <Dialog.Title>{t("account.disable_2fa")}</Dialog.Title>
            <Dialog.Description>
              {t("account.disable_2fa_confirmation")}
            </Dialog.Description>
            <Flex direction="column" gap="2" className="mt-4">
              <label htmlFor="disable_2fa_code">
                {t("account.2fa_otp_input_prompt")}
              </label>
              <TextField.Root
                id="disable_2fa_code"
                type="number"
                placeholder="000000"
                value={code}
                onChange={(e) => setCode((e.target as HTMLInputElement).value)}
              />
            </Flex>
            <Flex gap="2" justify="end" className="mt-4">
              <Button variant="soft" onClick={() => setIsOpen(false)}>
                {t("common.cancel")}
              </Button>
              <Button disabled={saving} color="red" onClick={disable2fa}>
                {t("common.confirm")}
              </Button>
            </Flex>
          </AppDialogContent>
        </Dialog.Root>
      </div>
    </Flex>
  );
};

export default Account;

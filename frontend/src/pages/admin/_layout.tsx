import AppDialogContent from "@/components/AppDialogContent";
import { useOutlet } from "react-router-dom";

import AdminPanelBar from "../../components/admin/AdminPanelBar";
import LoginDialog from "../../components/Login";
import { useAccount } from "@/contexts/AccountContext";
import {
  SettingsProvider,
  updateSettingsWithToast,
  useSettings,
} from "@/lib/api";
import { Button, Callout, Dialog, Flex, Spinner } from "@radix-ui/themes";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert, RefreshCw } from "lucide-react";
import { Eula } from "@/utils/field";
import { normalizeLanguage, readStoredLanguage } from "@/utils/language";
import { resolveAdminAuthView } from "@/utils/adminAuth";
import FullPageLoading from "@/components/FullPageLoading";
import { NodeDetailsProvider } from "@/contexts/NodeDetailsContext";
import { PingTaskProvider } from "@/contexts/PingTaskContext";
import AdminRouteViewport from "@/components/admin/AdminRouteViewport";

const AuthStatusScreen = ({
  failed = false,
  onRetry,
}: {
  failed?: boolean;
  onRetry?: () => void;
}) => {
  const { t } = useTranslation();

  return (
    <Flex
      direction="column"
      align="center"
      justify="center"
      gap="3"
      style={{ minHeight: "100dvh", backgroundColor: "var(--accent-1)" }}
    >
      {failed ? (
        <>
          <Callout.Root color="red" role="alert">
            <Callout.Icon>
              <CircleAlert size={16} />
            </Callout.Icon>
            <Callout.Text>{t("login.account_status_failed")}</Callout.Text>
          </Callout.Root>
          <Button variant="soft" onClick={onRetry}>
            <RefreshCw size={16} />
            {t("common.retry")}
          </Button>
        </>
      ) : null}
    </Flex>
  );
};

const AdminRouteLoading = () => (
  <Flex
    data-admin-route-pending="true"
    align="center"
    justify="center"
    role="status"
    aria-label="页面加载中"
    style={{ minHeight: "min(20rem, 55vh)" }}
  >
    <Spinner size="3" />
  </Flex>
);

const AdminAuthenticatedContent = () => {
  const outlet = useOutlet();
  const { settings, loading, error, setSettings } = useSettings();
  const lang = readStoredLanguage() || "en";
  const [open, setOpen] = useState(false);
  const [accepting, setAccepting] = useState(false);

  useEffect(() => {
    if (loading || error || settings.eula_accepted !== false) {
      setOpen(false);
      return;
    }
    if (normalizeLanguage(lang).startsWith("zh")) {
      setOpen(true);
    }
  }, [loading, error, settings.eula_accepted, lang]);

  const acceptEula = async () => {
    if (accepting) return;
    setAccepting(true);
    try {
      await updateSettingsWithToast(
        { eula_accepted: true },
        (key) => key,
      );
      setSettings((current) => ({ ...current, eula_accepted: true }));
      setOpen(false);
    } catch {
      setOpen(true);
    } finally {
      setAccepting(false);
    }
  };

  return (
    <>
      <Dialog.Root open={open}>
        <AppDialogContent>
          <Dialog.Title>法律声明与合规指引</Dialog.Title>
          <div className="flex flex-col gap-2">
            <div className="max-h-[70vh] overflow-y-auto space-y-4">
              <pre className="text-wrap">{Eula}</pre>
            </div>
            <div className="flex flex-row gap-2 justify-end items-center">
              <Button
                variant="soft"
                color="red"
                onClick={() => window.close()}
              >
                不接受
              </Button>
              <Button
                variant="solid"
                disabled={accepting}
                onClick={() => void acceptEula()}
              >
                我已详细阅读并接受
              </Button>
            </div>
          </div>
        </AppDialogContent>
      </Dialog.Root>
      <AdminPanelBar
        content={
          <AdminRouteViewport
            fallback={<AdminRouteLoading />}
            outlet={outlet}
          />
        }
      />
    </>
  );
};

const AdminAuthenticatedLayout = () => (
  <SettingsProvider>
    <NodeDetailsProvider>
      <PingTaskProvider>
        <AdminAuthenticatedContent />
      </PingTaskProvider>
    </NodeDetailsProvider>
  </SettingsProvider>
);

const AdminGuard = () => {
  const accountState = useAccount();
  const view = resolveAdminAuthView(accountState);

  if (view === "loading") {
    return <FullPageLoading />;
  }
  if (view === "error") {
    return (
      <AuthStatusScreen
        failed
        onRetry={() => {
          void accountState.refresh();
        }}
      />
    );
  }
  if (view === "login") {
    return (
      <Flex
        align="center"
        justify="center"
        style={{ minHeight: "100dvh", backgroundColor: "var(--accent-1)" }}
      >
        <LoginDialog
          autoOpen
          standalone
          showSettings={false}
          redirectAfterLogin={false}
        />
      </Flex>
    );
  }

  return <AdminAuthenticatedLayout />;
};

const AdminLayout = () => <AdminGuard />;

export default AdminLayout;

import { useOutlet } from "react-router-dom";

import AdminPanelBar from "../../components/admin/AdminPanelBar";
import LoginDialog from "../../components/Login";
import { useAccount } from "@/contexts/AccountContext";
import { SettingsProvider } from "@/lib/api";
import { Button, Callout, Flex, Spinner } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import { CircleAlert, RefreshCw } from "lucide-react";
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

  return (
    <AdminPanelBar
      content={
        <AdminRouteViewport
          fallback={<AdminRouteLoading />}
          outlet={outlet}
        />
      }
    />
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

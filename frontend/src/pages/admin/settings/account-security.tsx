import Account from "@/pages/admin/account";
import Sessions from "@/pages/admin/sessions";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import { Flex, Tabs } from "@radix-ui/themes";
import { UserCircle, Users } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";

type AccountSecurityTab = "account" | "sessions";

const ACCOUNT_SECURITY_TABS = new Set<AccountSecurityTab>([
  "account",
  "sessions",
]);

function resolveTab(value: string | null): AccountSecurityTab {
  return ACCOUNT_SECURITY_TABS.has(value as AccountSecurityTab)
    ? (value as AccountSecurityTab)
    : "account";
}

export default function AccountSecuritySettings() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = resolveTab(searchParams.get("tab"));

  const setActiveTab = (value: string) => {
    const tab = resolveTab(value);
    const next = new URLSearchParams(searchParams);
    if (tab === "account") {
      next.delete("tab");
    } else {
      next.set("tab", tab);
    }
    setSearchParams(next, { replace: true });
  };

  return (
    <Flex direction="column" gap="3">
      <AdminPageTitle
        description={t(
          "settings.account_security_page_description",
          "管理管理员账户、登录方式与当前会话。",
        )}
      >
        {t("navigation.account_security")}
      </AdminPageTitle>

      <Tabs.Root value={activeTab} onValueChange={setActiveTab}>
        <div className="w-full overflow-x-auto pb-1">
          <Tabs.List className="w-max min-w-full">
            <Tabs.Trigger value="account" className="min-w-[7.5rem] flex-1">
              <UserCircle size={15} />
              {t("account.title")}
            </Tabs.Trigger>
            <Tabs.Trigger value="sessions" className="min-w-[7.5rem] flex-1">
              <Users size={15} />
              {t("sessions.title")}
            </Tabs.Trigger>
          </Tabs.List>
        </div>

        <Tabs.Content value="account" className="pt-3">
          {activeTab === "account" ? <Account /> : null}
        </Tabs.Content>
        <Tabs.Content value="sessions" className="pt-3">
          {activeTab === "sessions" ? <Sessions /> : null}
        </Tabs.Content>
      </Tabs.Root>
    </Flex>
  );
}

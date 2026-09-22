import { useTranslation } from "react-i18next";
import { Button, Code, Flex, Text, TextField } from "@radix-ui/themes";
import {
  updateSettingsWithToast,
  useSettings,
  type SettingsResponse,
} from "@/lib/api";
import {
  SettingCardButton,
  SettingCardCollapse,
  SettingCardLabel,
  SettingCardSelect,
  SettingCardShortTextInput,
  SettingCardSwitch,
} from "@/components/admin/SettingCard";
import React from "react";
import { toast } from "sonner";
import SettingsPageSkeleton from "@/components/admin/SettingsPageSkeleton";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import {
  ADMIN_LIST_PAGE_SIZE_MAX,
  ADMIN_LIST_PAGE_SIZE_MIN,
  isValidAdminPageSize,
} from "@/utils/adminPagination";

export default function GeneralSettings() {
  const { t } = useTranslation();
  const { settings, loading, error, refetch } = useSettings();
  const [geoip_testResult, setGeoipTestResult] = React.useState<string | null>(
    null
  );
  if (loading) {
    return <SettingsPageSkeleton />;
  }

  if (error) {
    return <Text color="red">{error}</Text>;
  }

  return (
    <>
      <AdminPageTitle
        description={t(
          "settings.general.page_description",
          "配置自动发现、GeoIP 与其他全局行为。",
        )}
      >
        {t("settings.general.title")}
      </AdminPageTitle>
      <SettingCardLabel>
        {t("settings.general.admin_default_page_size")}
      </SettingCardLabel>
      <SettingCardShortTextInput
        description={t("settings.general.admin_default_page_size_description")}
        defaultValue={settings.admin_default_page_size || 10}
        type="number"
        min={ADMIN_LIST_PAGE_SIZE_MIN}
        max={ADMIN_LIST_PAGE_SIZE_MAX}
        step={1}
        OnSave={async (data) => {
          const pageSize = Number(data);
          if (!isValidAdminPageSize(pageSize)) {
            toast.error(
              t("settings.general.admin_default_page_size_invalid", {
                min: ADMIN_LIST_PAGE_SIZE_MIN,
                max: ADMIN_LIST_PAGE_SIZE_MAX,
              }),
            );
            throw new Error("Invalid admin default page size");
          }
          await updateSettingsWithToast(
            { admin_default_page_size: pageSize },
            t,
          );
          await refetch();
        }}
      />
      <SettingCardLabel>
        {t("settings.general.interface_motion")}
      </SettingCardLabel>
      <SettingCardSwitch
        title={t("settings.general.reduce_motion")}
        description={t("settings.general.reduce_motion_description")}
        defaultChecked={Boolean(settings.reduce_motion)}
        onChange={async (checked) => {
          await updateSettingsWithToast({ reduce_motion: checked }, t);
          await refetch();
        }}
      />
      <SettingCardLabel>
        {t("settings.general.auto_discovery")}
      </SettingCardLabel>
      <ApiCard settings={settings} />
      <SettingCardLabel>{t("settings.geoip.title")}</SettingCardLabel>
      <SettingCardSwitch
        title={t("settings.geoip.enable_title")}
        description={t("settings.geoip.enable_description")}
        defaultChecked={settings.geo_ip_enabled}
        onChange={async (checked) => {
          await updateSettingsWithToast({ geo_ip_enabled: checked }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.general.auto_order_new_clients")}
        description={t("settings.general.auto_order_new_clients_description")}
        defaultChecked={Boolean(settings.auto_order_new_clients_by_region)}
        onChange={async (checked) => {
          await updateSettingsWithToast(
            { auto_order_new_clients_by_region: checked },
            t,
          );
        }}
      />
      <SettingCardSelect
        title={t("settings.geoip.provider_title")}
        description={t("settings.geoip.provider_description")}
        defaultValue={settings.geo_ip_provider}
        options={[
          { value: "empty", label: t("common.none") },
          { value: "mmdb", label: "MaxMind" },
          { value: "ip-api", label: "ip-api.com" },
          { value: "geojs", label: "geojs.io" },
          { value: "ipinfo", label: "ipinfo.io" },
        ]}
        OnSave={async (value) => {
          await updateSettingsWithToast({ geo_ip_provider: value }, t);
        }}
      />
      <SettingCardButton
        title={t("settings.geoip.update_title")}
        onClick={async () => {
          const result = await fetch("/api/admin/update/mmdb", {
            method: "POST",
          });
          const data = await result.json();
          if (data.status === "success") {
            toast.success(t("settings.geoip.update_success"));
          } else {
            toast.error(
              data.message || t("settings.geoip.update_error")
            );
          }
        }}
      >
        {t("common.update")}
      </SettingCardButton>
      <SettingCardCollapse
        title={t("settings.geoip.test_title")}
        description={t("settings.geoip.test_description")}
      >
        <Flex className="w-full gap-2" direction="column">
          <TextField.Root placeholder="1.1.1.1 or 2606:4700:4700::1111"></TextField.Root>
          <div>
            <Button
              variant="solid"
              onClick={async () => {
                const ip = (
                  document.querySelector(
                    "input[placeholder]"
                  ) as HTMLInputElement
                ).value;
                const result = await fetch(`/api/admin/test/geoip?ip=${ip}`);
                const data = await result.json();
                setGeoipTestResult(
                  JSON.stringify(data.data, null, 2) || t("common.no_results")
                );
              }}
            >
              {t("settings.geoip.test_button")}
            </Button>
          </div>{" "}
          <Flex className="w-full">
            {geoip_testResult && (
              <Code
                className="w-full whitespace-pre-wrap text-sm p-3 rounded-md overflow-auto max-h-96"
                style={{ display: "block" }}
              >
                {geoip_testResult}
              </Code>
            )}
          </Flex>
        </Flex>
      </SettingCardCollapse>
    </>
  );
}

const ApiCard = ({ settings }: { settings: SettingsResponse }) => {

  //const { settings } = useSettings();
  const { t } = useTranslation();
  const [apiValues, setApiValues] = React.useState<string>(
    settings?.auto_discovery_key || ""
  );

  // 生成32位随机字符串
  const generateRandomString = () => {
    const chars =
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
    let result = "";
    for (let i = 0; i < 24; i++) {
      result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return result;
  };

  // 处理生成按钮点击
  const handleGenerateApiKey = () => {
    const newApiKey = generateRandomString();
    setApiValues(newApiKey);
  };

  // 初始化API值
  React.useEffect(() => {
    if (settings?.auto_discovery_key) {
      setApiValues(settings.auto_discovery_key);
    }
  }, [settings?.auto_discovery_key]);

  return (
    <SettingCardShortTextInput
      title={t("settings.general.auto_discovery_key")}
      description={t("settings.general.auto_discovery_key_description")}
      value={apiValues}
      onChange={(e) => setApiValues(e.target.value)}
      OnSave={async (values) => {
        if (!values) {
          await updateSettingsWithToast({ auto_discovery_key: "" }, t);
          return;
        }
        if (values.length < 12) {
          toast.error(t("settings.api.key_length_error"));
          return;
        }
        await updateSettingsWithToast({ auto_discovery_key: values }, t);
      }}
    >
      <div className="flex flex-row gap-2 justify-start items-center">
        <Button variant="soft" color="green" onClick={handleGenerateApiKey}>
          {t("common.generate")}
        </Button>
        <Button
          variant="soft"
          color="mint"
          onClick={() => {
            window.open(
              "https://nuomiiiii.github.io/komari-document/install/agent-ad",
              "_blank"
            );
          }}
        >
          {t("common.help")}
        </Button>
      </div>
    </SettingCardShortTextInput>
  );
};

import React, { useEffect, useMemo, useState } from "react";
import { Flex, Callout, Button } from "@radix-ui/themes";
import { usePublicInfo } from "@/contexts/PublicInfoContext";
import ThemeConfigTabs from "@/components/admin/ThemeConfigTabs";
import { toast } from "sonner";
import Loading from "@/components/loading";
import { useTranslation } from "react-i18next";
import { resolveI18nText, type I18nText } from "@/utils/i18nText";
import {
  getThemeConfigurationType,
  THEME_CONFIGURATION_MANAGED,
  type ThemeConfiguration,
} from "@/utils/themeConfiguration";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import type { ThemeConfigTabField } from "@/utils/themeConfigTabs";

type ThemeFieldBase = ThemeConfigTabField;

interface ThemeConfigResponse {
  name?: I18nText;
  configuration?: ThemeConfiguration;
  [k: string]: any;
}

const ThemeManaged: React.FC = () => {
  const {
    publicInfo,
    isLoading: publicInfoLoading,
    error: publicInfoError,
    refresh,
  } = usePublicInfo();
  const theme = publicInfo?.theme;
  const themeSettings = publicInfo?.theme_settings || {}; // 当前值
  const { t, i18n } = useTranslation();

  const currentLanguage =
    i18n.resolvedLanguage ||
    i18n.language ||
    (typeof navigator !== "undefined" ? navigator.language : "");

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [fields, setFields] = useState<ThemeFieldBase[]>([]);
  const [values, setValues] = useState<Record<string, any>>({});
  const [themeDisplayName, setThemeDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [firstLoading, setFirstLoading] = useState(true);

  // 拉取主题配置
  useEffect(() => {
    async function load() {
      if (publicInfoLoading || (!publicInfo && !publicInfoError)) {
        setLoading(true);
        return;
      }
      if (publicInfoError) {
        setError(publicInfoError);
        setLoading(false);
        setFirstLoading(false);
        return;
      }
      if (!theme) {
        setFields([]);
        setValues({});
        setThemeDisplayName("");
        setLoading(false);
        setFirstLoading(false);
        return;
      }
      setLoading(true);
      setError(null);
      try {
        const resp = await fetch(`/themes/${theme}/komari-theme.json`, {
          cache: "no-cache",
        });
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const data: ThemeConfigResponse = await resp.json();
        const configuration = data.configuration;
        setThemeDisplayName(
          resolveI18nText(data.name, currentLanguage) ||
            (theme === "default" ? "" : theme),
        );
        if (
          getThemeConfigurationType(configuration) !==
            THEME_CONFIGURATION_MANAGED ||
          !Array.isArray(configuration?.data)
        ) {
          setFields([]);
          setValues({});
          return;
        }
        const ds = configuration.data as ThemeFieldBase[];
        setFields(ds);
        // 初始值：优先 publicInfo.theme_settings，其次 default
        const init: Record<string, any> = {};
        ds.forEach((f) => {
          if (f.type !== "title" && f.key) {
            init[f.key] =
              themeSettings && themeSettings[f.key] !== undefined
                ? themeSettings[f.key]
                : f.default;
          }
        });
        setValues(init);
      } catch (e: any) {
        setError(e.message || t("theme.load_config_failed"));
      } finally {
        setLoading(false);
        setFirstLoading(false);
      }
    }
    load();
  }, [currentLanguage, publicInfo, publicInfoError, publicInfoLoading, theme, themeSettings, t]);

  const handleValueChange = (key: string, val: any) => {
    setValues((v) => ({ ...v, [key]: val }));
  };

  const payload = useMemo(() => {
    // 全量：对所有字段（非 title）输出当前值
    const obj: Record<string, any> = {};
    fields.forEach((f) => {
      if (f.type === "title" || !f.key) return;
      const current = values[f.key];
      // 直接使用当前值，undefined 时才用默认值
      if (current !== undefined) {
        obj[f.key] = current;
      } else if (f.default !== undefined) {
        obj[f.key] = f.default;
      } else {
        obj[f.key] = "";
      }
    });
    return obj;
  }, [fields, values]);

  const saveAll = async () => {
    if (!theme) return;
    console.log("保存前的 values:", values);
    console.log("保存前的 payload:", payload);
    setSaving(true);
    try {
      const resp = await fetch(
        `/api/admin/theme/settings?theme=${encodeURIComponent(theme)}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      if (!resp.ok) {
        const d = await resp.json().catch(() => ({ message: "unknown" }));
        throw new Error(d.message || `HTTP ${resp.status}`);
      }
      toast.success(t("settings.settings_saved"));
      // 刷新 publicInfo 以反映最新设置
      refresh();
    } catch (e: any) {
      toast.error(`${t("settings.settings_save_failed")}: ${e.message || e}`);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Flex
      direction="column"
      gap="4"
      className="km-page-admin-theme-managed p-0 md:p-4"
    >
      <Flex justify="between" align="center" gap="3" wrap="wrap">
        <AdminPageTitle description={t("theme.manage_description", "调整当前主题提供的显示和功能选项。")}> 
          {theme
            ? t("theme.manage_with_name", {
                name: themeDisplayName,
              })
            : t("theme.manage")}
        </AdminPageTitle>
        {fields.length > 0 && (
          <Button onClick={saveAll} disabled={saving}>
            {t("common.save")}
          </Button>
        )}
      </Flex>
      {error && (
        <Callout.Root color="red">
          <Callout.Text>{error}</Callout.Text>
        </Callout.Root>
      )}
      {loading && firstLoading && <Loading />}
      {!loading && !error && fields.length === 0 && theme !== "default" && (
        <Callout.Root>
          <Callout.Text>{t("theme.no_config")}</Callout.Text>
        </Callout.Root>
      )}
      {fields.length > 0 && (
        <ThemeConfigTabs
          fields={fields}
          values={values}
          onValueChange={handleValueChange}
          resolveText={(value) => resolveI18nText(value, currentLanguage)}
          footer={
            <Flex>
              <Button onClick={saveAll} disabled={saving}>
                {t("common.save")}
              </Button>
            </Flex>
          }
        />
      )}
    </Flex>
  );
};

export default ThemeManaged;

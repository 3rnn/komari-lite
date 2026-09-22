import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Box, Button, Flex, Heading, Tabs } from "@radix-ui/themes";
import { ChevronLeft, ChevronRight, ListChecks } from "lucide-react";
import { motion } from "motion/react";
import { useTranslation } from "react-i18next";
import {
  SettingCard,
  SettingCardLongTextInput,
  SettingCardSelect,
  SettingCardShortTextInput,
  SettingCardSwitch,
} from "@/components/admin/SettingCard";
import NodeSelectorDialog from "@/components/NodeSelectorDialog";
import PingTaskSelectorDialog from "@/components/PingTaskSelectorDialog";
import type { I18nText } from "@/utils/i18nText";
import {
  groupThemeConfigFields,
  type ThemeConfigTabField,
} from "@/utils/themeConfigTabs";
import { useSettings } from "@/lib/api";

interface ThemeConfigTabsProps {
  fields: ThemeConfigTabField[];
  values: Record<string, any>;
  onValueChange: (key: string, value: any) => void;
  resolveText: (value?: I18nText) => string | undefined;
  footer?: ReactNode;
}

const ThemeConfigTabs = ({
  fields,
  values,
  onValueChange,
  resolveText,
  footer,
}: ThemeConfigTabsProps) => {
  const { t } = useTranslation();
  const { settings } = useSettings();
  const reduceMotion = Boolean(settings.reduce_motion);
  const groups = useMemo(() => groupThemeConfigFields(fields), [fields]);
  const [activeTab, setActiveTab] = useState(0);
  const [tabDirection, setTabDirection] = useState(1);
  const [scrollEdges, setScrollEdges] = useState({ left: false, right: false });
  const tabsListRef = useRef<HTMLDivElement | null>(null);
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);

  useEffect(() => {
    setActiveTab(0);
    tabRefs.current = tabRefs.current.slice(0, groups.length);
  }, [groups]);

  const updateScrollEdges = useCallback(() => {
    const list = tabsListRef.current;
    if (!list) return;
    const maxScrollLeft = Math.max(0, list.scrollWidth - list.clientWidth);
    setScrollEdges({
      left: list.scrollLeft > 1,
      right: list.scrollLeft < maxScrollLeft - 1,
    });
  }, []);

  useEffect(() => {
    const list = tabsListRef.current;
    if (!list) return;
    updateScrollEdges();
    const observer = new ResizeObserver(updateScrollEdges);
    observer.observe(list);
    list.addEventListener("scroll", updateScrollEdges, { passive: true });
    window.addEventListener("resize", updateScrollEdges);
    return () => {
      observer.disconnect();
      list.removeEventListener("scroll", updateScrollEdges);
      window.removeEventListener("resize", updateScrollEdges);
    };
  }, [groups, updateScrollEdges]);

  useEffect(() => {
    const list = tabsListRef.current;
    const tab = tabRefs.current[activeTab];
    if (!list || !tab) return;
    const listRect = list.getBoundingClientRect();
    const tabRect = tab.getBoundingClientRect();
    if (tabRect.left < listRect.left) {
      list.scrollBy({
        left: tabRect.left - listRect.left - 12,
        behavior: reduceMotion ? "auto" : "smooth",
      });
    } else if (tabRect.right > listRect.right) {
      list.scrollBy({
        left: tabRect.right - listRect.right + 12,
        behavior: reduceMotion ? "auto" : "smooth",
      });
    }
    window.requestAnimationFrame(updateScrollEdges);
  }, [activeTab, reduceMotion, updateScrollEdges]);

  const scrollTabs = (direction: -1 | 1) => {
    const list = tabsListRef.current;
    if (!list) return;
    list.scrollBy({
      left: direction * Math.max(160, list.clientWidth * 0.7),
      behavior: reduceMotion ? "auto" : "smooth",
    });
  };

  const handleTabChange = (value: string) => {
    const index = Number(value);
    if (Number.isNaN(index) || index < 0 || index >= groups.length) return;
    setTabDirection(index >= currentTab ? 1 : -1);
    setActiveTab(index);
  };

  const renderField = (field: ThemeConfigTabField) => {
    const key = field.key!;
    const value = values[key];
    const title = resolveText(field.name);
    const description = resolveText(field.help);

    switch (field.type) {
      case "nodes": {
        const selected = (Array.isArray(value)
          ? value
          : parseSelectorValue(value)
        ).map(String);
        return (
          <Box key={key} id={key}>
            <SettingCard title={title} description={description}>
              <SettingCard.Action>
                <NodeSelectorDialog
                  value={selected}
                  onChange={(ids) => onValueChange(key, ids)}
                  title={title}
                >
                  <Button variant="soft">
                    <ListChecks size={16} />
                    {t("common.selected", { count: selected.length })}
                  </Button>
                </NodeSelectorDialog>
              </SettingCard.Action>
            </SettingCard>
          </Box>
        );
      }
      case "pingtasks": {
        const selected = (Array.isArray(value)
          ? value
          : parseSelectorValue(value)
        )
          .map(Number)
          .filter((id) => Number.isInteger(id) && id > 0);
        return (
          <Box key={key} id={key}>
            <SettingCard title={title} description={description}>
              <SettingCard.Action>
                <PingTaskSelectorDialog
                  value={selected}
                  onChange={(ids) => onValueChange(key, ids)}
                  title={title}
                >
                  <Button variant="soft">
                    <ListChecks size={16} />
                    {t("common.selected", { count: selected.length })}
                  </Button>
                </PingTaskSelectorDialog>
              </SettingCard.Action>
            </SettingCard>
          </Box>
        );
      }
      case "switch":
        return (
          <Box key={key} id={key}>
            <SettingCardSwitch
              title={title}
              description={description}
              defaultChecked={Boolean(value)}
              onChange={(checked) => onValueChange(key, checked)}
            />
          </Box>
        );
      case "select": {
        const options = (field.options || "")
          .split(",")
          .map((option) => option.trim())
          .filter(Boolean)
          .map((option) => ({
            value: option,
            label: resolveText(field.optionLabels?.[option]),
          }));
        const selectedValue = value === undefined ? "" : String(value);
        const selectedLabel =
          options.find((option) => option.value === selectedValue)?.label ||
          selectedValue;
        return (
          <Box key={key} id={key}>
            <SettingCardSelect
              title={title}
              description={description}
              value={selectedValue}
              options={options}
              OnSave={(next) => onValueChange(key, next)}
              label={selectedLabel || t("common.select")}
            />
          </Box>
        );
      }
      case "number":
        return (
          <Box key={key} id={key}>
            <SettingCardShortTextInput
              title={title}
              description={description}
              type="number"
              showSaveButton={false}
              value={value !== undefined ? String(value) : ""}
              onChange={(event) =>
                onValueChange(
                  key,
                  event.target.value === ""
                    ? undefined
                    : Number(event.target.value),
                )
              }
            />
          </Box>
        );
      case "richtext":
        return (
          <Box key={key} id={key}>
            <SettingCardLongTextInput
              title={title}
              description={description}
              defaultValue={value !== undefined ? String(value) : ""}
              showSaveButton={false}
              onChange={(event) => onValueChange(key, event.target.value)}
            />
          </Box>
        );
      case "string":
      default:
        return (
          <Box key={key} id={key}>
            <SettingCardShortTextInput
              title={title}
              description={description}
              value={value !== undefined ? String(value) : ""}
              required={field.required}
              showSaveButton={false}
              onChange={(event) => onValueChange(key, event.target.value)}
            />
          </Box>
        );
    }
  };

  const currentTab = Math.min(activeTab, Math.max(groups.length - 1, 0));
  const activeGroup = groups[currentTab];

  return (
    <Flex direction="column" gap="4" className="km-theme-config-form">
      {groups.length > 1 && (
        <Box className="km-theme-config-tabs">
          <Tabs.Root
            value={String(currentTab)}
            onValueChange={handleTabChange}
            className="km-theme-config-tabs-root"
          >
            <div
              className={`km-theme-config-tabs-viewport${scrollEdges.left ? " can-scroll-left" : ""}${scrollEdges.right ? " can-scroll-right" : ""}`}
            >
              <Tabs.List
                ref={tabsListRef}
                className="km-theme-config-tabs-list"
              >
                {groups.map((group, index) => (
                  <Tabs.Trigger
                    key={index}
                    ref={(element) => {
                      tabRefs.current[index] = element;
                    }}
                    value={String(index)}
                    className="km-theme-config-tabs-trigger"
                  >
                    {group.title
                      ? resolveText(group.title) || t("common.title")
                      : t("settings.general.title")}
                  </Tabs.Trigger>
                ))}
              </Tabs.List>
              {scrollEdges.left ? (
                <button
                  type="button"
                  className="km-theme-config-scroll-button is-left"
                  title={t("common.previous", "向左滚动")}
                  aria-label={t("common.previous", "向左滚动")}
                  onClick={() => scrollTabs(-1)}
                >
                  <ChevronLeft size={16} aria-hidden="true" />
                </button>
              ) : null}
              {scrollEdges.right ? (
                <button
                  type="button"
                  className="km-theme-config-scroll-button is-right"
                  title={t("common.next", "向右滚动")}
                  aria-label={t("common.next", "向右滚动")}
                  onClick={() => scrollTabs(1)}
                >
                  <ChevronRight size={16} aria-hidden="true" />
                </button>
              ) : null}
            </div>
          </Tabs.Root>
        </Box>
      )}

      {activeGroup && (
        <motion.div
          key={currentTab}
          className="km-theme-config-section"
          initial={reduceMotion ? false : { opacity: 0.35, x: tabDirection * 8 }}
          animate={{ opacity: 1, x: 0 }}
          transition={
            reduceMotion
              ? { duration: 0 }
              : { duration: 0.18, ease: [0.22, 1, 0.36, 1] }
          }
        >
          {activeGroup.title && (
            <Heading size="3">
              {resolveText(activeGroup.title) || t("common.title")}
            </Heading>
          )}
          <Flex direction="column" gap="3" className="mt-5 mb-3">
            {activeGroup.items.map(renderField)}
          </Flex>
        </motion.div>
      )}
      {footer}
    </Flex>
  );
};

function parseSelectorValue(value: unknown): unknown[] {
  if (typeof value !== "string" || value.trim() === "") return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export default ThemeConfigTabs;

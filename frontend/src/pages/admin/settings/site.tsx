import AppDialogContent from "@/components/AppDialogContent";
import { useTranslation } from "react-i18next";
import { Button, Dialog, Flex, Text } from "@radix-ui/themes";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import {
  SettingCardButton,
  SettingCardCollapse,
  SettingCardLabel,
  SettingCardLongTextInput,
  SettingCardShortTextInput,
  SettingCardSwitch,
} from "@/components/admin/SettingCard";
import { toast } from "sonner";
import SettingsPageSkeleton from "@/components/admin/SettingsPageSkeleton";
import { useRef, useState } from "react";
import AdminPageTitle from "@/components/admin/AdminPageTitle";
import UploadDialog from "@/components/UploadDialog";
import { uploadArchive } from "@/utils/archiveUpload";
import {
  UPLOAD_COMPLETED_VISIBLE_MS,
  UPLOAD_DIALOG_EXIT_MS,
  createCompletedUploadState,
  createProcessingUploadState,
  delay,
  type UploadProgressState,
  withUploadProgressCopy,
} from "@/utils/uploadProgress";

export default function SiteSettings() {
  const { t } = useTranslation();
  const { settings, loading, error, refetch } = useSettings();
  const [shareHours, setShareHours] = useState(1);
  const [faviconRevision, setFaviconRevision] = useState(() => Date.now());

  const refreshFavicon = () => {
    const revision = Date.now();
    setFaviconRevision(revision);
    document.querySelectorAll<HTMLLinkElement>('link[rel*="icon"]').forEach((link) => {
      const href = link.getAttribute("href") || "/favicon.ico";
      const url = new URL(href, window.location.origin);
      if (url.pathname.endsWith("/favicon.ico")) {
        url.searchParams.set("v", String(revision));
        link.href = `${url.pathname}${url.search}${url.hash}`;
      }
    });
  };

  // 恢复备份对话框与上传状态
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restoreState, setRestoreState] = useState<UploadProgressState | null>(
    null,
  );
  const restoreController = useRef<AbortController | null>(null);
  const restoreStateRef = useRef<UploadProgressState | null>(null);
  const restoreCopy = {
    preparing: t("settings.site.phase_preparing", "Preparing backup"),
    uploading: t("settings.site.phase_uploading", "Uploading backup"),
    merging: t(
      "settings.site.phase_processing",
      "Restoring data on the server",
    ),
    processing: t(
      "settings.site.phase_processing",
      "Restoring data on the server",
    ),
    restarting: t("settings.site.phase_restarting", "Restarting service"),
    completed: t("settings.site.phase_completed", "Backup restored"),
    failed: t("settings.site.backup_restore_error", "Restore backup failed"),
    nonCancelable: t(
      "settings.site.phase_non_cancelable",
      "Server processing has started and can no longer be canceled",
    ),
  };

  const setTrackedRestoreState = (state: UploadProgressState | null) => {
    restoreStateRef.current = state;
    setRestoreState(state);
  };

  const downloadBackup = async (scope: "full" | "config") => {
    const response = await fetch(`/api/admin/download/backup?scope=${scope}`);
    if (!response.ok) {
      const body = await response.text();
      let message = body || `HTTP ${response.status}`;
      try {
        message = JSON.parse(body)?.message || message;
      } catch {
        // Keep the server response as-is when it is not JSON.
      }
      toast.error(message);
      throw new Error(message);
    }

    const disposition = response.headers.get("content-disposition") || "";
    const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1]
      || `Komari-Lite-${scope}.zip`;
    const objectURL = URL.createObjectURL(await response.blob());
    const anchor = document.createElement("a");
    anchor.href = objectURL;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    window.setTimeout(() => URL.revokeObjectURL(objectURL), 1_000);
  };

  const uploadBackup = async (file: File) => {
    if (!file.name.endsWith(".zip")) {
      toast.error(t("theme.invalid_file_type", "仅支持 .zip 文件"));
      return;
    }

    setTrackedRestoreState(null);
    const controller = new AbortController();
    restoreController.current = controller;
    try {
      await uploadArchive({
        basePath: "/api/admin/upload",
        purpose: "backup",
        file,
        signal: controller.signal,
        onStateChange: (state) => {
          const nextState =
            state.stage === "merging"
              ? createProcessingUploadState(state)
              : state;
          setTrackedRestoreState(withUploadProgressCopy(nextState, restoreCopy));
        },
      });
      setTrackedRestoreState(
        withUploadProgressCopy(
          createCompletedUploadState(restoreStateRef.current),
          restoreCopy,
        ),
      );
      toast.success(t("account_settings.upload_success", "上传成功"));
      await delay(UPLOAD_COMPLETED_VISIBLE_MS);
      setRestoreOpen(false);
      await delay(UPLOAD_DIALOG_EXIT_MS);
      setTrackedRestoreState(null);
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") {
        setTrackedRestoreState(null);
      } else {
        toast.error(
          reason instanceof Error
            ? reason.message
            : t("settings.site.backup_restore_error", "恢复备份失败"),
        );
      }
    } finally {
      restoreController.current = null;
    }
  };

  const cancelRestore = () => {
    restoreController.current?.abort();
    setTrackedRestoreState(null);
  };

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
          "settings.site.page_description",
          "管理站点信息、访问策略、备份与接口安全设置。",
        )}
      >
        {t("settings.site.title")}
      </AdminPageTitle>
      <SettingCardShortTextInput
        title={t("settings.site.name")}
        description={t("settings.site.name_description")}
        defaultValue={settings.sitename || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ sitename: data }, t);
        }}
      />
      <SettingCardLongTextInput
        title={t("settings.site.description")}
        description={t("settings.site.description_description")}
        defaultValue={settings.description || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ description: data }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.site.cors_origin_check_enabled")}
        description={t("settings.site.cors_origin_check_enabled_description")}
        defaultChecked={settings.cors_origin_check_enabled ?? true}
        onChange={async (checked) => {
          await updateSettingsWithToast({ cors_origin_check_enabled: checked }, t);
        }}
      />
      <SettingCardLongTextInput
        title={t("settings.site.cors_allowed_origins", "API CORS 允许列表")}
        description={t(
          "settings.site.cors_allowed_origins_description",
          "每行或用逗号分隔一个 Origin，例如 https://example.com",
        )}
        defaultValue={settings.cors_allowed_origins || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ cors_allowed_origins: data }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.site.ws_origin_check_enabled", "WebSocket Origin 校验")}
        description={t(
          "settings.site.ws_origin_check_enabled_description",
          "开启后 WebSocket 请求只允许同源或允许列表中的 Origin",
        )}
        defaultChecked={settings.ws_origin_check_enabled ?? true}
        onChange={async (checked) => {
          await updateSettingsWithToast(
            { ws_origin_check_enabled: checked },
            t,
          );
        }}
      />
      <SettingCardLongTextInput
        title={t("settings.site.ws_allowed_origins", "WebSocket Origin 允许列表")}
        description={t(
          "settings.site.ws_allowed_origins_description",
          "每行或用逗号分隔一个 Origin，例如 https://example.com",
        )}
        defaultValue={settings.ws_allowed_origins || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ ws_allowed_origins: data }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.site.send_ip_addr_to_guest")}
        description={t("settings.site.send_ip_addr_to_guest_description")}
        defaultChecked={settings.send_ip_addr_to_guest}
        onChange={async (checked) => {
          await updateSettingsWithToast({ send_ip_addr_to_guest: checked }, t);
        }}
      />
      <SettingCardShortTextInput
        title={t("settings.site.script_domain")}
        description={t("settings.site.script_domain_description")}
        placeholder={`${window.location.origin}`}
        defaultValue={settings.script_domain || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ script_domain: data }, t);
        }}
      />
      <SettingCardLabel>{t("settings.site.private_site")}</SettingCardLabel>
      <SettingCardSwitch
        title={t("settings.site.private_site")}
        description={t("settings.site.private_site_description")}
        defaultChecked={settings.private_site}
        onChange={async (checked) => {
          await updateSettingsWithToast({ private_site: checked }, t);
        }}
      />
      <SettingCardCollapse
        title={t("settings.site.tempory_share")}
        description={t("settings.site.tempory_share_description")}
      >
        <div className="flex w-full flex-col gap-4">
          <SettingCardShortTextInput
            title={t("settings.site.tempory_share_current_link")}
            value={
              settings.tempory_share_token
                ? `${window.location.origin}/?temp_key=${settings.tempory_share_token}`
                : ""
            }
            showSaveButton={false}
            description={`${t("admin.nodeTable.expiredAt")}: ${new Date((settings.tempory_share_token_expire_at || 0) * 1000).toLocaleString()}`}
            disabled
            bordless
          >
            <Button
              onClick={() => {
                if (!settings.tempory_share_token) return;
                navigator.clipboard.writeText(
                  `${window.location.origin}/?temp_key=${settings.tempory_share_token}`,
                );
                toast.success(t("copy"));
              }}
            >
              {t("copy")}
            </Button>
          </SettingCardShortTextInput>
          <SettingCardShortTextInput
            title={t("settings.site.tempory_share_hours")}
            bordless
            showSaveButton={false}
            value={shareHours}
            type="number"
            onChange={(e) => {
              const val = Number.parseInt(e.target.value, 10);
              if (!Number.isNaN(val)) {
                setShareHours(val);
              }
            }}
          ></SettingCardShortTextInput>
          <div className="flex flex-row w-full gap-2">
            <Button
              onClick={async () => {
                const chars =
                  "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
                let key = "";
                for (let i = 0; i < 8; i++) {
                  key += chars.charAt(Math.floor(Math.random() * chars.length));
                }
                await updateSettingsWithToast(
                  {
                    tempory_share_token: key,
                    tempory_share_token_expire_at:
                      Math.floor(Date.now() / 1000) + shareHours * 3600,
                  },
                  t,
                );
                await refetch();
              }}
            >
              {t("common.generate")}
            </Button>
            <Button
              color="red"
              variant="soft"
              onClick={async () => {
                await updateSettingsWithToast(
                  { tempory_share_token: "", tempory_share_token_expire_at: 0 },
                  t,
                );
                await refetch();
              }}
            >
              {t("settings.site.tempory_share_revoke")}
            </Button>
          </div>
        </div>
      </SettingCardCollapse>
      <SettingCardLabel>{t("settings.site.custom")}</SettingCardLabel>
      <label className="text-sm text-muted-foreground -mt-4">
        {t("settings.custom.note")}
      </label>
      <SettingCardLongTextInput
        title={t("settings.custom.header")}
        description={t("settings.custom.header_description")}
        defaultValue={settings.custom_head || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ custom_head: data }, t);
        }}
      />
      <SettingCardLongTextInput
        title={t("settings.custom.body", "自定义 Body")}
        description={t(
          "settings.custom.body_description",
          "在页面底部添加自定义内容",
        )}
        defaultValue={settings.custom_body || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ custom_body: data }, t);
        }}
      />
      <SettingCardCollapse
        title={t("settings.custom.favicon", "自定义 Favicon")}
        description={t(
          "settings.custom.favicon_description",
          "在浏览器标签页显示的图标",
        )}
        defaultOpen={true}
      >
        <Flex
          width={"100%"}
          justify="between"
          align="start"
          direction={"column"}
          gap="2"
        >
          <Flex gap="2" align="center">
            {t("settings.custom.favicon_current", "当前 Favicon")}
            <img
              src={`/favicon.ico?v=${faviconRevision}`}
              alt="Favicon"
              style={{ width: 32, height: 32 }}
            />
          </Flex>
          <label className="text-sm text-muted-foreground">
            {t(
              "settings.custom.favicon_note",
              "Favicon 图标的更新速度可能较慢，通常需要清除浏览器缓存后才能看到更改。",
            )}
          </label>
          <Flex gap="2" align="center">
            <Dialog.Root>
              <Dialog.Trigger>
                <Button color="tomato">
                  {t("settings.custom.favicon_default", "恢复默认")}
                </Button>
              </Dialog.Trigger>
              <AppDialogContent>
                <Dialog.Title>
                  {t("settings.custom.favicon_default", "恢复默认")}
                </Dialog.Title>
                <Dialog.Description>
                  {t(
                    "settings.custom.favicon_default_description",
                    "这将恢复默认的 Favicon 图标，是否继续？",
                  )}
                </Dialog.Description>
                <Flex gap="2" justify="end">
                  <Dialog.Close>
                    <Button variant="soft">{t("common.cancel", "取消")}</Button>
                  </Dialog.Close>
                  <Dialog.Trigger>
                    <Button
                      color="red"
                      onClick={async () => {
                        fetch("/api/admin/update/favicon", {
                          method: "POST",
                        })
                          .then((response) => {
                            return response.json();
                          })
                          .then((data) => {
                            if (data.status === "success") {
                              refreshFavicon();
                              toast.success(t("settings.custom.favicon_default_success"));
                            } else {
                              toast.error(
                                data.message || t("settings.custom.favicon_default_error"),
                              );
                            }
                          })
                          .catch((error) => {
                            toast.error("" + error);
                          });
                      }}
                    >
                      {t("settings.custom.favicon_confirm")}
                    </Button>
                  </Dialog.Trigger>
                </Flex>
              </AppDialogContent>
            </Dialog.Root>
            <Button
              onClick={async () => {
                const input = document.createElement("input");
                input.type = "file";
                input.accept = "image/*";
                input.onchange = async (e) => {
                  const file = (e.target as HTMLInputElement).files?.[0];
                  if (file) {
                    try {
                      const response = await fetch(
                        "/api/admin/update/favicon",
                        {
                          method: "PUT",
                          body: file,
                          headers: {
                            "Content-Type": "application/octet-stream",
                          },
                        },
                      );
                      const data = await response.json();
                      if (data.status === "success") {
                        refreshFavicon();
                        toast.success(
                          t(
                            "settings.custom.favicon_update_success"
                          ),
                        );
                      } else {
                        toast.error(data.message || "Failed to update Favicon");
                      }
                    } catch (error) {
                      toast.error("" + error);
                    }
                  }
                };
                input.click();
              }}
            >
              {t("settings.custom.favicon_change")}
            </Button>
          </Flex>
        </Flex>
      </SettingCardCollapse>
      <SettingCardLabel>{t("settings.site.backup")}</SettingCardLabel>
      <SettingCardButton
        title={t("settings.site.backup_full_download")}
        description={t("settings.site.backup_full_download_description")}
        onClick={() => downloadBackup("full")}
      >
        {t("common.export")}
      </SettingCardButton>
      <SettingCardButton
        title={t("settings.site.backup_config_download")}
        description={t("settings.site.backup_config_download_description")}
        onClick={() => downloadBackup("config")}
      >
        {t("common.export")}
      </SettingCardButton>
      <SettingCardButton
        title={t("settings.site.backup_restore")}
        description={t("settings.site.backup_restore_description")}
        onClick={() => setRestoreOpen(true)}
      >
        {t("common.select")}
      </SettingCardButton>

      {/* 上传备份对话框 */}
      <UploadDialog
        open={restoreOpen}
        onOpenChange={(open) => {
          setRestoreOpen(open);
          if (!open && restoreState?.stage !== "completed") {
            setTrackedRestoreState(null);
          }
        }}
        title={t("settings.site.backup_restore")}
        description={t("settings.site.backup_restore_description")}
        accept=".zip"
        dragDropText={t("theme.drag_drop")}
        clickToBrowseText={t("theme.or_click_to_browse")}
        hintText={t("theme.zip_files_only")}
        uploadState={restoreState}
        cancelUploadLabel={t("common.cancel")}
        onCancelUpload={cancelRestore}
        onFileSelected={(file) => uploadBackup(file)}
        closeLabel={t("common.cancel")}
      />
    </>
  );
}

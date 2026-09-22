import { useEffect, useRef, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  Button,
  Callout,
  Card,
  Container,
  Flex,
  Heading,
  Progress,
  Text,
  TextArea,
  TextField,
} from "@radix-ui/themes";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  LoaderCircle,
  ShieldCheck,
  Upload,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import GuideHeader from "@/components/GuideHeader";
import UploadDialog from "@/components/UploadDialog";
import { uploadArchive } from "@/utils/archiveUpload";
import {
  UPLOAD_RESTARTING_VISIBLE_MS,
  createCompletedUploadState,
  createRestartingUploadState,
  delay,
  type UploadProgressState,
  withUploadProgressCopy,
} from "@/utils/uploadProgress";

type APIResponse<T> = {
  status: "success" | "error";
  message?: string;
  data?: T;
};
type InstallStatus = { state: string; required: boolean };
const INSTALL_REDIRECT_DELAY_MS = 2500;
const INSTALL_STEPS = ["welcome", "administrator", "site", "database", "confirm"];

function InstallLayout({
  step,
  children,
}: {
  step: number;
  children: ReactNode;
}) {
  const { t } = useTranslation();

  return (
    <main className="min-h-screen px-4 py-8 sm:px-6">
      <Container size="2">
        <div className="mb-5">
          <GuideHeader />
        </div>
        <Heading size="7" mb="5">
          {t("install.title")}
        </Heading>
        <Progress
          value={((step + 1) / INSTALL_STEPS.length) * 100}
          size="2"
          mb="4"
        />
        <Flex gap="3" mb="6" wrap="wrap">
          {INSTALL_STEPS.map((title, index) => (
            <Text
              key={title}
              size="2"
              weight={index === step ? "bold" : "regular"}
              color={index === step ? undefined : "gray"}
            >
              {index + 1}. {t(`install.steps.${title}`)}
            </Text>
          ))}
        </Flex>
        {children}
      </Container>
    </main>
  );
}

function isSQLiteDSN(dsn: string): boolean {
  const normalized = dsn.trim().toLowerCase();
  return (
    !normalized.startsWith("mysql://") &&
    !normalized.startsWith("postgres://") &&
    !normalized.startsWith("postgresql://") &&
    !normalized.includes("@tcp(") &&
    !normalized.includes("@unix(") &&
    !normalized.includes("dbname=")
  );
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/install${path}`, {
    ...init,
    headers: init?.body
      ? { "Content-Type": "application/json", ...init.headers }
      : init?.headers,
    cache: "no-store",
  });
  const contentType = response.headers.get("content-type") || "";
  if (!contentType.toLowerCase().includes("application/json"))
    throw new Error(`HTTP ${response.status}`);
  const payload = (await response.json()) as APIResponse<T>;
  if (!response.ok || payload.status !== "success")
    throw new Error(payload.message || `HTTP ${response.status}`);
  return payload.data as T;
}

export default function Install() {
  const { t } = useTranslation();
  const [step, setStep] = useState(0);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [passwordAgain, setPasswordAgain] = useState("");
  const [sitename, setSitename] = useState("Komari Lite");
  const [description, setDescription] = useState(
    "A simple server monitor tool.",
  );
  const [metricDSN, setMetricDSN] = useState("./data/metrics.db");
  const [busy, setBusy] = useState(false);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restoreState, setRestoreState] = useState<UploadProgressState | null>(
    null,
  );
  const restoreController = useRef<AbortController | null>(null);
  const restoreStateRef = useRef<UploadProgressState | null>(null);
  const [error, setError] = useState("");
  const [ready, setReady] = useState<boolean | null>(null);
  const [restartCountdown, setRestartCountdown] = useState<number | null>(null);
  const restoreCopy = {
    preparing: t("install.phase_preparing", "Preparing backup"),
    uploading: t("install.phase_uploading", "Uploading backup"),
    merging: t("install.phase_processing", "Restoring installation data"),
    processing: t("install.phase_processing", "Restoring installation data"),
    restarting: t("install.phase_restarting", "Restarting service"),
    completed: t("install.phase_completed", "Restore completed"),
    failed: t("install.restore_failed"),
    nonCancelable: t(
      "install.phase_non_cancelable",
      "Server processing has started and can no longer be canceled",
    ),
  };

  const setTrackedRestoreState = (state: UploadProgressState | null) => {
    restoreStateRef.current = state;
    setRestoreState(state);
  };

  useEffect(() => {
    void request<InstallStatus>("/status")
      .then((status) => setReady(status.required))
      .catch((reason) =>
        setError(
          reason instanceof Error
            ? reason.message
            : t("install.connection_error"),
        ),
      );
  }, [t]);

  useEffect(() => {
    if (ready !== false) return;
    const redirect = window.setTimeout(
      () => window.location.replace("/"),
      INSTALL_REDIRECT_DELAY_MS,
    );
    return () => window.clearTimeout(redirect);
  }, [ready]);

  const next = () => {
    setError("");
    if (step === 1 && !username.trim())
      return setError(t("install.username_required"));
    if (step === 1 && password.length < 8)
      return setError(t("account.password_too_short_error"));
    if (step === 1 && !/(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/.test(password))
      return setError(t("account.password_strength_error"));
    if (step === 1 && password !== passwordAgain)
      return setError(t("account.password_mismatch_error"));
    if (step === 2 && !sitename.trim())
      return setError(t("install.sitename_required"));
    if (step === 3 && !metricDSN.trim())
      return setError(t("install.dsn_required"));
    setStep((current) => Math.min(current + 1, 4));
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (step !== 4) return next();
    setBusy(true);
    setError("");
    try {
      await request("/complete", {
        method: "POST",
        body: JSON.stringify({
          username,
          password,
          sitename,
          description,
          metric_dsn: metricDSN,
        }),
      });
      setReady(false);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("install.failed"));
      setBusy(false);
    }
  };

  const restoreBackup = async (file: File) => {
    if (!file.name.toLowerCase().endsWith(".zip")) {
      setError(t("install.restore_file_type"));
      return;
    }

    setError("");
    setRestartCountdown(null);
    setTrackedRestoreState(null);
    const controller = new AbortController();
    restoreController.current = controller;
    try {
      await uploadArchive({
        basePath: "/api/install/upload",
        purpose: "backup",
        file,
        signal: controller.signal,
        onStateChange: (state) => {
          const nextState =
            state.stage === "merging"
              ? withUploadProgressCopy(
                  {
                    ...state,
                    stage: "processing",
                    indeterminate: true,
                    canCancel: false,
                    percent: null,
                  },
                  restoreCopy,
                )
              : state;
          setTrackedRestoreState(
            "stage" in nextState
              ? withUploadProgressCopy(nextState, restoreCopy)
              : nextState,
          );
        },
      });
      setTrackedRestoreState(
        withUploadProgressCopy(
          createRestartingUploadState(restoreStateRef.current),
          restoreCopy,
        ),
      );
      await delay(UPLOAD_RESTARTING_VISIBLE_MS);
      setTrackedRestoreState(
        withUploadProgressCopy(
          createCompletedUploadState(restoreStateRef.current, {
            detail: t("install.phase_redirecting", "Opening the dashboard shortly"),
          }),
          restoreCopy,
        ),
      );
      restoreController.current = null;
      setRestartCountdown(5);
      window.setTimeout(() => window.location.assign("/"), 5000);
    } catch (reason) {
      if (!(reason instanceof DOMException && reason.name === "AbortError")) {
        setError(
          reason instanceof Error
            ? reason.message
            : t("install.restore_failed"),
        );
      }
      setTrackedRestoreState(null);
      setRestartCountdown(null);
      restoreController.current = null;
    }
  };

  const cancelRestore = () => {
    restoreController.current?.abort();
    setTrackedRestoreState(null);
    setRestartCountdown(null);
  };

  useEffect(() => {
    if (restartCountdown === null || restartCountdown <= 0) return;
    const timer = window.setTimeout(() => {
      setRestartCountdown((current) =>
        current === null ? current : Math.max(0, current - 1),
      );
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [restartCountdown]);

  if (ready === null)
    return (
      <main className="flex min-h-screen items-center justify-center p-6">
        <Flex direction="column" align="center" gap="4">
          <LoaderCircle
            size={28}
            className={error ? undefined : "animate-spin"}
          />
          <Text color={error ? "red" : "gray"}>
            {error || t("loading")}
          </Text>
        </Flex>
      </main>
    );

  if (ready === false)
    return (
      <InstallLayout step={INSTALL_STEPS.length - 1}>
        <Card size="3">
          <Flex
            direction="column"
            align="center"
            gap="5"
            className="py-10 text-center sm:py-14"
            aria-live="polite"
          >
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-[var(--green-a3)] text-[var(--green-11)]">
              <Check size={34} strokeWidth={2} />
            </div>
            <Flex direction="column" align="center" gap="2">
              <Heading size="7">{t("install.completed_title")}</Heading>
              <Text size="3" color="gray">
                {t("install.completed")}
              </Text>
            </Flex>
          </Flex>
        </Card>
      </InstallLayout>
    );

  return (
    <InstallLayout step={step}>
        {error && (
          <Callout.Root color="red" variant="surface" mb="4">
            <Callout.Text>{error}</Callout.Text>
          </Callout.Root>
        )}
        <form onSubmit={submit}>
          <Card size="3">
            {step === 0 && (
              <Flex direction="column" gap="5">
                <Flex align="center" gap="3">
                  <div>
                    <Heading size="6">{t("install.welcome_title")}</Heading>
                    <Text size="2" color="gray">
                      {t("install.welcome_subtitle")}
                    </Text>
                  </div>
                </Flex>
                <Text size="3">{t("install.welcome_description")}</Text>
              </Flex>
            )}
            {step === 1 && (
              <Flex direction="column" gap="4">
                <Flex align="center" gap="3">
                  <ShieldCheck size={24} />
                  <Heading size="5">{t("install.admin_title")}</Heading>
                </Flex>
                <label>
                  <Text as="div" size="2" weight="bold" mb="1">
                    {t("install.username")}
                  </Text>
                  <TextField.Root
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    autoComplete="username"
                    autoFocus
                  />
                </label>
                <label>
                  <Text as="div" size="2" weight="bold" mb="1">
                    {t("install.password")}
                  </Text>
                  <TextField.Root
                    type="password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    autoComplete="new-password"
                  />
                </label>
                <label>
                  <Text as="div" size="2" weight="bold" mb="1">
                    {t("install.password_confirm")}
                  </Text>
                  <TextField.Root
                    type="password"
                    value={passwordAgain}
                    onChange={(event) => setPasswordAgain(event.target.value)}
                    autoComplete="new-password"
                  />
                </label>
              </Flex>
            )}
            {step === 2 && (
              <Flex direction="column" gap="4">
                <div>
                  <Heading size="5">{t("install.site_title")}</Heading>
                  <Text size="2" color="gray">
                    {t("install.site_hint")}
                  </Text>
                </div>
                <label>
                  <Text as="div" size="2" weight="bold" mb="1">
                    {t("install.sitename")}
                  </Text>
                  <TextField.Root
                    value={sitename}
                    onChange={(event) => setSitename(event.target.value)}
                    autoFocus
                  />
                </label>
                <label>
                  <Text as="div" size="2" weight="bold" mb="1">
                    {t("install.description")}
                  </Text>
                  <TextArea
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                    rows={4}
                  />
                </label>
              </Flex>
            )}
            {step === 3 && (
              <Flex direction="column" gap="4">
                <Flex align="center" gap="3">
                  <div>
                    <Heading size="5">{t("install.database_title")}</Heading>
                    <Text size="2" color="gray">
                      {t("settings.metrics.dsn_description")}
                    </Text>
                  </div>
                </Flex>
                <label>
                  <Text as="div" size="2" weight="bold" mb="1">
                    {t("settings.metrics.dsn_title")}
                  </Text>
                  <TextField.Root
                    value={metricDSN}
                    onChange={(event) => setMetricDSN(event.target.value)}
                    autoFocus
                  />
                </label>
                {isSQLiteDSN(metricDSN) ? (
                  <Callout.Root color="amber" variant="surface">
                    <Callout.Text>{t("install.sqlite_warning")}</Callout.Text>
                  </Callout.Root>
                ) : null}
              </Flex>
            )}
            {step === 4 && (
              <Flex direction="column" gap="4">
                <Heading size="5">{t("install.confirm_title")}</Heading>
                <Card variant="surface">
                  <Flex direction="column" gap="2">
                    <Text size="2">
                      <strong>{t("install.summary_admin")}</strong>
                      {username}
                    </Text>
                    <Text size="2">
                      <strong>{t("install.summary_sitename")}</strong>
                      {sitename}
                    </Text>

                    <Text size="2" className="break-all">
                      <strong>{t("install.summary_dsn")}</strong>
                      {metricDSN}
                    </Text>
                  </Flex>
                </Card>
              </Flex>
            )}
          </Card>
          <Flex justify="between" mt="5">
            <Button
              type="button"
              variant="soft"
              color="gray"
              disabled={step === 0 || busy || Boolean(restoreState)}
              onClick={() => {
                setError("");
                setStep((current) => current - 1);
              }}
            >
              <ArrowLeft size={16} />
              {t("install.back")}
            </Button>
            {step < 4 ? (
              <Flex gap="3">
                {step === 0 && (
                  <Button
                    type="button"
                    variant="soft"
                    disabled={busy || Boolean(restoreState) || ready === null}
                    onClick={() => setRestoreOpen(true)}
                  >
                    {restoreState ? (
                      <LoaderCircle size={16} className="animate-spin" />
                    ) : (
                      <Upload size={16} />
                    )}
                    {t(
                      restoreState
                        ? "install.restore_restarting"
                        : "install.restore",
                    )}
                  </Button>
                )}
                <Button type="button" disabled={Boolean(restoreState)} onClick={next}>
                  {t(step === 0 ? "install.start" : "install.next")}
                  <ArrowRight size={16} />
                </Button>
              </Flex>
            ) : (
              <Button type="submit" disabled={busy || ready === null}>
                {busy ? (
                  <LoaderCircle size={16} className="animate-spin" />
                ) : (
                  <Check size={16} />
                )}
                {t("install.complete")}
              </Button>
            )}
          </Flex>
        </form>
        <UploadDialog
          open={restoreOpen}
          onOpenChange={(open) => {
            setRestoreOpen(open);
            if (!open && restoreState?.stage !== "completed") {
              setTrackedRestoreState(null);
              setRestartCountdown(null);
            }
          }}
          title={t("install.restore")}
          description={error || t("install.restore_description")}
          accept=".zip"
          dragDropText={t("theme.drag_drop")}
          clickToBrowseText={t("theme.or_click_to_browse")}
          hintText={t("theme.zip_files_only")}
          uploadState={
            restartCountdown !== null && restoreState?.stage === "completed"
              ? {
                  ...restoreState,
                  detail: t("install.phase_redirect_countdown", {
                    defaultValue: "Opening the dashboard in {{count}}s",
                    count: restartCountdown,
                  }),
                }
              : restoreState
          }
          cancelUploadLabel={t("common.cancel")}
          onCancelUpload={cancelRestore}
          onFileSelected={restoreBackup}
          closeLabel={t("common.cancel")}
        />
    </InstallLayout>
  );
}

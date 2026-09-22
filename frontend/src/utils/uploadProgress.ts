export type UploadProgressStage =
  | "preparing"
  | "uploading"
  | "merging"
  | "processing"
  | "restarting"
  | "completed"
  | "failed";

export type UploadProgressState = {
  stage: UploadProgressStage;
  percent: number | null;
  uploadedBytes: number;
  totalBytes: number;
  uploadedChunks: number;
  totalChunks: number;
  canCancel: boolean;
  indeterminate: boolean;
  label?: string;
  detail?: string;
  actionLabel?: string;
  errorMessage?: string;
};

export const UPLOAD_COMPLETED_VISIBLE_MS = 900;
export const UPLOAD_RESTARTING_VISIBLE_MS = 1100;
export const UPLOAD_FINAL_PROGRESS_VISIBLE_MS = 240;
export const UPLOAD_DIALOG_EXIT_MS = 240;

type UploadProgressSnapshot = Pick<
  UploadProgressState,
  "percent" | "uploadedBytes" | "totalBytes" | "uploadedChunks" | "totalChunks"
>;

const clampPercent = (value: number | null | undefined) => {
  if (typeof value !== "number" || !Number.isFinite(value)) return null;
  return Math.max(0, Math.min(100, Math.round(value)));
};

const snapshotProgress = (
  snapshot?: UploadProgressSnapshot | UploadProgressState | null,
): UploadProgressSnapshot => ({
  percent: clampPercent(snapshot?.percent),
  uploadedBytes: Math.max(0, snapshot?.uploadedBytes ?? 0),
  totalBytes: Math.max(0, snapshot?.totalBytes ?? 0),
  uploadedChunks: Math.max(0, snapshot?.uploadedChunks ?? 0),
  totalChunks: Math.max(0, snapshot?.totalChunks ?? 0),
});

function createStageState(
  stage: UploadProgressStage,
  snapshot?: UploadProgressSnapshot | UploadProgressState | null,
  overrides: Partial<UploadProgressState> = {},
): UploadProgressState {
  const base = snapshotProgress(snapshot);
  const isDeterminate = stage === "uploading" || stage === "completed";
  const percent =
    stage === "completed"
      ? 100
      : isDeterminate
        ? base.percent ?? 0
        : null;

  return {
    stage,
    percent,
    uploadedBytes:
      overrides.uploadedBytes ??
      (stage === "completed"
        ? Math.max(base.totalBytes, base.uploadedBytes)
        : base.uploadedBytes),
    totalBytes: overrides.totalBytes ?? base.totalBytes,
    uploadedChunks:
      overrides.uploadedChunks ??
      (stage === "completed"
        ? Math.max(base.totalChunks, base.uploadedChunks)
        : base.uploadedChunks),
    totalChunks: overrides.totalChunks ?? base.totalChunks,
    canCancel: stage === "preparing" || stage === "uploading",
    indeterminate:
      stage === "merging" || stage === "processing" || stage === "restarting",
    ...overrides,
  };
}

export function createPreparingUploadState(totalBytes: number) {
  return createStageState("preparing", {
    percent: 0,
    totalBytes,
    uploadedBytes: 0,
    uploadedChunks: 0,
    totalChunks: 0,
  });
}

export function createUploadingUploadState({
  totalBytes,
  uploadedBytes,
  totalChunks,
  uploadedChunks,
}: {
  totalBytes: number;
  uploadedBytes: number;
  totalChunks: number;
  uploadedChunks: number;
}) {
  const percent =
    totalBytes > 0 ? (uploadedBytes / totalBytes) * 100 : uploadedBytes > 0 ? 100 : 0;
  return createStageState(
    "uploading",
    { percent, totalBytes, uploadedBytes, totalChunks, uploadedChunks },
    { percent },
  );
}

export function createMergingUploadState(
  snapshot?: UploadProgressSnapshot | UploadProgressState | null,
) {
  const base = snapshotProgress(snapshot);
  return createStageState(
    "merging",
    {
      ...base,
      uploadedBytes: Math.max(base.totalBytes, base.uploadedBytes),
      uploadedChunks: Math.max(base.totalChunks, base.uploadedChunks),
    },
  );
}

export function createProcessingUploadState(
  snapshot?: UploadProgressSnapshot | UploadProgressState | null,
  overrides: Partial<UploadProgressState> = {},
) {
  return createStageState("processing", snapshot, overrides);
}

export function createRestartingUploadState(
  snapshot?: UploadProgressSnapshot | UploadProgressState | null,
  overrides: Partial<UploadProgressState> = {},
) {
  return createStageState("restarting", snapshot, overrides);
}

export function createCompletedUploadState(
  snapshot?: UploadProgressSnapshot | UploadProgressState | null,
  overrides: Partial<UploadProgressState> = {},
) {
  return createStageState("completed", snapshot, overrides);
}

export function createFailedUploadState(
  message: string,
  snapshot?: UploadProgressSnapshot | UploadProgressState | null,
  overrides: Partial<UploadProgressState> = {},
) {
  return createStageState("failed", snapshot, {
    errorMessage: message,
    ...overrides,
  });
}

export function isUploadStageActive(stage: UploadProgressStage) {
  return stage !== "failed";
}

export function formatUploadBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const digits = value >= 100 || unitIndex === 0 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(digits)} ${units[unitIndex]}`;
}

export function delay(ms: number) {
  return new Promise<void>((resolve) => {
    globalThis.setTimeout(resolve, ms);
  });
}

export type UploadProgressCopy = {
  preparing: string;
  uploading: string;
  merging: string;
  processing: string;
  restarting: string;
  completed: string;
  failed?: string;
  nonCancelable?: string;
};

export function withUploadProgressCopy(
  state: UploadProgressState,
  copy: UploadProgressCopy,
  overrides: Partial<UploadProgressState> = {},
): UploadProgressState {
  const actionLabel =
    state.stage === "merging" ||
    state.stage === "processing" ||
    state.stage === "restarting"
      ? copy.nonCancelable
      : undefined;
  const label =
    state.stage === "preparing"
      ? copy.preparing
      : state.stage === "uploading"
        ? copy.uploading
        : state.stage === "merging"
          ? copy.merging
          : state.stage === "processing"
            ? copy.processing
            : state.stage === "restarting"
              ? copy.restarting
              : state.stage === "completed"
                ? copy.completed
                : copy.failed ?? state.errorMessage ?? copy.processing;
  return {
    ...state,
    label,
    actionLabel,
    ...overrides,
  };
}

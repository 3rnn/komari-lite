import React, { useMemo, useRef } from "react";
import { Box, Button, Card, Flex, Text, Spinner, Dialog } from "@radix-ui/themes";
import { CheckCircle2, LoaderCircle, Upload as UploadIcon, XCircle } from "lucide-react";
import AppDialogContent from "@/components/AppDialogContent";
import {
  createUploadingUploadState,
  formatUploadBytes,
  isUploadStageActive,
  type UploadProgressState,
} from "@/utils/uploadProgress";

export type UploadDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: React.ReactNode;
  description?: React.ReactNode;
  visuallyHiddenDescription?: React.ReactNode;
  accept?: string;
  dragDropText?: React.ReactNode;
  clickToBrowseText?: React.ReactNode;
  hintText?: React.ReactNode;
  uploadState?: UploadProgressState | null;
  uploading?: boolean;
  progress?: number;
  uploadingText?: React.ReactNode;
  cancelUploadLabel?: React.ReactNode;
  onCancelUpload?: () => void;
  onFileSelected?: (file: File) => void;
  closeLabel?: React.ReactNode;
};

function matchesAccept(file: File, accept: string | undefined) {
  if (!accept || accept.trim() === "" || accept === "*/*") return true;
  const items = accept.split(",").map((s) => s.trim().toLowerCase());
  const name = file.name.toLowerCase();
  const type = (file.type || "").toLowerCase();
  for (const item of items) {
    if (item.startsWith(".")) {
      if (name.endsWith(item)) return true;
    } else if (item.includes("/")) {
      if (type === item) return true;
      const [major] = item.split("/");
      const [fileMajor] = type.split("/");
      if (major && major === fileMajor) return true;
    }
  }
  return false;
}

function formatUploadDetail(state: UploadProgressState) {
  if (state.stage === "uploading") {
    const bytes =
      state.totalBytes > 0
        ? `${formatUploadBytes(state.uploadedBytes)} / ${formatUploadBytes(state.totalBytes)}`
        : formatUploadBytes(state.uploadedBytes);
    const chunks =
      state.totalChunks > 0
        ? ` · ${state.uploadedChunks}/${state.totalChunks}`
        : "";
    return `${bytes}${chunks}`;
  }
  if (state.errorMessage) return state.errorMessage;
  if (state.detail) return state.detail;
  if (state.indeterminate) return state.actionLabel;
  return undefined;
}

function UploadStageIcon({ state }: { state: UploadProgressState }) {
  if (state.stage === "completed") {
    return <CheckCircle2 size={20} className="text-green-600" />;
  }
  if (state.stage === "failed") {
    return <XCircle size={20} className="text-red-600" />;
  }
  if (state.indeterminate) {
    return <Spinner size="2" />;
  }
  return <LoaderCircle size={20} className="animate-spin text-blue-600" />;
}

const UploadDialog: React.FC<UploadDialogProps> = ({
  open,
  onOpenChange,
  title,
  description,
  visuallyHiddenDescription,
  accept = "*/*",
  dragDropText,
  clickToBrowseText,
  hintText,
  uploadState,
  uploading = false,
  progress = 0,
  uploadingText,
  cancelUploadLabel,
  onCancelUpload,
  onFileSelected,
  closeLabel = "Close",
}) => {
  const inputRef = useRef<HTMLInputElement>(null);

  const normalizedState = useMemo<UploadProgressState | null>(() => {
    if (uploadState) return uploadState;
    if (!uploading) return null;
    const derived = createUploadingUploadState({
      totalBytes: 100,
      uploadedBytes: Math.max(0, Math.min(100, progress)),
      totalChunks: 0,
      uploadedChunks: 0,
    });
    return {
      ...derived,
      label: typeof uploadingText === "string" ? uploadingText : undefined,
      detail: `${Math.round(Math.max(0, Math.min(100, progress)))}%`,
    };
  }, [progress, uploadState, uploading, uploadingText]);

  const uploadActive = normalizedState ? isUploadStageActive(normalizedState.stage) : false;
  const canCancel = normalizedState?.canCancel ?? false;

  const handleDrop = (e: React.DragEvent) => {
    if (uploadActive) return;
    e.preventDefault();
    const files = Array.from(e.dataTransfer.files);
    const file = files.find((f) => matchesAccept(f, accept));
    if (file && onFileSelected) onFileSelected(file);
  };

  const handleDragOver = (e: React.DragEvent) => {
    if (uploadActive) return;
    e.preventDefault();
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file && matchesAccept(file, accept) && onFileSelected) {
      onFileSelected(file);
    }
    e.target.value = "";
  };

  const detail = normalizedState ? formatUploadDetail(normalizedState) : undefined;
  const showUploadPercent = normalizedState?.stage === "uploading";
  const progressValue =
    normalizedState?.stage === "completed"
      ? 100
      : normalizedState?.indeterminate
        ? undefined
        : Math.max(0, Math.min(100, normalizedState?.percent ?? 0));

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && uploadActive) return;
        onOpenChange(nextOpen);
      }}
    >
      <AppDialogContent
        maxWidth="450px"
        title={title}
        description={description}
        visuallyHiddenDescription={visuallyHiddenDescription}
      >
        <Box className="space-y-4 mt-4">
          <Flex
            direction="column"
            align="center"
            justify="center"
            className={
              uploadActive
                ? "border-2 border-dashed border-gray-200 rounded-lg p-8 text-center opacity-60 cursor-not-allowed"
                : "border-2 border-dashed border-gray-300 rounded-lg p-8 text-center cursor-pointer hover:border-gray-400 transition-colors"
            }
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onClick={() => {
              if (!uploadActive) inputRef.current?.click();
            }}
          >
            <UploadIcon size={48} className="mx-auto text-gray-400 mb-4" />
            {dragDropText ? (
              <Text size="3" weight="medium">{dragDropText}</Text>
            ) : null}
            {clickToBrowseText ? (
              <Text size="2" color="gray" className="mt-2">
                {clickToBrowseText}
              </Text>
            ) : null}
            {hintText ? (
              <Text size="1" color="gray" className="mt-2">
                {hintText}
              </Text>
            ) : null}
          </Flex>

          <input
            ref={inputRef}
            type="file"
            accept={accept}
            onChange={handleFileSelect}
            className="hidden"
            disabled={uploadActive}
          />

          {normalizedState ? (
            <Card className="km-upload-status-card p-5">
              <Flex direction="column" gap="3">
                <Flex align="center" gap="2" className="km-upload-status-heading">
                  <UploadStageIcon state={normalizedState} />
                  <Text size="3" weight="medium">
                    {normalizedState.label ?? uploadingText ?? title}
                  </Text>
                </Flex>

                <Box
                  className={
                    normalizedState.indeterminate
                      ? "relative w-full bg-gray-200 dark:bg-gray-700 rounded-full h-3 overflow-hidden"
                      : "w-full bg-gray-200 dark:bg-gray-700 rounded-full h-3 overflow-hidden"
                  }
                >
                  <Box
                    className={
                      normalizedState.indeterminate
                        ? "absolute inset-y-0 left-0 w-2/5 rounded-full bg-gradient-to-r from-blue-500 via-blue-400 to-blue-500 km-upload-indeterminate-bar"
                        : normalizedState.stage === "completed"
                          ? "h-full rounded-full bg-gradient-to-r from-green-500 to-green-600 transition-[width] duration-150 ease-out"
                          : normalizedState.stage === "failed"
                            ? "h-full rounded-full bg-gradient-to-r from-red-500 to-red-600 transition-[width] duration-150 ease-out"
                            : "h-full rounded-full bg-gradient-to-r from-blue-500 to-blue-600 transition-[width] duration-150 ease-out"
                    }
                    style={
                      typeof progressValue === "number"
                        ? { width: `${progressValue}%` }
                        : undefined
                    }
                  />
                </Box>

                <Flex
                  justify="between"
                  align="center"
                  gap="3"
                  className="km-upload-status-detail"
                >
                  <Text size="2" color="gray">
                    {detail ?? ""}
                  </Text>
                  {showUploadPercent && typeof normalizedState.percent === "number" ? (
                    <Text size="2" color="gray">
                      {Math.round(normalizedState.percent)}%
                    </Text>
                  ) : null}
                </Flex>
              </Flex>
            </Card>
          ) : null}
        </Box>

        <Flex gap="3" mt="4" justify="end" className="km-upload-dialog-actions">
          {normalizedState && canCancel && onCancelUpload ? (
            <Button
              variant="soft"
              color="gray"
              onClick={() => {
                onCancelUpload();
                onOpenChange(false);
              }}
            >
              {cancelUploadLabel ?? "Cancel"}
            </Button>
          ) : normalizedState &&
            uploadActive &&
            normalizedState.stage !== "completed" ? (
            <Button variant="soft" color="gray" disabled>
              {cancelUploadLabel ?? closeLabel}
            </Button>
          ) : normalizedState?.stage === "completed" ? (
            null
          ) : (
            <Dialog.Close>
              <Button variant="soft" color="gray">
                {closeLabel}
              </Button>
            </Dialog.Close>
          )}
        </Flex>
      </AppDialogContent>
    </Dialog.Root>
  );
};

export default UploadDialog;

import {
  UPLOAD_FINAL_PROGRESS_VISIBLE_MS,
  createFailedUploadState,
  createMergingUploadState,
  createPreparingUploadState,
  createUploadingUploadState,
  delay,
  type UploadProgressState,
} from "./uploadProgress.ts";

export type ArchiveUploadPurpose = "backup" | "theme";

type APIResponse<T> = {
  status: "success" | "error";
  message?: string;
  data?: T;
};

type UploadInit = {
  upload_id: string;
  chunk_size: number;
  chunks: number;
};

export type ArchiveUploadOptions = {
  basePath: string;
  purpose: ArchiveUploadPurpose;
  file: File;
  signal?: AbortSignal;
  onProgress?: (progress: number) => void;
  onStateChange?: (state: UploadProgressState) => void;
  maxChunkAttempts?: number;
};

export class ArchiveUploadError extends Error {
  status: number;

  constructor(message: string, status = 0) {
    super(message);
    this.name = "ArchiveUploadError";
    this.status = status;
  }
}

async function parseResponse<T>(response: Response): Promise<APIResponse<T>> {
  const contentType = response.headers.get("content-type") || "";
  let payload: APIResponse<T> = {
    status: response.ok ? "success" : "error",
  };
  if (contentType.toLowerCase().includes("application/json")) {
    payload = (await response.json()) as APIResponse<T>;
  }
  if (!response.ok || payload.status !== "success") {
    throw new ArchiveUploadError(
      payload.message || `HTTP ${response.status}`,
      response.status,
    );
  }
  return payload;
}

async function requestJSON<T>(
  path: string,
  body: unknown,
  signal?: AbortSignal,
): Promise<APIResponse<T>> {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    cache: "no-store",
    signal,
  });
  return parseResponse<T>(response);
}

function isRetryable(reason: unknown): boolean {
  if (reason instanceof DOMException && reason.name === "AbortError") {
    return false;
  }
  if (reason instanceof ArchiveUploadError) {
    return (
      reason.status === 408 ||
      reason.status === 429 ||
      reason.status >= 500
    );
  }
  return true;
}

function emitUploadState(
  state: UploadProgressState,
  onStateChange?: (state: UploadProgressState) => void,
  onProgress?: (progress: number) => void,
) {
  onStateChange?.(state);
  if (typeof state.percent === "number") {
    onProgress?.(state.percent);
  }
}

function isAbortReason(reason: unknown) {
  return reason instanceof DOMException && reason.name === "AbortError";
}

function describeUploadFailure(reason: unknown) {
  return reason instanceof Error ? reason.message : "Upload failed";
}

function waitForRetry(delay: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const cleanup = () => signal?.removeEventListener("abort", abort);
    const timer = globalThis.setTimeout(() => {
      cleanup();
      resolve();
    }, delay);
    const abort = () => {
      globalThis.clearTimeout(timer);
      cleanup();
      reject(new DOMException("Upload cancelled", "AbortError"));
    };
    if (signal?.aborted) {
      abort();
      return;
    }
    signal?.addEventListener("abort", abort, { once: true });
  });
}

async function uploadChunk(
  path: string,
  uploadID: string,
  chunkIndex: number,
  chunk: Blob,
  signal: AbortSignal | undefined,
  maxAttempts: number,
  onProgress?: (loadedBytes: number) => void,
): Promise<void> {
  let lastError: unknown;
  let maxReportedLoaded = 0;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      await uploadChunkAttempt(
        path,
        uploadID,
        chunkIndex,
        chunk,
        signal,
        (loadedBytes) => {
          maxReportedLoaded = Math.max(maxReportedLoaded, loadedBytes);
          onProgress?.(maxReportedLoaded);
        },
      );
      return;
    } catch (reason) {
      lastError = reason;
      if (attempt === maxAttempts || !isRetryable(reason)) throw reason;
      await waitForRetry(250 * 2 ** (attempt - 1), signal);
    }
  }
  throw lastError;
}

function createChunkBody(uploadID: string, chunkIndex: number, chunk: Blob) {
  const body = new FormData();
  body.append("upload_id", uploadID);
  body.append("chunk_index", String(chunkIndex));
  body.append("chunk_data", chunk, "chunk.part");
  return body;
}

async function uploadChunkAttempt(
  path: string,
  uploadID: string,
  chunkIndex: number,
  chunk: Blob,
  signal: AbortSignal | undefined,
  onProgress: (loadedBytes: number) => void,
) {
  const body = createChunkBody(uploadID, chunkIndex, chunk);
  if (typeof XMLHttpRequest === "undefined") {
    const response = await fetch(path, {
      method: "POST",
      body,
      cache: "no-store",
      signal,
    });
    await parseResponse(response);
    return;
  }

  await new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    let settled = false;
    const cleanup = () => signal?.removeEventListener("abort", abort);
    const finish = (callback: () => void) => {
      if (settled) return;
      settled = true;
      cleanup();
      callback();
    };
    const abort = () => {
      xhr.abort();
      finish(() => reject(new DOMException("Upload cancelled", "AbortError")));
    };

    xhr.open("POST", path);
    xhr.upload.onprogress = (event) => {
      onProgress(Math.min(chunk.size, Math.max(0, event.loaded)));
    };
    xhr.onload = () => {
      const ok = xhr.status >= 200 && xhr.status < 300;
      let payload: APIResponse<unknown> = { status: ok ? "success" : "error" };
      if (xhr.responseText) {
        try {
          payload = JSON.parse(xhr.responseText) as APIResponse<unknown>;
        } catch {
          // Non-JSON success responses retain the status-derived fallback.
        }
      }
      if (!ok || payload.status !== "success") {
        finish(() => reject(new ArchiveUploadError(
          payload.message || xhr.statusText || `HTTP ${xhr.status}`,
          xhr.status,
        )));
        return;
      }
      finish(resolve);
    };
    xhr.onerror = () => finish(() => reject(new ArchiveUploadError("Network error")));
    xhr.onabort = () => finish(() => reject(new DOMException("Upload cancelled", "AbortError")));

    if (signal?.aborted) {
      abort();
      return;
    }
    signal?.addEventListener("abort", abort, { once: true });
    xhr.send(body);
  });
}

async function cancelUpload(basePath: string, uploadID: string): Promise<void> {
  try {
    await requestJSON(`${basePath}/cancel`, { upload_id: uploadID });
  } catch {
    // The server also expires abandoned sessions and clears them on restart.
  }
}

export async function uploadArchive({
  basePath,
  purpose,
  file,
  signal,
  onProgress,
  onStateChange,
  maxChunkAttempts = 3,
}: ArchiveUploadOptions): Promise<APIResponse<unknown>> {
  if (!file.name.toLowerCase().endsWith(".zip")) {
    throw new ArchiveUploadError("Only ZIP archives are supported");
  }
  if (file.size <= 0) {
    throw new ArchiveUploadError("The archive is empty");
  }

  let uploadID = "";
  let currentState = createPreparingUploadState(file.size);
  emitUploadState(currentState, onStateChange, onProgress);
  try {
    const init = await requestJSON<UploadInit>(
      `${basePath}/init`,
      { purpose, filename: file.name, size: file.size },
      signal,
    );
    uploadID = init.data?.upload_id || "";
    const chunkSize = init.data?.chunk_size || 0;
    const chunkCount = init.data?.chunks || 0;
    if (!uploadID || chunkSize <= 0 || chunkCount <= 0) {
      throw new ArchiveUploadError("The server returned invalid upload metadata");
    }

    currentState = createUploadingUploadState({
      totalBytes: file.size,
      uploadedBytes: 0,
      totalChunks: chunkCount,
      uploadedChunks: 0,
    });
    emitUploadState(currentState, onStateChange, onProgress);
    for (let index = 0; index < chunkCount; index += 1) {
      const start = index * chunkSize;
      const end = Math.min(file.size, start + chunkSize);
      await uploadChunk(
        `${basePath}/chunk`,
        uploadID,
        index,
        file.slice(start, end),
        signal,
        Math.max(1, maxChunkAttempts),
        (loadedBytes) => {
          currentState = createUploadingUploadState({
            totalBytes: file.size,
            uploadedBytes: Math.min(end, start + loadedBytes),
            totalChunks: chunkCount,
            uploadedChunks: index,
          });
          emitUploadState(currentState, onStateChange, onProgress);
        },
      );
      currentState = createUploadingUploadState({
        totalBytes: file.size,
        uploadedBytes: end,
        totalChunks: chunkCount,
        uploadedChunks: index + 1,
      });
      emitUploadState(currentState, onStateChange, onProgress);
    }

    if (onStateChange || onProgress) {
      await delay(UPLOAD_FINAL_PROGRESS_VISIBLE_MS);
    }

    currentState = createMergingUploadState(currentState);
    emitUploadState(currentState, onStateChange, onProgress);

    return await requestJSON<unknown>(
      `${basePath}/merge`,
      { upload_id: uploadID },
      signal,
    );
  } catch (reason) {
    if (!isAbortReason(reason)) {
      emitUploadState(
        createFailedUploadState(describeUploadFailure(reason), currentState),
        onStateChange,
        onProgress,
      );
    }
    if (uploadID) await cancelUpload(basePath, uploadID);
    throw reason;
  }
}

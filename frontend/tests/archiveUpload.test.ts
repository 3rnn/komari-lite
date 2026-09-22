import assert from "node:assert/strict";
import test from "node:test";
import { uploadArchive } from "../src/utils/archiveUpload.ts";
import {
  createCompletedUploadState,
  withUploadProgressCopy,
} from "../src/utils/uploadProgress.ts";

type FetchCall = { url: string; init?: RequestInit };

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

test("uploads backup archives in server-sized chunks and merges last", async () => {
  const originalFetch = globalThis.fetch;
  const calls: FetchCall[] = [];
  const progress: number[] = [];
  const stages: string[] = [];
  globalThis.fetch = async (input, init) => {
    calls.push({ url: String(input), init });
    if (String(input).endsWith("/init")) {
      return jsonResponse({
        status: "success",
        data: { upload_id: "upload-1", chunk_size: 4, chunks: 3 },
      });
    }
    if (String(input).endsWith("/merge")) {
      return jsonResponse({ status: "success", data: {} });
    }
    return jsonResponse({ status: "success", data: { received: true } });
  };

  try {
    const file = new File(["abcdefghij"], "backup.zip");
    await uploadArchive({
      basePath: "/api/admin/upload",
      purpose: "backup",
      file,
      onProgress: (value) => progress.push(value),
      onStateChange: (state) => stages.push(state.stage),
    });

    assert.deepEqual(
      calls.map((call) => call.url),
      [
        "/api/admin/upload/init",
        "/api/admin/upload/chunk",
        "/api/admin/upload/chunk",
        "/api/admin/upload/chunk",
        "/api/admin/upload/merge",
      ],
    );
    const chunkForms = calls.slice(1, 4).map((call) => call.init?.body as FormData);
    assert.deepEqual(chunkForms.map((form) => form.get("upload_id")), ["upload-1", "upload-1", "upload-1"]);
    assert.deepEqual(chunkForms.map((form) => form.get("chunk_index")), ["0", "1", "2"]);
    const chunks = chunkForms.map((form) => form.get("chunk_data") as File);
    assert.deepEqual(chunks.map((chunk) => chunk.size), [4, 4, 2]);
    assert.equal(calls[0].init?.body, JSON.stringify({
      purpose: "backup",
      filename: "backup.zip",
      size: 10,
    }));
    assert.deepEqual(progress, [0, 40, 80, 100]);
    assert.deepEqual(stages, ["preparing", "uploading", "uploading", "uploading", "uploading", "merging"]);
    assert.equal(calls.at(-1)?.init?.body, JSON.stringify({ upload_id: "upload-1" }));
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("reports in-flight browser upload bytes before merge processing begins", async () => {
  const originalFetch = globalThis.fetch;
  const hadXMLHttpRequest = "XMLHttpRequest" in globalThis;
  const originalXMLHttpRequest = globalThis.XMLHttpRequest;
  const progress: number[] = [];
  const stages: string[] = [];
  const chunkPaths: string[] = [];

  class FakeXMLHttpRequest {
    upload = {
      onprogress: null as ((event: { loaded: number }) => void) | null,
    };
    status = 200;
    statusText = "OK";
    responseText = JSON.stringify({ status: "success", data: {} });
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    onabort: (() => void) | null = null;
    private path = "";

    open(_method: string, path: string) {
      this.path = path;
    }

    send(body: FormData) {
      chunkPaths.push(this.path);
      const chunk = body.get("chunk_data") as File;
      this.upload.onprogress?.({ loaded: Math.floor(chunk.size / 2) });
      this.upload.onprogress?.({ loaded: chunk.size });
      this.onload?.();
    }

    abort() {
      this.onabort?.();
    }
  }

  Object.defineProperty(globalThis, "XMLHttpRequest", {
    configurable: true,
    writable: true,
    value: FakeXMLHttpRequest,
  });
  globalThis.fetch = async (input) => {
    const url = String(input);
    if (url.endsWith("/init")) {
      return jsonResponse({
        status: "success",
        data: { upload_id: "upload-browser", chunk_size: 4, chunks: 2 },
      });
    }
    if (url.endsWith("/merge")) {
      assert.equal(stages.at(-1), "merging");
      assert.equal(progress.at(-1), 100);
      return jsonResponse({ status: "success", data: {} });
    }
    throw new Error(`Unexpected fetch request: ${url}`);
  };

  try {
    await uploadArchive({
      basePath: "/api/admin/upload",
      purpose: "backup",
      file: new File(["abcdefgh"], "backup.zip"),
      onProgress: (value) => progress.push(value),
      onStateChange: (state) => stages.push(state.stage),
    });

    assert.deepEqual(chunkPaths, [
      "/api/admin/upload/chunk",
      "/api/admin/upload/chunk",
    ]);
    assert.ok(progress.includes(25));
    assert.ok(progress.includes(75));
    assert.equal(progress.at(-1), 100);
    assert.equal(stages.at(-1), "merging");
  } finally {
    globalThis.fetch = originalFetch;
    if (hadXMLHttpRequest) {
      Object.defineProperty(globalThis, "XMLHttpRequest", {
        configurable: true,
        writable: true,
        value: originalXMLHttpRequest,
      });
    } else {
      delete (globalThis as { XMLHttpRequest?: unknown }).XMLHttpRequest;
    }
  }
});

test("retries only a failed chunk and cancels the session on terminal failure", async () => {
  const originalFetch = globalThis.fetch;
  const calls: FetchCall[] = [];
  let chunkAttempts = 0;
  globalThis.fetch = async (input, init) => {
    const url = String(input);
    calls.push({ url, init });
    if (url.endsWith("/init")) {
      return jsonResponse({
        status: "success",
        data: { upload_id: "upload-2", chunk_size: 4, chunks: 1 },
      });
    }
    if (url.endsWith("/chunk")) {
      chunkAttempts += 1;
      return jsonResponse({ status: "error", message: "temporary" }, 503);
    }
    return jsonResponse({ status: "success", data: {} });
  };

  try {
    await assert.rejects(
      uploadArchive({
        basePath: "/api/install/upload",
        purpose: "backup",
        file: new File(["data"], "backup.zip"),
        maxChunkAttempts: 2,
      }),
      /temporary/,
    );
    assert.equal(chunkAttempts, 2);
    assert.equal(calls.filter((call) => call.url.endsWith("/init")).length, 1);
    assert.equal(calls.filter((call) => call.url.endsWith("/cancel")).length, 1);
    assert.equal(calls.filter((call) => call.url.endsWith("/merge")).length, 0);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("does not report 100 percent when merge validation fails", async () => {
  const originalFetch = globalThis.fetch;
  const progress: number[] = [];
  const stages: string[] = [];
  globalThis.fetch = async (input) => {
    const url = String(input);
    if (url.endsWith("/init")) {
      return jsonResponse({
        status: "success",
        data: { upload_id: "upload-3", chunk_size: 4, chunks: 1 },
      });
    }
    if (url.endsWith("/merge")) {
      return jsonResponse({ status: "error", message: "invalid backup" }, 400);
    }
    return jsonResponse({ status: "success", data: {} });
  };

  try {
    await assert.rejects(
      uploadArchive({
        basePath: "/api/admin/upload",
        purpose: "backup",
        file: new File(["data"], "backup.zip"),
        onProgress: (value) => progress.push(value),
        onStateChange: (state) => stages.push(state.stage),
      }),
      /invalid backup/,
    );
    assert.deepEqual(progress, [0, 100]);
    assert.deepEqual(stages, ["preparing", "uploading", "uploading", "merging", "failed"]);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("upload progress copy keeps completion explicit before a dialog may close", () => {
  const completed = withUploadProgressCopy(
    createCompletedUploadState({
      percent: 100,
      uploadedBytes: 10,
      totalBytes: 10,
      uploadedChunks: 2,
      totalChunks: 2,
    }),
    {
      preparing: "Preparing",
      uploading: "Uploading",
      merging: "Merging",
      processing: "Processing",
      restarting: "Restarting",
      completed: "Completed",
      nonCancelable: "Cannot cancel",
    },
  );

  assert.equal(completed.stage, "completed");
  assert.equal(completed.percent, 100);
  assert.equal(completed.label, "Completed");
  assert.equal(completed.canCancel, false);
});

test("uses only same-origin relative upload endpoints", () => {
  const source = String(uploadArchive);
  assert.doesNotMatch(source, /https?:\/\//);
});

export type ClipboardEnvironment = {
  navigator?: Navigator;
  document?: Document;
};

export type ClipboardWriteResult = {
  confirmed: boolean;
  method: "clipboard" | "legacy";
};

function browserClipboardEnvironment(): ClipboardEnvironment {
  return {
    navigator: typeof navigator === "undefined" ? undefined : navigator,
    document: typeof document === "undefined" ? undefined : document,
  };
}

function copyWithTemporaryTextarea(text: string, documentObject?: Document): boolean {
  if (!documentObject?.body || typeof documentObject.execCommand !== "function") {
    return false;
  }

  const textarea = documentObject.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.top = "0";
  textarea.style.left = "0";
  textarea.style.width = "1px";
  textarea.style.height = "1px";
  textarea.style.opacity = "0";
  textarea.style.pointerEvents = "none";
  documentObject.body.appendChild(textarea);
  try {
    textarea.focus();
    textarea.select();
    textarea.setSelectionRange?.(0, text.length);
    return documentObject.execCommand("copy");
  } finally {
    textarea.remove();
  }
}

export async function writeClipboardText(
  text: string,
  environment: ClipboardEnvironment = browserClipboardEnvironment(),
): Promise<ClipboardWriteResult> {
  // Keep the synchronous path inside the original click gesture. Some Edge
  // profiles resolve writeText without updating the Windows clipboard, so a
  // resolved write is not treated as confirmation until it can be read back.
  const legacyCopied = copyWithTemporaryTextarea(text, environment.document);
  if (environment.navigator?.clipboard?.writeText) {
    let writeError: unknown;
    try {
      await environment.navigator.clipboard.writeText(text);
    } catch (error) {
      writeError = error;
    }

    if (environment.navigator.clipboard.readText) {
      try {
        const copiedText = await environment.navigator.clipboard.readText();
        if (copiedText === text) {
          return { confirmed: true, method: "clipboard" };
        }
      } catch {
        // Clipboard read permission is separate from write permission. An
        // unreadable result remains unconfirmed instead of reporting success.
      }
    }

    if (legacyCopied || writeError === undefined) {
      return {
        confirmed: false,
        method: legacyCopied ? "legacy" : "clipboard",
      };
    }
    throw writeError;
  }

  if (legacyCopied) {
    return { confirmed: false, method: "legacy" };
  }

  throw new Error("clipboard unavailable");
}

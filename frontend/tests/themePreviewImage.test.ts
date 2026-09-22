import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { resolveThemePreviewStatus, themePreviewSrc } from "../src/utils/themePreviewImage.ts";

const previewImageSource = readFileSync(
  "src/components/ThemePreviewImage.tsx",
  "utf8",
);

test("cached preview images resolve as loaded without waiting for onLoad", () => {
  assert.equal(resolveThemePreviewStatus(undefined, null), "error");
  assert.equal(resolveThemePreviewStatus("", null), "error");
  assert.equal(
    resolveThemePreviewStatus("/themes/glass/preview.png", null),
    "loading",
  );
  assert.equal(
    resolveThemePreviewStatus("/themes/glass/preview.png", {
      complete: false,
      naturalWidth: 0,
    }),
    "loading",
  );
  assert.equal(
    resolveThemePreviewStatus("/themes/glass/preview.png", {
      complete: true,
      naturalWidth: 640,
    }),
    "loaded",
  );
  assert.equal(
    resolveThemePreviewStatus("/themes/glass/preview.png", {
      complete: true,
      naturalWidth: 0,
    }),
    "error",
  );
});

test("preview image syncs cached complete state after layout", () => {
  assert.match(previewImageSource, /useLayoutEffect/);
  assert.match(previewImageSource, /resolveThemePreviewStatus\(src, imageRef\.current\)/);
  assert.match(previewImageSource, /onLoad=\{syncStatus\}/);
  assert.match(previewImageSource, /onError=\{syncStatus\}/);
  assert.match(previewImageSource, /decoding="async"/);
});

test("theme cards request resized previews", () => {
  assert.equal(
    themePreviewSrc("/themes/glass/preview.png", { card: true, version: "1.0.7" }),
    "/themes/glass/preview.png?card=1&v=1.0.7",
  );
  assert.equal(themePreviewSrc(undefined, { card: true }), undefined);
});

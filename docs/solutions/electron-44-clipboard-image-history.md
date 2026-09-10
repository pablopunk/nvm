# Electron 44 clipboard history lost copied images

## Problem or symptoms

Clipboard History continued to record text but stopped recording copied images after the Electron 44 clipboard migration.

The user reproduced the failure with screenshots copied as image files and later confirmed the final repair.

## Context

Electron 44 replaced the legacy synchronous image clipboard methods with the asynchronous `ClipboardItem` API. Nevermind adapted this contract in:

- `src/app/electron/electron-clipboard.ts`
- `src/app/electron/clipboard-history.ts`
- `src/app/electron/clipboard-history.test.ts`

The clipboard watcher starts as a background job. Its startup failures are written to `.tmp/dev.log` under `clipboard.watcher.start.failed`.

## What did not work

The first repair called `clipboard.readImage()` when no PNG or JPEG item was found. Electron 44 no longer exposes that function, so the watcher failed at startup with:

```text
deps.clipboard.readImage is not a function
```

Reading all visible `image/*` formats through `ClipboardItem` fixed direct image content, but it did not restore screenshots copied as files.

## Root cause

The migration changed two relevant behaviors:

1. Calling the removed `clipboard.readImage()` method crashed the watcher before it could poll any clipboard content.
2. On macOS, a screenshot app or Finder can copy both a file URL and image representations. Chromium gives the file representation priority and does not expose the image representation through `clipboard.read()`, so scanning `ClipboardItem.types` alone cannot detect this image.

The second behavior explained why old image entries still rendered correctly while newly copied screenshot files did not enter history.

## Fix

Read visible image MIME types through the Electron 44 `ClipboardItem` API and return an empty native image instead of calling removed methods.

When the clipboard contains a file URL for an image, treat that path as an image clipboard candidate, decode it with `nativeImage.createFromPath`, persist its PNG bytes through the existing clipboard-image storage path, and deduplicate it before repeated persistence.

Clipboard snapshots also preserve file paths so concealed paste and restore operations do not replace a copied file with incomplete text or image data.

## Verification

The user manually confirmed that image clipboard history worked after the image-file path repair.

Formatting and static diagnostics run for the repair:

```sh
mise exec -- pnpm format src/app/electron/electron-clipboard.ts src/app/electron/electron-clipboard.test.ts
mise exec -- pnpm check src/app/electron/electron-clipboard.ts src/app/electron/electron-clipboard.test.ts
mise exec -- pnpm format src/app/electron/clipboard-history.ts src/app/electron/clipboard-history.test.ts
mise exec -- pnpm check src/app/electron/clipboard-history.ts src/app/electron/clipboard-history.test.ts
```

Runtime tests and builds were intentionally left to CI under the repository verification policy.

## Notes for future searches

Inspect `.tmp/dev.log` before changing clipboard code. Confirm separately whether the watcher started, whether new history state was persisted, and whether existing thumbnails rendered; these boundaries distinguish capture failures from renderer failures.

Keywords: clipboard history images, Electron 44, `ClipboardItem`, `clipboard.readImage is not a function`, `clipboard.watcher.start.failed`, Finder image copy, screenshot file URL, `public.file-url`, `text/uri-list`, `image/tiff`, `image/png`, CleanShot, `nativeImage.createFromPath`.

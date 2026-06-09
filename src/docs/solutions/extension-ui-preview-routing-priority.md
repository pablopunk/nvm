# Extension UI Preview Routing Priority

## Problem or symptoms

Extensions that pass a file object with `kind: 'image'` or `kind: 'video'` to `ctx.ui.preview(file, view)` get a blank preview panel — no image or video renders, only subtitle text shows.

Concrete case: the Screenshots extension sets `kind` on its hydrated file objects (`adjustedFile.kind = 'image'`) before calling `ctx.ui.preview()`. Clicking any grid tile opened an empty preview.

## Context

`ctx.ui.preview()` in `src/electron/extension-ui-api.ts` routes to three code paths:

1. **File preview** — when the input has `path` or `fileUrl` (file objects), it builds a full preview view with `image`, `video`, `title`, etc.
2. **Clipboard/item preview** — when the input has `kind` matching `['clipboard', 'image', 'video', 'file', 'text']`, it delegates to `buildPreviewItemAction()` which returns a `previewClipboardItem` action (no `view.image`).
3. **Plain view** — fallback that stamps `type: 'preview'` on the input.

The `kind` field is overloaded: `ExtensionFile.kind` uses `'image' | 'video' | 'file'`, and preview-item descriptors also use `kind: 'image' | 'video' | 'file' | 'clipboard' | 'text'`. A file with `kind: 'image'` matches both branches.

## What did not work

Simply swapping the priority without changing `isFilePreviewInput` would fix file previews but break clipboard previews — clipboard descriptors with `videoUrl` or `thumbnailUrl` would then match `isFilePreviewInput` and get misrouted to the file path.

Separating the two paths with an additional property check on the file object was not enough; the shared fields needed to be removed from the file-detection guard.

## Root cause

`isPreviewableItem` was checked **before** `isFilePreviewInput`. Since both shapes can carry `kind: 'image'`, files were misrouted to `buildPreviewItemAction`, which returns an action object without `.view.image` — the renderer showed a `PreviewView` with no media.

Additionally, `isFilePreviewInput` originally included `videoUrl` and `thumbnailUrl`, which are present on both file objects and clipboard preview descriptors. This made the check non-discriminating.

## Fix

Two changes in `src/electron/extension-ui-api.ts`:

1. **Reorder checks**: `isFilePreviewInput` runs first, so file objects always take the preview-view path regardless of `kind`.
2. **Tighten `isFilePreviewInput`** to only `value?.path || value?.fileUrl`. `path` is required on `ExtensionFile` and absent from clipboard descriptors; `fileUrl` is a Nevermind custom-protocol URL only present on host-hydrated files.

```ts
// Before
function isFilePreviewInput(value: any) {
  return value?.path || value?.fileUrl || value?.videoUrl || value?.thumbnailUrl
}
preview: (fileOrView, view = {}) => {
  if (isPreviewableItem(fileOrView)) return buildPreviewItemAction(fileOrView)
  if (!isFilePreviewInput(fileOrView)) return { ...fileOrView, type: 'preview' }
  // ...
}

// After
function isFilePreviewInput(value: any) {
  return value?.path || value?.fileUrl
}
preview: (fileOrView, view = {}) => {
  if (isFilePreviewInput(fileOrView)) {
    // file preview path
  }
  if (isPreviewableItem(fileOrView)) return buildPreviewItemAction(fileOrView)
  return { ...fileOrView, type: 'preview' }
}
```

## Verification

```bash
cd /Users/pablopunk/src/nvm && mise exec -- pnpm -C backend exec node --import tsx --test ../src/electron/extension-ui-api.test.ts
```

The existing test validates all three routing branches:

```ts
// Clipboard preview (kind: 'image', no path/fileUrl) → buildPreviewItemAction
ui.preview({ kind: 'image', title: 'Image' })
// File preview (has fileUrl) → preview view
ui.preview({ name: 'Report', displayPath: '~/Report.pdf', fileUrl: 'file://report', thumbnailUrl: 'thumb://report' })
// Plain view (no kind, no path/fileUrl) → stamped preview type
ui.preview({ title: 'Plain', content: 'Markdown' })
```

## Notes for future searches

Keywords: extension preview blank, preview images not showing, screenshots preview broken, isPreviewableItem, isFilePreviewInput, buildPreviewItemAction, kind field collision, ExtensionFileKind, ui.preview routing, ctx.ui.preview file object, extension-ui-api.ts preview function.

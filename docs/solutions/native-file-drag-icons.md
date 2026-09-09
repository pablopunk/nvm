# Non-image results needed a visible native drag icon

## Problem or symptoms

Screenshot grid items could be dragged out of the palette, but list and root results backed by ordinary files, directories, or macOS `.app` bundles could not.

The broken rows had `draggable="true"`, their `dragstart` handlers ran, and they supplied valid absolute paths to Electron's `webContents.startDrag`.

## Context

The working Screenshots extension lives in the installed user-extension directory and returns image-backed grid items with a file `path`. Electron can decode those screenshot paths as drag images.

Relevant paths:

- `src/app/palette/App.tsx`
- `src/app/palette/command-list.tsx`
- `src/app/palette/extension-view.tsx`
- `src/app/palette/ui.tsx`
- `src/app/electron/main.ts`
- `src/app/electron/file-drag.ts`

## What did not work

These changes were necessary for complete row support but did not fix the native failure:

- Adding `draggable` and `onDragStart` to list and root rows.
- Preserving file paths while converting root actions into command items.
- Falling back to paths on extension-item actions.
- Setting nested row images to `draggable={false}` so the row owned the gesture.

Checking only the DOM and renderer event proved that the request started, not that Electron created a usable native drag session.

## Root cause

`nativeImage.createFromPath(filePath)` can decode screenshots and other image files, so the Screenshots grid received a real 64 px drag image. It returns an empty image for ordinary files, directories, and application bundles.

The fallback was a 1×1 PNG. That unusable native drag image made every non-image case appear unable to leave the palette even though the renderer and IPC path were correct.

## Fix

Keep image-backed dragging unchanged and use a visible 48×48 PNG when Electron cannot decode the dragged path. The fallback is constructed as an Electron `NativeImage` in the shared main-process path, so the behavior is not tied to macOS.

`src/app/electron/file-drag.ts` owns drag-image selection and focused contract coverage. `src/app/electron/main.ts` still calls the existing `webContents.startDrag` flow.

## Verification

The user manually confirmed that dragging worked after the fallback changed.

Formatting and static diagnostics run for the repair:

```sh
mise exec -- pnpm exec biome check --write src/app/electron/file-drag.ts src/app/electron/file-drag.test.ts src/app/electron/main.ts
git diff --check
```

Runtime tests and builds were intentionally left to CI under the repository verification policy.

## Notes for future searches

Keywords: native file drag, `webContents.startDrag`, `drag:file`, `nativeImage.createFromPath`, one-pixel icon, 1×1 PNG, directory drag, folder drag, `.app` bundle drag, Screenshots extension, grid drag, list drag, root result drag.

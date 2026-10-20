# Clipboard history images were never deleted

## Problem or symptoms

`<userData>/clipboard-images` grew without bound. One real profile held 2,141 PNGs (~480 MB) while history referenced about 125 of them.

## Root cause

`persistClipboardImage` writes `<sha1(png)>.png` when an image is copied. History is capped and entries can be removed, but both paths only dropped the history record. No code owned the file lifetime.

## Fix

A host job (`clipboard.images.gc`, 30 s after startup, then hourly) deletes unreferenced PNGs. Cleanup is eventual, not a hard invariant: extensions and open views can still hold an `imagePath`, so `copyImage` refuses an empty image with an error toast.

A PNG is deleted only when all of these hold:

- This process started from a valid `state.json` (`userStateStore.loadedStatePath()`). A corrupt, missing or unreadable load leaves history empty and must never trigger a sweep.
- No entry references it by `imagePath` basename in live history or in `state.json` on disk. Never match by entry id: `image-file:<sha1(path)>` entries use a different id than their PNG name, and several entries can share one PNG.
- It matches `^[0-9a-f]{40}\.png$`, is a regular file (no symlinks or subfolders), and is older than 10 minutes.
- After a fresh `lstat`, live history is re-checked synchronously right before `unlink`.

Each run is single-flight, stops after about 15 s, and deletes at most 500 files.

## Known limits

- Dev and packaged builds share one user data folder. If one instance removes an entry another still lists, the other shows a broken thumbnail until its next save; state is already last-writer-wins.
- Restoring a `state.json.bak-*` can bring back entries whose PNGs were swept.
- The thumbnail cache (`<osCache>/icons/thumbs`) has no cleanup of its own. It is a separate follow-up.

## Verification

Sweeper and `copyImage` unit tests cover each guard (`clipboard-image-sweeper.test.ts`, `clipboard-actions.test.ts`); they run in CI.

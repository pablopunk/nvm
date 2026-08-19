# Instant Extension Media Grids

## Problem or symptoms

A media-heavy generated extension such as Screenshots can show the macOS beach ball, lag the palette, or sit on an `Opening…` progress view for seconds or minutes when launched from a global shortcut.

Users expect shortcut-launched extension grids to feel instant even if the newest filesystem state is not available yet. Stale-but-useful rows are preferable to blocking first paint.

## Context

Generated extensions live under the Electron `userData/extensions` directory. In this app that path is derived in main from:

```ts
path.join(app.getPath('userData'), 'extensions')
```

For debugging generated extensions, inspect that directory directly instead of searching broad home directories.

The Screenshots extension scanned roots such as Desktop, Downloads, and CleanShot folders using `ctx.desktop.files.findMedia(..., { sortBy: 'added' })`. Logs showed the hot path in `extension.root-item.handler` taking tens of seconds, with examples around 53s, 90s, and 138s.

## What did not work

- Limiting each root before combining results made opening faster but broke ordering: newer files that appeared later in directory order could be omitted before the global newest-first sort.
- Sorting an instant file-index snapshot by filename was fast but incorrect for "last inserted should be first".
- Keeping a short extension-level cache made the shortcut faster, but new screenshots were hidden until cache expiry.
- Showing `isLoading` on the initial grid was visually sticky when refreshes patched only rows.

## Root cause

There were two related issues:

1. `findFiles` applied the result limit during directory traversal before collecting all candidates and sorting. With `sortBy: 'added'`, this made the API semantically wrong: it could return older files simply because they were encountered first.
2. The extension awaited an expensive filesystem/native metadata scan before returning its first view. Shortcut launch therefore blocked on file walking, `stat`, Spotlight `mdls` date-added lookup, and media URL hydration.

Image dimension loading was another avoidable cost: `nativeImage.createFromPath` during broad scans can block media grids and should be opt-in.

## Fix

The API should expose two distinct paths and one cache handoff:

- **Instant snapshot path**: read a persisted extension snapshot from `ctx.storage` for first paint; fall back to `ctx.desktop.files.recent()` / `indexSnapshot()` if no extension snapshot exists.
- **Accurate refresh path**: `ctx.desktop.files.find*()` may scan and enrich metadata. Run it after the view is visible, usually via host-owned `view.refresh`.
- **Cache promotion**: the refresh result must update the persisted snapshot that the next first paint reads. If refresh only patches the visible view, every reopen starts stale again.

Concrete changes from this case:

- `findFiles` collects candidates up to a scan cap, attaches cheap stats/date-added metadata only when needed for sorting, sorts globally, then hydrates only selected rows.
- `findFiles(..., { sortBy: 'added' })` sorts by `dateAddedMs` with creation/modified fallback after collection, not before.
- `includeDimensions` defaults to false; callers should use `metadata(path)` or explicit `includeDimensions` only for detail hydration.
- `fileDateAddedMs` batches `mdls` work with bounded concurrency.
- `files.recent()` / `indexSnapshot()` return timestamp and host-safe URL fields, sorted newest-first by default.
- `view.refresh.immediate` lets an extension render a cheap snapshot first and trigger a host-owned refresh immediately after paint.
- `ctx.launch.refresh` lets the command distinguish first paint from refresh work.

The Screenshots extension pattern became:

```ts
const files = ctx.launch?.refresh
  ? await refreshFilesAndPersistSnapshot(ctx)
  : await readPersistedSnapshotOrHostIndex(ctx)

return ctx.ui.grid({
  title: 'Screenshots',
  items: files.map(toGridItem),
  refresh: { immediate: true, intervalMs: 60_000, mode: 'replace' },
})
```

## Verification

Run:

```bash
mise exec -- pnpm typecheck
mise exec -- pnpm test
```

Dogfood the generated Screenshots extension:

- Press its global shortcut with the palette hidden.
- The grid should appear immediately from the persisted extension snapshot without beach-balling.
- A newly inserted screenshot may appear after the immediate refresh, not before first paint.
- After that refresh, closing and reopening the extension should show the new screenshot immediately from the promoted cache.
- The first three rows should match Finder/Spotlight "date added" ordering after refresh.
- Image and video tiles should use host thumbnail URLs; previews should use full file URLs.
- Logs should show shortcut/open latency in milliseconds for first paint, with any heavier `findMedia` work under `view.refresh.host-action` rather than `extension.root-item.handler`.

## Notes for future searches

Keywords: Screenshots extension, media grid, beach ball, Opening active, shortcut latency, first paint, lazy refresh, view.refresh.immediate, ctx.launch.refresh, files.recent, indexSnapshot, findMedia, sortBy added, kMDItemDateAdded, mdls, dateAddedMs, includeDimensions, nativeImage, thumbnailUrl.

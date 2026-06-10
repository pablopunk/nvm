# Non-Blocking Data Loader API

**Status: Implemented** (2026-06-10)

## Problem

Extension `command.run(ctx)` blocks the palette UI until it returns. There is no way to paint a
skeleton first and fill in data later. The `refresh: { immediate: true }` escape hatch is fragile:

- Re-invokes the entire `run()` function a second time.
- Races between the initial paint and the refresh IPC.
- Discards non-item view metadata (`isLoading`, `emptyView`) during refresh.
- Requires the extension to bifurcate logic with `ctx.launch?.refresh`.

Extension authors cannot build progress UIs, streaming UIs, or lazy-loaded lists without
contending with these races.

## Goal

Make it **impossible to block the UI** from an extension. The common case (fetch → list) should
be one extra method call. The host owns the loading/error/empty lifecycle.

## Design

### Extension API

```ts
// New namespace: ctx.data
type ExtensionData = {
  /**
   * Declare items that resolve asynchronously. The host:
   * 1. Paints the skeleton immediately (spinner deferred 200ms).
   * 2. Calls loader() in the background.
   * 3. Patches items when resolved.
   * 4. Renders emptyView when loader returns [].
   * 5. Renders an error view when loader throws.
   */
  loader<T extends ExtensionItem[]>(
    fn: () => Promise<T>,
    options?: { retry?: boolean }
  ): ExtensionDataLoaderHandle
}

type ExtensionDataLoaderHandle = {
  /** Opaque handle; the host replaces this with items after resolution. */
  _loader: true
}
```

`ctx.data.loader(fn)` returns an opaque handle. The host recognizes it during
`normalizeViewItems` / `normalizeExtensionView` and sets up the async pipeline.

### Required `emptyView`

Views that use `ctx.data.loader()` must declare `emptyView`. The skeleton paints the `emptyView`
content immediately (behind the deferred spinner), so there's never a flash of nothing:

```ts
commands: [{
  id: 'show-prs',
  title: 'Show My Pull Requests',
  async run(ctx) {
    return ctx.ui.list({
      title: 'My Pull Requests',
      emptyView: { title: 'No open PRs', subtitle: 'You have no open pull requests' },
      items: ctx.data.loader(async () => {
        const result = await ctx.desktop.shell.script('gh search prs …')
        return JSON.parse(result.stdout).map(toItem)
      }),
    })
  }
}]
```

`run()` returns synchronously (the view object is constructed before any `await`). The host
paints the skeleton, then invokes the loader. The extension does not manage timing or loading
state.

### Spinner debounce

The renderer defers the loading spinner by 200ms. If the loader resolves before the threshold,
the spinner is never shown. This prevents flicker on fast data.

```
t=0      Paint skeleton + emptyView (no spinner)
t=120    Loader resolves → patch items, spinner never appeared
```

```
t=0      Paint skeleton + emptyView
t=200    Spinner fades in
t=850    Loader resolves → patch items, spinner dismissed
```

The threshold is host-owned, not configurable by the extension (to guarantee consistency).

### Error state

When the loader throws, the host renders a standard error view. If `options.retry` is true, the
view includes a "Retry" button that re-runs the loader.

```
t=0      Paint skeleton
t=200    Spinner appears
t=500    Loader throws → error view with message, optional retry button
```

### Progress views stay imperative

`ctx.data.loader()` covers the fetch → list case. For multi-step progress (media compression),
`ctx.paint()` remains available as a separate follow-up — out of scope for this spec.

## Execution flow

### Host side (main.ts)

```
run() returns view with items = LoaderHandle
  │
  ├─→ normalizeViewItems: detect LoaderHandle, strip it, set items: []
  ├─→ normalizeView: inject refresh-like handle for the loader
  ├─→ send view to renderer IMMEDIATELY (do not await loader)
  │
  └─→ spawn background job:
        loader()
          .then(items => send view:hydrate { viewId, items, isLoading: false })
          .catch(err  => send view:hydrate { viewId, error: message, retry: bool })
```

Key: the IPC result for `runViewAction` / `execute` returns the skeleton view **before** the
loader resolves. The loader result arrives later via a `view:hydrate` push message.

### IPC

New channel: `view:hydrate`

```ts
// preload.ts
onViewHydrate: (callback) => {
  ipcRenderer.on('view:hydrate', listener)
}

// main.ts → renderer
paletteWindow.win?.webContents.send('view:hydrate', {
  viewId: string,
  items?: ExtensionItem[],      // present on success
  isLoading?: false,             // always false when items present
  emptyView?: …,                 // carried from initial view
  error?: { message: string },   // present on failure
  retry?: boolean,               // whether to show a retry button
})
```

### Renderer side (App.tsx)

```ts
useEffect(() => {
  return window.nvm.onViewHydrate((payload) => {
    if (payload.viewId !== extensionViewRef.current?.id) return
    if (payload.error) {
      // Replace view with error state
      showExtensionView(errorView(payload.error, payload.retry), 'replace')
      return
    }
    applyViewPatch({
      mode: 'replace',
      items: payload.items,
      isLoading: false,
    })
  })
}, [])
```

## What happens to `refresh: { immediate: true }`?

It stays working for backward compatibility but is deprecated. The `ctx.data.loader()` host
pipeline is implemented on top of the same `view:hydrate` IPC rather than the fragile
double-`run()` pattern. Existing extensions using `refresh` continue to work unchanged.

## Implementation steps

1. **Type definitions** — add `ctx.data` to `ExtensionContext`, add `ExtensionDataLoaderHandle` type to `nevermind-extension-api.d.ts`.

2. **`createExtensionContext`** — add `data: { loader(fn, opts) }` namespace.

3. **`normalizeViewItems`** — detect `LoaderHandle`, record the loader against a view id, return `[]` for items.

4. **Loader registry** — `Map<viewId, { fn, retry }>`. Register on initial view send, clean up on view close/pop.

5. **`executeViewActionResult` / `executeViewAction`** — when a view has a pending loader, send the skeleton immediately via `action:view-open`, then spawn the loader in background. The await on `run()` is still required (it returns fast since `ctx.data.loader()` is synchronous), but the loader runs after.

6. **IPC** — add `view:hydrate` channel to preload, main sender, renderer listener.

7. **Spinner delay** — in `ExtensionViewRenderer`, when `items.length === 0 && emptyView` and no error, start a 200ms timer before showing spinner. Cancel on hydrate.

8. **Error rendering** — host generates error view from `payload.error`, includes optional retry button that re-registers and re-runs the loader.

9. **`emptyView` required** — at runtime, if a view has `ctx.data.loader()` items but no `emptyView`, warn and use a default empty state. TypeScript types should mark it as required in the relevant overload.

10. **Tests** — new unit test for loader lifecycle, hydration IPC, spinner debounce, error/retry, empty state.

## Risks

- **`run()` still blocks on synchronous work.** If the extension does sync work before returning the skeleton (e.g., `fs.readFileSync` on a huge file), the UI blocks. The fix for this is `ctx.paint()` (out of scope for this spec).
- **Multiple loaders per view.** Not supported in v1. If an extension calls `ctx.data.loader()` twice in one view, the second call replaces the first.
- **Loader surviving navigation.** If the user navigates away before the loader resolves, the hydration message is dropped (viewId mismatch). The background work completes but results are discarded.

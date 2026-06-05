# Host-Owned Extension View Refresh

## Problem or symptoms

Extension views with polling refresh can become laggy, show repeated error previews, or fail with:

```text
Error: Untrusted runExtensionAction action
```

This is especially visible in media-heavy grids such as the Screenshots extension, where frequent refreshes also make thumbnail loading appear broken or unstable.

## Context

Extension views are normalized in the main process, then sent to the renderer over Electron IPC. Executable extension actions are intentionally tokenized and registered server-side so the renderer cannot forge privileged work.

The original `view.refresh` shape sent a normal executable action to the renderer:

```ts
refresh: { intervalMs, action: ctx.views.refresh(), mode }
```

The renderer stored that action and invoked it repeatedly on an interval.

## Root cause

Polling refresh was modeled as a renderer-held executable action token instead of a host-owned lifecycle primitive. That made refresh depend on volatile `runExtensionAction` execution records and handler IDs across view replacement, HMR, extension reloads, and other host lifecycle changes.

When the token became stale or untrusted, the normal action error path returned an error preview. Because the renderer timer kept running, the view could repeatedly replace itself with failures or keep retrying stale work.

## Fix

Refresh is now an opaque host-owned handle:

- `normalizeView` registers `view.refresh` in main via `registerViewRefreshForRenderer`.
- The renderer receives only clone-safe metadata: `{ id, intervalMs, mode }`.
- The renderer calls `view:refresh` with the opaque id instead of invoking a refresh action.
- Main executes the registered refresh action or command with a fresh extension context.
- Stale, unknown, backoff, or already-running refreshes return `{ skipped: true }` instead of an error preview.
- Refresh results are structured-clone checked before returning.

## Verification

Run:

```bash
mise exec -- pnpm test
```

Dogfood the Screenshots extension and confirm:

- Renderer view payloads have `refresh.id` but no `refresh.action`.
- `window.nvm.refreshView({ id, viewId })` returns patches for valid visible views.
- Logs do not contain `Untrusted runExtensionAction action` during polling.
- CPU remains stable while the grid is open.

## Notes for future searches

Keywords: extension refresh, view.refresh, runExtensionAction, Untrusted action, polling, host-owned lifecycle, opaque refresh handle, Screenshots extension.

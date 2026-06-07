# Running app status blocked palette responsiveness

## Problem or symptoms

The palette felt laggy or unusable while opening, searching, or rendering app results. Performance logs showed slow decorative running-app status checks:

- `apps.running.snapshot` around 800–1460ms
- `apps.running.get` / `ipc.apps:running-paths` waiting on that snapshot
- palette `showPalette.later` callbacks delayed by roughly 1.2–3.5s

## Context

Root app results render a running/open indicator via `runningAppClassName` in `src/App.tsx`. That status is visual decoration; it must never gate user-visible search, input, command execution, or palette reveal.

Relevant paths:

- `src/App.tsx`
- `src/electron/main.ts`
- `src/electron/os.ts`
- `src/electron/preload.ts`
- `src/preload-api.ts`

## What did not work

Only making the macOS running-app detector faster was a symptom patch. Replacing a slow native call with a faster native call still left a decorative UI affordance on the renderer-to-main request path, so future regressions could again make palette interactions wait on OS process inspection.

## Root cause

`apps:running-paths` awaited `runningAppPathSnapshot()`. When the cached snapshot was missing or stale, the IPC handler synchronously waited for native running-app detection before responding to the renderer.

On macOS, the detector used AppleScript through System Events, which could take around a second. Multiple renderer requests then piled up behind the same snapshot work, making the palette feel blocked even though the running indicator was not required for primary results.

## Fix

Treat running-app status as stale-while-revalidate decoration:

- `apps:running-paths` returns immediately from the last cached snapshot, or an empty snapshot if none exists yet.
- A stale or missing snapshot schedules background refresh and does not block the IPC response.
- The host dedupes in-flight refreshes.
- When the refreshed snapshot changes, main sends `apps:running-paths-changed` to the renderer.
- The renderer listens for that event and re-queries only to update row decoration.
- macOS detection uses bounded `ps` process inspection instead of AppleScript, so the background refresh is cheaper too.

## Verification

Run:

```sh
mise exec -- pnpm typecheck
mise exec -- pnpm build
```

Both passed after the change.

## Notes for future searches

Keywords: running app indicator, open app marker, `apps:running-paths`, `apps.running.snapshot`, `apps.running.get`, `ipc.apps:running-paths`, stale-while-revalidate, background refresh, decorative status, AppleScript, System Events, `ps -axo comm=`, palette lag, user actions should be instant.

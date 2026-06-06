# Host View Action Tokenization Before IPC

## Problem or symptoms

A host-owned view can show an action failure preview like:

```text
Error: Untrusted downloadUpdate action
    at resolveViewActionForIpc
    at executeViewActionForIpc
```

This was seen from the Updates surface after selecting a `downloadUpdate` primary action rendered in a host view.

## Context

Nevermind treats Electron IPC as a trust boundary. Privileged view actions such as `downloadUpdate`, `installUpdate`, `runExtensionAction`, filesystem actions, shell actions, and system actions must be registered in the main process and sent to the renderer with an `executionId` token. The renderer may pass form values or selection data back, but it must not be able to forge privileged action payloads.

Extension views normally go through `normalizeView` / `normalizeViewAction`, which registers actions before IPC. Some internal host paths returned or patched views directly from main-process helpers, so their actions could skip tokenization.

## Root cause

The host result paths for `actions:execute`, `view-action:execute`, `view:refresh`, and `view:patch` were structured-clone checked, but not consistently normalized before crossing IPC. A direct host view result could therefore contain a raw privileged action like:

```ts
{ type: 'downloadUpdate', title: 'Download Update' }
```

When the renderer invoked it, `resolveViewActionForIpc` correctly rejected the action because it had no trusted `executionId`.

## Fix

Normalize host-owned view results and patches immediately before returning or sending them over IPC:

- `executeActionForIpc` normalizes returned `view` and `patch` payloads.
- `executeViewActionForIpc` normalizes returned `view` and `patch` payloads.
- `refreshViewForIpc` normalizes refresh results before clone-checking and returning them.
- `patchOpenView` normalizes patches before sending `view:patch`.
- Existing opaque `refresh.id` handles are preserved instead of re-registering refresh actions.

This keeps the renderer-facing contract consistent: any privileged action visible to the renderer is either renderer-only or has a main-owned execution token.

## Verification

Run:

```bash
mise exec -- pnpm typecheck
mise exec -- pnpm test
```

Dogfood the Updates view and confirm that selecting “Download Update” no longer shows `Untrusted downloadUpdate action`.

## Notes for future searches

Keywords: Electron IPC, view-action:execute, actions:execute, view:patch, host view, internal view, Updates, downloadUpdate, installUpdate, Untrusted action, executionId, normalizeViewAction, tokenized actions.

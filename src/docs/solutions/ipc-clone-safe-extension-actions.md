# IPC Clone-Safe Extension Actions

## Problem or symptoms

Electron search IPC can fail with:

```text
Error occurred in handler for 'actions:search': Error: An object could not be cloned.
```

When this happens, opening the palette or typing a query may show no results even though providers and extensions otherwise load.

## Context

Nevermind extension commands, actions, root items, fixtures, and action panels can start as rich in-process objects that include handler functions such as `run` or `__handler`. Renderer IPC payloads cannot contain functions; they must contain clone-safe data and stable handler IDs only.

This surfaced after adding background-job fixtures. A persistent fixture action spread a registered extension action item into a view item. That copied a raw `run` function into a search/view payload. `JSON.stringify`-based debug output hid the problem, but Electron IPC rejected it.

## What did not work

- Relying on `palette:debug` alone was insufficient because JSON output drops functions silently.
- Inspecting printed search results did not reveal the uncloneable field for the same reason.

## Root cause

The extension normalization path registered executable handlers but did not consistently remove raw executable fields from every object sent to the renderer. Specifically, view item normalization needed to strip `run`, `__handler`, and raw `action` after converting the executable action to a `handlerId`. Action-panel sections also needed to drop `lazyActions` after normalization.

## Fix

- Normalize item primary actions from `item.primaryAction || item.action`.
- Strip raw `run`, `__handler`, and `action` fields from view items before IPC.
- Strip `lazyActions` from action-panel sections after converting them to normal actions.
- Add `structuredClone(sorted)` in `searchActions` so uncloneable search results fail before Electron IPC.
- Keep `structuredClone(result)` in `executeActionForIpc` for action results.
- Add `scripts/check-clone-safe-actions.cjs` to guard the expected normalization and clone checks.

## Verification

Run:

```bash
mise exec pnpm -- pnpm test
mise exec pnpm -- pnpm palette:debug
```

Then dogfood live search with the dev app and CDP:

```bash
mise exec pnpm -- pnpm dev
agent-browser --cdp 9222 eval "window.nvm.hide(); window.nvm.shortcutReady()"
agent-browser --cdp 9222 eval "Promise.all(['', 'background', 'fixtures'].map(q => window.nvm.search(q).then(r => ({ q, count: r.length, first: r[0]?.title }))))"
```

Check the dev log for absence of:

```text
An object could not be cloned
Error occurred in handler for 'actions:search'
```

## Notes for future searches

Keywords: Electron IPC, structuredClone, actions:search, view-action IPC, extension run handler, __handler, lazyActions, clone-safe payload, Background Tasks fixture.

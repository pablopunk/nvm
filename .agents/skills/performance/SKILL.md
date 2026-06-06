---
name: performance
description: >-
  Use when changing or reviewing performance-sensitive Nevermind paths: command
  palette search, keyboard input, AI chat streaming, extension views, cache
  invalidation, IPC, Electron window behavior, file/icon/thumbnail/clipboard work,
  or any request mentioning lag, latency, responsiveness, jank, render cost, or
  “performance is UX”. Trigger before making assumptions; explore the repo and
  identify the hot path first.
---

# Performance

Performance is UX. In Nevermind, every millisecond affects trust because the product is a keyboard-first command palette. Treat perceived latency, input responsiveness, and visual stability as product requirements, not polish.

## How to use this skill

1. **Explore before assuming.** Identify the exact interaction path: renderer input → IPC → main/provider work → cache/revalidation → render/update.
2. **Turn on local traces.** Debug performance marks are dev-gated by default. In the renderer, use `localStorage.setItem('nvm.debugPerformance', 'true')` if needed and tune noisy logs with `localStorage.setItem('nvm.debugPerformance.slowMs', '16')`. In main/CLI runs, use `NVM_DEBUG_PERFORMANCE=1` and optionally `NVM_DEBUG_PERFORMANCE_SLOW_MS=16`.
3. **Read the logs before fixing.** Use `mise exec -- pnpm logs:tail` or inspect the app log for `performance.measure` entries. Compare paired layers such as `search.renderer-to-results`, `ipc.actions:search`, `search.actions`, `extension.search.provider`, `root-actions.to-command-items`, `app.commit`, `view-action.ipc`, `ipc.palette:hide`, and `palette-window.show` to locate where time is spent.
4. **Separate perceived and actual latency.** Prefer showing cached/snapshot data immediately, hydrating expensive details later, and refreshing in place.
5. **Find the blocking edge.** Look for synchronous CPU, awaited fanout, repeated IPC, re-render storms, layout reads/writes, heavy serialization, filesystem/native calls, and broad cache invalidation.
6. **Fix the disease, then re-measure.** Prefer no-op guards, in-flight dedupe, request coalescing, stale-result drops, memoization, and lazy hydration before architectural rewrites. Confirm the same `performance.measure` names improve instead of adding unrelated patches.
7. **Verify with the closest real flow.** Use `mise exec -- pnpm typecheck`, `mise exec -- pnpm test` for safety, and `mise exec -- pnpm palette:debug` for provider/search behavior; manually dogfood UI changes when render/input feel matters.

## Hot paths to inspect first

- Root search: `src/use-search-results.ts`, `src/electron/main.ts` search/provider fanout, ranking, cache invalidation.
- Input/rendering: `src/App.tsx`, `src/filtering.ts`, `src/ui.tsx`, `src/extension-view.tsx`.
- Chat streaming: `src/use-ai-chat.ts`, markdown rendering in `src/App.tsx`, AI event forwarding in `src/electron/ai.ts` and `src/electron/main.ts`.
- Native/Electron work: `src/electron/palette-window.ts`, app icons, thumbnails, clipboard polling, filesystem scans, logging.
- Extension API surfaces: `src/resources/nevermind-extension-api.d.ts` when fixing missing primitives rather than bypassing extension APIs.

## Rules

- **Input must stay responsive.** Never add work to keystroke paths without debounce, cancellation, coalescing, or a hard cap.
- **Do not await decoration.** Icons, thumbnails, previews, and metadata should hydrate after primary results appear.
- **Drop stale work.** Search, selection previews, and chat/UI patches must ignore old results when the active query/view/selection changed.
- **Batch streams.** Token-by-token AI updates should be coalesced to animation frames or similarly bounded render cadence.
- **No broad invalidation by default.** Invalidate the smallest provider/cache/view possible; broad root invalidation is a smell unless the data truly affects all roots.
- **Treat polling and refresh as lifecycle ownership problems.** For laggy extension views, audit renderer-held action tokens, IPC clone-safety, staleness, in-flight dedupe, and host-owned refresh primitives before suppressing repeated errors.
- **Avoid render-time derivation loops.** Memoize filtered sections/items, flattened view items, markdown nodes, and action rows when they depend on stable inputs.
- **Prevent avoidable re-renders.** Keep state as local as possible, split components around independent update frequencies, stabilize callbacks/props passed to large lists, and avoid rebuilding arrays/objects/React nodes in parent render paths unless their inputs changed.
- **Do not let selection/input churn redraw everything.** Typing, arrow-key movement, chat token deltas, and hover/preview changes should update only the affected surface or row; use refs for mutable bookkeeping that does not affect paint.
- **Memoize expensive presentation.** Markdown, icons, accessories, action panels, filtered sections, and large list rows should be memoized or moved into memoized child components before adding more state to `App.tsx`.
- **Guard Electron window operations.** Resizing, centering, opacity changes, and IPC mode updates should no-op when state is unchanged.
- **Preserve immediate dismissal.** For `dismissAfterRun: 'auto'` native/app actions, verify `ipc.palette:hide` and `palette-window.hide` happen before slow execute/open work; enqueue hide before execute IPC and defer native launch work out of the current IPC turn when needed.
- **Treat native calls as expensive.** `nativeImage`, `file-icon`, clipboard image conversion, filesystem scans, and log reads need caching, limits, and in-flight dedupe.
- **Measure or bound.** If exact measurement is hard, add conservative caps/timeouts and document the remaining risk.

## Review checklist

- Does typing trigger only the minimum required IPC/provider work?
- Can older async results overwrite newer UI state?
- Does opening the palette show useful cached results immediately?
- Are expensive visual assets lazy and deduped?
- Does chat streaming re-render or re-parse more than necessary?
- Are cache invalidations scoped to the changed extension/surface?
- Are large lists filtered/rendered with memoization or caps?
- Does this state change force unrelated rows, sibling panes, markdown, or action panels to re-render?
- Are callback/array/object props stable enough for memoized children to help?
- Are window/layout operations guarded against no-op repeats?
- Do immediate-dismiss actions hide the palette before slow native/app work starts?
- Did verification cover both tests and the real UX path?

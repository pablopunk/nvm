# Agent Guidelines

## Workflow

* Use `mise exec pnpm` for package-manager commands. Never call npm, yarn, pnpm, or bun directly; Node/pnpm are pinned in `mise.toml`.
* Before changing code, inspect nearby conventions and reuse existing helpers, overlays, primitives, formatters, confirmation flows, shortcut plumbing, and OS capabilities.
* Treat unexpected file changes as user work; do not overwrite them.
* Use git while iterating: inspect history when useful and commit early/often.

## Architecture

* Keep files small, focused, and performant; fix slow patterns when you touch them.
* Prefer generic first-party primitives over one-off helpers. Fix shared models/lifecycles instead of compensating in UI.
* OS-specific desktop behavior goes through intent-named capabilities in `src/electron/os.ts`; follow `src/docs/os-architecture.md` and never gate one OS mechanism through an unrelated capability.
* Extension APIs are declarative host-controlled contribution points, not app-internal backdoors; follow `src/docs/extension-api.md` and `src/docs/design-system-and-extension-api.md`.
* Extensions are first-class app contributors. Preserve native behavioral contracts when migrating features: return shape, identity, selection, shortcuts, icons/media, dismissal, and async lifecycle.
* Extension command/action boundaries must catch, normalize, and clone-check results so failures render as Nevermind error views, not raw IPC/log errors. When debugging, inspect both repo code and installed/generated artifacts in `~/Library/Application Support/nvm/extensions`.
* AI builder chats provide history/context and write scope. Generated extension files are durable; deleting chat history must not delete code. AI writes are limited to extension files already touched/owned by that chat.

## Product and UX

* Nevermind is command-k first: decisions, confirmations, warnings, and configuration live in the palette, usually as `preview`/`list` views with action panels or existing-view items. Never use `window.confirm`, `window.prompt`, `window.alert`, or a separate preferences window.
* Async UX must show a stable intended surface or cached snapshot immediately, then refresh in place; see `src/docs/extension-api.md` for view/action lifecycle expectations. External handoff actions hide the palette before OS work and continue in the background. Do not insert/reorder passive visible lists unless the user explicitly refreshes/navigates; loading/progress surfaces must not pollute extension navigation history.
* Scope presentation state to context: reset filters/search when navigating unless inheritance is intentional; action panels/submenus own palette sizing and selection while open.
* Do not show empty-state UI in passive content surfaces; reserve empty states for result/action lists.
* New settings use the existing pipeline: add `SETTING_DEFINITIONS` in `src/electron/main.ts`, persist under `userState.settings`, and surface in Settings. Booleans use `toggle-setting`; richer types get a `nativeAction` plus renderer handling.
* Keyboard accelerators use canonical storage/registration form (`Command+Alt+K`) and symbol display form (`⌘⌥K`). User-visible labels must pass through `shortcutLabel` (`src/ui.tsx`) or `formatShortcut` (`src/electron/main.ts`); `Space` remains literal.

## Code, docs, and style

* Comments are a smell; prefer several well-named functions over explanatory comments.
* Keep `AGENTS.md` minimal: only durable guidelines/instructions that apply repo-wide. Move overflow to `src/docs/` when needed.
* Document hard-won learnings in `src/docs/` only after substantial iteration. Docs should capture intention/guidelines, not implementation details.
* Use design tokens from `:root` in `src/styles.css` (`--radius-*`, `--surface-*`, `--border-*`, `--text-*`, `--accent-*`, `--danger-*`). Do not add ad-hoc `rgba(255,255,255,…)` surfaces/borders/text or pixel radii; extend tokens if needed.

## Verification

* Before committing palette or extension API migrations, run tests and verify representative flows.
* For root/search bugs, follow `src/docs/palette-debug.md`; use `mise exec pnpm -- pnpm palette:debug --query ...` and trace provider output, caches, ranking, limits, renderer refresh, and final rendered items before patching.
* For file/metadata/thumbnail extension API changes, verify an installed extension with `mise exec pnpm -- pnpm palette:debug --query ... --execute ...`; keep OS/tool calls batched or bounded.
* Verify open view, primary action, repeated shortcuts/navigation, back stack, action-panel action, selection, icons/media, and dismissal behavior.

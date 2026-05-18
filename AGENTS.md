# Agent Guidelines

## Workflow

* Always use `mise exec pnpm` for any package manager commands. Never use npm, yarn, or bun directly — the project pins Node v22 and pnpm v10 via mise.toml.
* When changing/adding code, always explore the repo to understand conventions and similar use cases before assuming the intended architecture. Reuse existing helpers, overlays, and primitives before adding new ones — formatters, confirm-style flows, shortcut recorder plumbing, etc. already exist.
* If you suddenly see changes you have not done, it might be the user in the background, do not mess it up.
* When debugging generated/plugin behavior, inspect both the framework API and the installed/generated artifacts that consume it (for Nevermind extensions, check `~/Library/Application Support/nvm/extensions` as well as repo code). Extension command/action boundaries must catch, normalize, and clone-check results so extension failures surface as Nevermind error views, not raw Electron IPC/log errors.

## Code and docs

* Try to keep files small and focused.
* New primitives, especially extension API additions, must be generic first-party building blocks rather than helpers tailored to one generated extension or user prompt.
* Treat extensions as first-class app contributors: users should be able to build workflows with extension APIs that feel as capable as native Nevermind features, while the host keeps ownership of safety, rendering, ranking, and core state.
* Performance is a top priority. If you change/add code make sure it's performant. If you see existing slow/bab patterns change them for performance improvments.
* Comments are a smell. 3 long named functions is better than 1 function with a comment.
* If you spend a lot of iterations with the user to finally find a solution for something, document your learnings in src/docs/. But you shouldn't touch it if all goes smooth.
* Documentation needs to hold intention, never implementation (except general quirks on the point above). Implementation details are already on files, you're not adding value to it. Document intention and guidelines.
* Style with the design tokens defined in `:root` of `src/styles.css` (`--radius-*`, `--surface-*`, `--border-*`, `--text-*`, `--accent-*`, `--danger-*`). Do not reintroduce ad-hoc `rgba(255,255,255,…)` surfaces/borders/text or pixel border-radii — extend the token set if you need a new step.

## Product conventions

* This app is command-k first. Decisions, confirmations, warnings, and configuration live inside the palette — usually a `preview`/`list` view with an actionPanel, or an item in an existing view. Never use `window.confirm`, `window.prompt`, or `window.alert`, and never introduce a separate preferences window. Extension `requiresConfirmation` and destructive flows must render as in-palette confirmation/action-panel states.
* Extension APIs should be declarative contribution points, not backdoors into app internals. Prefer host-controlled surfaces such as root items, views, actions, background refresh, permissions, quotas, TTLs, and bounded ranking over exposing mutable global state.
* Async UX should expose stable UI immediately: render the user’s intended surface or a cached snapshot first, show loading/progress in place, and refresh when work settles. Keep visible passive surfaces snapshot-stable; never insert/reorder items into an already-rendered list unless the user explicitly refreshes or navigates.
* AI builder chats are history/context, not owners of extensions. Extension files are standalone durable artifacts, and deleting chat history must not delete extension code.
* Reset scoped filters/search state when navigating into a different context unless inheritance is explicitly desired. Keep extension view presentation state scoped: action panels/submenus should own palette sizing and selection state while open.
* Do not show empty-state UI in passive content surfaces; reserve empty states for places where the user expects result/action lists.
* New user-configurable options go through the existing settings pipeline: add an entry to `SETTING_DEFINITIONS` in `src/electron/main.ts`, persist it under `userState.settings`, and surface it as an item in the Settings view. Booleans toggle via `toggle-setting`; richer types get their own `nativeAction` kind plus renderer handling.
* Keyboard accelerators have two forms: the canonical `Command+Alt+K` for storage/registration, and the symbol form `⌘⌥K` for display. Anything user-visible must pass through `shortcutLabel` (renderer, `src/ui.tsx`) or `formatShortcut` (main, `src/electron/main.ts`). `Space` intentionally stays as the literal word.

# Agent Guidelines

* Always use `mise exec pnpm` for any package manager commands. Never use npm, yarn, or bun directly — the project pins Node v22 and pnpm v10 via mise.toml.
* Try to keep files small and focused.
* When changing/adding code, always explore the repo to understand conventions and similar use cases before assuming the intended architecture. Reuse existing helpers, overlays, and primitives before adding new ones — formatters, confirm-style flows, shortcut recorder plumbing, etc. already exist.
* Comments are a smell. 3 long named functions is better than 1 function with a comment.
* If you suddenly see changes you have not done, it might be the user in the background, do not mess it up.
* If you spend a lot of iterations with the user to finally find a solution for something, document your learnings in src/docs/. But you shouldn't touch it if all goes smooth.
* Documentation needs to hold intention, never implementation (except general quirks on the point above). Implementation details are already on files, you're not adding value to it. Document intention and guidelines.
* This app is command-k first. Decisions, confirmations, warnings, and configuration live inside the palette — usually a `preview`/`list` view with an actionPanel, or an item in an existing view. Never use `window.confirm`, `window.prompt`, or `window.alert`, and never introduce a separate preferences window.
* When debugging generated/plugin behavior, inspect both the framework API and the installed/generated artifacts that consume it (for Nevermind extensions, check `~/Library/Application Support/nvm/extensions` as well as repo code).
* Reset scoped filters/search state when navigating into a different context unless inheritance is explicitly desired.
* Do not show empty-state UI in passive content surfaces; reserve empty states for places where the user expects result/action lists.
* New user-configurable options go through the existing settings pipeline: add an entry to `SETTING_DEFINITIONS` in `src/electron/main.cjs`, persist it under `userState.settings`, and surface it as an item in the Settings view. Booleans toggle via `toggle-setting`; richer types get their own `nativeAction` kind plus renderer handling.
* Keyboard accelerators have two forms: the canonical `Command+Alt+K` for storage/registration, and the symbol form `⌘⌥K` for display. Anything user-visible must pass through `shortcutLabel` (renderer, `src/ui.tsx`) or `formatShortcut` (main, `src/electron/main.cjs`). `Space` intentionally stays as the literal word.

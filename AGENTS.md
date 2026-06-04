# Agent Guidelines

## Workflow

* Always use `mise exec pnpm` for all package management. Do not use npm, yarn, or bun directly.
* Leverage existing primitives, formatters, and OS capabilities instead of creating one-off helpers.
* Respect manual file changes and stop the app before editing `userState` to prevent overwrites.
* Commit frequently and use history to guide changes.
* Do not fix symptoms, fix diseases. After two successive symptom patches in the same code path, stop patching and audit the architecture.

## Architecture

* **Dogfood the Extension API as the app's primary development model. Treat extension APIs as declarative host points, not backdoors; fix missing primitives in the API instead of bypassing it with bespoke native code, and keep commands and provider items distinct to avoid duplication in search. When adding background or automation primitives, migrate an existing Nevermind host job first, then expose the extension API.**
* Keep files small and focused. Refactor slow patterns when encountered.
* Fix shared models and lifecycles at the source rather than patching UI behavior.
* Use intent-named capabilities in `src/electron/os.ts`. Follow `src/docs/os-architecture.md`.
* Maintain native behavioral contracts (shortcuts, icons, async lifecycle) when migrating features to extensions.
* Normalize action results so failures appear as UI error views rather than raw logs.
* Keep every action/search/view payload that crosses Electron IPC `structuredClone`-safe; strip handlers/functions after registering them and add clone-safety checks for new payload shapes.
* AI-generated code is durable. AI writes are restricted to extension files relevant to the current chat context.
* When fixing bugs, evaluate how the system would look if built from scratch and propose improvements.
* For services sitting between a client SDK and an upstream provider, default to transparent reverse-proxy (auth swap + verbatim forward + usage sniff) over message translation — translators lose provider-specific fields (auth signatures, cache markers, tool metadata) and force per-feature patches.

## Product and UX

* Use the Command-K palette for all interactions. Avoid `window.confirm`, `alert`, or dedicated settings windows.
* Show cached snapshots immediately and refresh in place. Do not disrupt navigation history with passive loading states.
* Reset filters and search during navigation unless inheritance is explicit.
* Reserve empty states for active result lists; do not show them in passive surfaces.
* Use the standard pipeline in `src/electron/main.ts` for new settings.
* Use canonical storage (`Command+Alt+K`) and symbol display (`⌘⌥K`) formats via `shortcutLabel`.

## Style and Documentation

* Prefer descriptive function naming over comments.
* Prefer named functions over anonymous arrow functions.
* Keep `AGENTS.md` concise. Move detailed guides to `src/docs/`. Document intention, NEVER implementation.
* Before debugging recurring architecture issues, search `src/docs/solutions/` for prior root causes and verification notes.
* Treat `src/resources/nevermind-extension-api.d.ts` as the self-documenting source of truth for the app's extension platform: runtime contract, validation types, author guidance, and AI builder context. Update its types/TSDoc with every API change and do not recreate API reference markdown.
* Every extension API endpoint that renders UI must have a dev-only fixture under `src/fixtures/`; use `.agents/skills/extensions-ui-test/SKILL.md` for dogfooding and see `src/docs/extension-api-ui-fixtures.md`.
* Use CSS variables from `src/styles.css` (e.g., `--accent-*`, `--radius-*`). Avoid hardcoded colors or radii.

## Verification

* Run tests and verify core flows before committing API or palette changes.
* Ensure new dependencies resolve correctly within the Electron `app.asar`.
* Use `mise exec pnpm -- pnpm palette:debug` to trace provider output and ranking.
* Verify navigation, action panels, shortcuts, and dismissal behavior for all changes.

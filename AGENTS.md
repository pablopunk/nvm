# Agent Guidelines

## Workflow

* **Tooling**: Always use `mise exec pnpm` for all package management. Do not use npm, yarn, or bun directly.
* **Reuse**: Leverage existing primitives, formatters, and OS capabilities instead of creating one-off helpers.
* **Safety**: Respect manual file changes and stop the app before editing `userState` to prevent overwrites.
* **Git**: Commit frequently and use history to guide changes.

## Architecture

* **Efficiency**: Keep files small and focused. Refactor slow patterns when encountered.
* **Primitives**: Fix shared models and lifecycles at the source rather than patching UI behavior.
* **OS Interop**: Use intent-named capabilities in `src/electron/os.ts`. Follow `src/docs/os-architecture.md`.
* **Extensions**: Treat Extension APIs as declarative host points, not backdoors. Ensure commands and provider items remain distinct to avoid duplication in search.
* **Consistency**: Maintain native behavioral contracts (shortcuts, icons, async lifecycle) when migrating features to extensions.
* **Error Handling**: Normalize action results so failures appear as UI error views rather than raw logs.
* **AI Scope**: AI-generated code is durable. AI writes are restricted to extension files relevant to the current chat context.
* **Innovation**: When fixing bugs, evaluate how the system would look if built from scratch and propose improvements.

## Product and UX

* **Palette First**: Use the Command-K palette for all interactions. Avoid `window.confirm`, `alert`, or dedicated settings windows.
* **Async UI**: Show cached snapshots immediately and refresh in place. Do not disrupt navigation history with passive loading states.
* **Context**: Reset filters and search during navigation unless inheritance is explicit.
* **Cleanliness**: Reserve empty states for active result lists; do not show them in passive surfaces.
* **Settings**: Use the standard pipeline in `src/electron/main.ts` for new settings.
* **Shortcuts**: Use canonical storage (`Command+Alt+K`) and symbol display (`⌘⌥K`) formats via `shortcutLabel`.

## Style and Documentation

* **Code**: Prefer descriptive function naming over comments.
* **Docs**: Keep `AGENTS.md` concise. Move detailed guides to `src/docs/`. Document intention, NEVER implementation.
* **Extension API Docs**: Keep extension-author documentation in `src/resources/nevermind-extension-api.d.ts` using TSDoc. Do not recreate API reference markdown; `read_extension_api` and extension validation both use the typed declaration file as the source of truth.
* **Design**: Use CSS variables from `src/styles.css` (e.g., `--accent-*`, `--radius-*`). Avoid hardcoded colors or radii.

## Verification

* **Testing**: Run tests and verify core flows before committing API or palette changes.
* **Bundling**: Ensure new dependencies resolve correctly within the Electron `app.asar`.
* **Debugging**: Use `mise exec pnpm -- pnpm palette:debug` to trace provider output and ranking.
* **UX Audit**: Verify navigation, action panels, shortcuts, and dismissal behavior for all changes.

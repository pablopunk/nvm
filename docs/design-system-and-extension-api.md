# Design System and Extension API Direction

Nevermind should feel like one product whether a feature is built into the app or provided by an extension. The extension API should expose Nevermind's product primitives, not a parallel UI system.

## Intent

Extensions declare what they want to show or do. Nevermind owns how it looks, how keyboard navigation works, how actions are presented, how shortcuts are displayed, how navigation stacks behave, and how platform integrations are performed.

The desired outcome is that an extension-built workflow feels indistinguishable from a native Nevermind workflow.

## Principles

- **One design system.** Built-in features and extensions use the same UI primitives.
- **Declarative extension UI.** Extensions describe views, items, actions, and capabilities; they do not invent bespoke rendering.
- **Host-owned interaction.** Selection, filtering, Enter behavior, Cmd+K/action panels, shortcuts, nested navigation, empty states, loading states, and errors are owned by Nevermind.
- **Typed built-in actions.** Common operations such as open, copy, reveal, quick look, open with, navigate, and run should be first-class actions with consistent titles, icons, descriptions, and behavior.
- **Powerful capabilities, clear boundaries.** Extensions should be able to automate the system, run scripts, inspect files, use storage, and call platform helpers, but those capabilities should be explicit and eventually governed by permissions and confirmations.
- **Backward compatibility.** Existing generated extensions should keep working while the API evolves toward the shared model.

## First-party extension dogfooding

Nevermind should be developed as if many native features could be extensions. This keeps pressure on the extension API to support real product workflows instead of becoming a demo-only abstraction.

Native features do not need to call the exact public `ctx.*` API internally, but they should increasingly produce the same underlying view, item, and action model that extensions produce. The public API can remain a safer authoring layer while first-party code uses the shared model directly when that is clearer.

Good first-party extension candidates include clipboard history, keyboard shortcut management, simple system commands, calculator results, and file/app result actions. Clipboard history is the best first candidate because it exercises list UX, text and image content, copy/paste actions, empty states, filtering, shortcuts, and native-feeling action panels.

The goal is not to move everything into extensions for its own sake. The goal is to make sure native features and extensions continuously shape the same design system.

Dogfooding only counts when first-party extension-style features use the same visual shell and row/action components as the native feature they replace. Recreating a native workflow with extension-specific wrappers, padding, shortcut slots, or item rendering is a failure of the abstraction, even if the data model is shared.

## Extension API shape

The API should expose product concepts:

- views such as list, grid, detail, chat, form, and progress
- items with titles, subtitles, media, accessories, keywords, and actions
- action panels with sections and nested choices
- built-in actions for common platform behavior
- capability namespaces for files, apps, clipboard, shell, storage, and AI

The API should avoid exposing renderer internals. Documentation should explain what extensions can express and what Nevermind guarantees, not where or how the renderer implements it.

## UX guarantees

For any view or action created by an extension, Nevermind should provide:

- consistent visual styling
- keyboard-first navigation
- visible shortcut hints
- sensible default Enter behavior
- Cmd+K/action panel access
- nested navigation with back behavior
- filtering/search behavior where appropriate
- native-feeling empty, loading, error, and progress states
- safe rendering of media and file previews

## Capability direction

Extensions should be useful beyond static UI. They should be able to perform local automation through deliberate capability APIs, including file access, app launching, clipboard operations, persistent storage, shell/script execution, and AI-assisted workflows.

Risky capabilities should move toward explicit permission declarations, timeouts, output limits, and confirmation flows for destructive operations.

## Documentation boundaries

- `docs/extension-api.md` describes the current public API surface for extension authors and AI builders.
- This document describes the product and architecture direction.
- Implementation details belong in source code, not here.
- If the implementation changes but the product intent remains the same, this document should not need to change.

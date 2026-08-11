# Durable Root Action Customization

## Problem or Symptoms

An extension capability was represented by a `Dictate` root item and a separate `Dictation Settings` command. After consolidating it into one root item, the Cmd-K panel showed duplicate `Dictate` actions and did not expose alias or shortcut options.

## Context

The search host selects `rootItems()` for an empty query and `searchItems()` for a typed query in `src/electron/search-snapshot.ts`. Root provider results become `extension-root-item` actions in `src/electron/main.ts`; durable `actions(ctx)` contributions become `extension-action` records with aliases, shortcuts, and customization metadata.

## What Did Not Work

- Using `ctx.actions.background(...)` directly as a root item's primary action created a transient handler with no durable customization record.
- Adding the same primary action to `actionPanel` rendered it twice because the host also treats the root primary action as the default action.
- Defining only `rootItems()` made the item disappear when the user typed a query; typed search requires `searchItems()`.
- `ExtensionActionContribution.placement` existed in the API types but was not enforced by host search filtering, so root-only durable actions could become duplicate standalone results.

## Root Cause

The extension API had separate concepts for dynamic root items and durable actions, but root-item normalization did not connect a `ctx.actions.ref(...)` primary action to its registered `extension-action` record. As a result, the renderer could not discover the action's canonical Options surface for aliases and shortcuts.

## Fix

- Register the primary capability action through `actions(ctx)` with `placement: ['root']`.
- Reference that contribution from the root item's `primaryAction` with `ctx.actions.ref(...)`.
- Derive the referenced durable action as the root item's host-owned `persistentAction` so the host can render canonical alias and shortcut options without trusting extension-supplied executable payloads.
- Filter durable action search results by `placement` and remove the primary action from root action panels by identity or registered-action reference.
- Keep Settings and other secondary actions in the root item's `actionPanel`; independently searchable secondary actions belong in `actions(ctx)` and can be referenced with `ctx.actions.ref(...)`.

## Verification

The Dictation extension tests cover one root item, a durable root action, root-only placement, typed-query discovery, and a Settings-only action panel. `mise exec -- pnpm test` passes all 258 tests, `mise exec -- pnpm typecheck`, source Biome checks, `git diff --check`, and `mise exec -- pnpm build` pass, and `mise exec -- pnpm palette:debug --no-build --query dictate` reports one Dictation root item with `persistentAction` aliases/shortcut/customizable metadata and only Settings in its action panel. The full `pnpm check` command still reports formatting for ignored generated `dist/main/main.js` after a build.

## Notes for Future Searches

Search for `extensionRootActionFromItem`, `persistentActionForRef`, `searchProviderDescriptors`, `placement: ['root']`, `ctx.actions.ref`, `extension-root-item`, and `extension-action` before changing root-item action behavior.

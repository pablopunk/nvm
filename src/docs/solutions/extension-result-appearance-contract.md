# Extension Result Appearance Contract

## Problem or symptoms

A generated extension set `appearance: { foreground: 'red' }` on a `rootItems()` result, but searching for the same action still showed a normal, uncolored result.

In the concrete case, the local `Quit All Apps` extension rendered red only when its `rootItems()` contribution appeared. Searching `quit` or `quit all apps` showed the durable `actions()` contribution instead, which did not carry `appearance` through the command/action search pipeline.

## Context

Nevermind has several extension surfaces that can become palette results:

- `rootItems(ctx)` and `searchItems(ctx, query)` return `ExtensionItem` objects.
- `actions(ctx)` returns durable `ExtensionActionContribution` objects.
- `commands` return `ExtensionCommand` objects that are normalized into durable actions.

Users and extension builders think of all of these as “results” once they appear in the palette, so visual affordances such as `appearance.foreground` should behave consistently across them.

## What did not work

Only adding a red `rootItems()` item was a symptom patch. It did not guarantee that search would show the red result, because the ranked durable action could outrank or replace the provider item.

Adding a duplicate `searchItems()` item can make the immediate extension look right, but it risks duplicating commands/provider items and fights the project rule to keep commands and provider items distinct.

## Root cause

The public/rendering contract was split: `ExtensionItem` supported `appearance`, but durable `ExtensionActionContribution` and `ExtensionCommand` did not. The runtime normalization path for commands also did not propagate command appearance into the registered action item.

This made `appearance.foreground` depend on which provider/search path produced the visible result, not on the semantic result itself.

## Fix

Fix the shared result primitive instead of relying on duplicate provider items:

- Add `appearance?: ExtensionItemAppearance` to `ExtensionActionContribution` and `ExtensionCommand` in `src/resources/nevermind-extension-api.d.ts`.
- Propagate command `appearance` during command registration in `src/electron/main.ts`.
- Keep renderer styling centralized in `src/styles.css` using the existing `data-foreground` attribute and `--item-foreground-*` tokens.
- Update AI builder capability guidance so generated extensions know `appearance.foreground` applies to item/action/command titles and Lucide/fallback icons.
- Use fixtures or a local generated extension to cover both provider items and durable action/command results.

## Verification

Useful checks:

```bash
mise exec -- pnpm typecheck
```

With the dev app running and CDP on port 9222, inspect the actual search payload rather than assuming which result is visible:

```bash
agent-browser --cdp 9222 eval "window.nvm.search('quit').then(r => r.map(x => ({ title: x.title, kind: x.kind, icon: x.icon, appearance: x.appearance })))"
```

The visible durable action or command result should include:

```json
{ "appearance": { "foreground": "red" } }
```

## Notes for future searches

Keywords: extension API appearance, foreground color, red result, result icon color, durable actions, commands, rootItems, searchItems, ExtensionActionContribution, ExtensionCommand, normalizeItemAppearance, command registration, result-shaped surface.

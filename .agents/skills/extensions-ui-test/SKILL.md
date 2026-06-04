---
name: extensions-ui-test
description: Use when testing, reviewing, or polishing Nevermind extension API UI fixtures, host-rendered extension views, or any UI-facing extension API endpoint. Trigger on requests mentioning extension fixtures, Fixtures root item, extension UI tests, dev-only UI fixtures, or visual polish of generated/extension views.
---

# Extensions UI Test

Use this skill to dogfood Nevermind's host-rendered extension UI contract. The goal is to make every UI-facing extension API primitive feel polished, keyboard-first, and consistent with the rest of the app before AI-generated extensions depend on it.

## Source of truth

- API contract: `src/resources/nevermind-extension-api.d.ts`
- Fixture rule: `src/docs/extension-api-ui-fixtures.md`
- Fixture extensions: `src/fixtures/`
- Current fixture entrypoint: search for `Fixtures` in the dev palette
- Renderer/model: `src/extension-view.tsx`, `src/ui.tsx`, `src/model.ts`, `src/styles.css`
- Runtime fixture loader: `src/electron/main.ts`
- General UI review skill: `.agents/skills/ui-design/`

## Non-negotiable rule

Every extension API method that renders host-owned UI must have a dev-only fixture under `src/fixtures/`. If a UI API changes, update the fixture in the same change. A UI API is not done until its fixture has been manually dogfooded.

## Start the app

Use the repo package-management rule:

```bash
mise exec pnpm -- pnpm dev
```

Run this as a background process in agent harnesses. Do not use `npm`, `yarn`, or `bun` directly.

The dev script starts Electron with a Chrome DevTools Protocol endpoint on port `9222` by default for automation. Override with `NVM_DEV_REMOTE_DEBUGGING_PORT` only if the port is busy.

Then open the palette and search `Fixtures`. There should be one visible fixture entrypoint named `Fixtures` (other normal fallback/root items may still rank below it). Opening it should show fixture commands directly, grouped by fixture extension, without an extra nesting level.

## Preferred automation

Use `agent-browser` for Electron UI automation when available.

1. Try to load agent-browser's Electron workflow before using it:
   ```bash
   agent-browser skills get electron
   agent-browser skills get dogfood
   ```
   Some installed versions do not support `skills`; if it prints `Unknown command: skills`, fall back to `agent-browser --help` and continue.
2. Connect to the running Electron app:
   ```bash
   agent-browser --cdp 9222 snapshot -i -c
   ```
3. Use the preload API for deterministic checks when UI typing is flaky. Reset to root before searching; otherwise typing inside a fixture filters the child view instead of root search:
   ```bash
   agent-browser --cdp 9222 eval "window.nvm.hide(); window.nvm.shortcutReady()"
   agent-browser --cdp 9222 eval "window.nvm.search('fixtures').then(r => r.map(x => x.title))"
   ```
4. Capture accessibility snapshots and screenshots for each fixture command:
   ```bash
   agent-browser --cdp 9222 screenshot /tmp/nvm-fixture.png
   ```
5. Record issues with reproduction steps and screenshots.

If agent-browser is unavailable, still perform a manual dogfood pass and use screenshots from the OS/browser automation tools available in the environment.

## Fixture traversal checklist

From the `Fixtures` root item, open every command in every fixture extension.

For each command, verify:

- It opens without throwing or showing raw logs.
- It is reachable by keyboard only.
- `Enter` runs the primary action.
- `Cmd+K` opens useful actions when actions exist.
- Back navigation returns to the fixture list predictably.
- The search input/filtering behavior is appropriate for the view.
- Empty/loading/error states are intentional and not shown on passive surfaces.
- Text, icons, accessories, and action hints are aligned and clipped correctly.
- The surface uses existing design tokens and matches Nevermind density.
- Focus, hover, selected, disabled, and destructive states are visible but not noisy.
- The UI works in default, stacked, and preview/large palette modes when applicable.

## Current fixture surface matrix

`src/fixtures/ui-fixtures.ts` should exercise at least:

- `ctx.ui.list`
  - Sections, rows, accessories, icons, action hints, action panels, confirmation, navigation.
- `ctx.ui.grid`
  - Images, aspect ratio, columns, title/subtitle clipping, action hints, selected tile state.
- `ctx.ui.preview`
  - Markdown/text/media rendering, action panels, scroll behavior.
- `ctx.ui.chat`
  - User/assistant/system messages, markdown content, scrolling, busy state if available.
- `ctx.ui.form`
  - Text, textarea, password, URL, number, date, checkbox, dropdown/select, multiselect, description, separator, field descriptions, field errors, submit action, submitted values.
- `ctx.ui.progress`
  - Step statuses, loading density, long titles.
- `ctx.ui.webview`
  - Sandbox presentation, focusability, sizing, action panel coexistence.
- `ctx.ui.camera`
  - Permission states, loading, live state, unavailable/error state, device switcher, controls.
- `ctx.ui.confirm`
  - Confirmation copy, destructive/regular state, keyboard path, cancellation.
- `ctx.ui.toast`
  - Placement, timing, default/error tone, not hiding important state.

When adding a new UI method, add it to this matrix and to the fixture.

## Visual review heuristics

Apply the `ui-design` skill checklist, especially:

- Compact palette density; avoid marketing-page whitespace.
- Grayscale hierarchy first; use color only as supporting signal.
- Use `src/styles.css` variables, not hardcoded colors/radii.
- Native desktop feel: no pointer cursor on normal buttons, no webby focus rings on text inputs.
- One clear primary action per surface.
- Keyboard-first interactions with visible shortcut/action affordances.
- No raw JSON, stack traces, unrendered markdown, broken media, or unstyled browser defaults unless the fixture intentionally demonstrates an error state.

## Common failure patterns to look for

- Fixture commands appear individually in root search instead of only through the single `Fixtures` entrypoint.
- Opening `Fixtures` adds pointless nesting before commands are visible.
- Form checkboxes inherit card/input styling and look misaligned.
- Select/multiselect controls use unpolished browser defaults or awkward heights.
- Field descriptions/errors fight the label hierarchy.
- Preview content renders as raw monospace text when markdown was expected.
- Grid tiles crop awkwardly or action hints overlap media/title.
- Action panels open empty or duplicate the primary action without value.
- Loading states replace cached content unnecessarily.
- Camera/webview surfaces ignore action panel space or overflow the card.

## Fix workflow

1. Reproduce through the `Fixtures` root item.
2. Capture the smallest failing fixture command and screenshot/snapshot evidence.
3. Identify whether the issue belongs in:
   - public contract: `src/resources/nevermind-extension-api.d.ts`
   - model: `src/model.ts`
   - renderer: `src/extension-view.tsx` or `src/ui.tsx`
   - styles: `src/styles.css`
   - runtime normalization/loading: `src/electron/main.ts`
   - fixture coverage: `src/fixtures/`
4. Fix the shared primitive, not the fixture data, unless the fixture is invalid.
5. Update the fixture to cover the regression.
6. Run:
   ```bash
   mise exec pnpm -- pnpm test
   ```
7. Dogfood the affected fixture again.

## Done criteria

A UI fixture polish task is done only when:

- The fixture is reachable through the single `Fixtures` root item.
- All affected commands were manually opened.
- Keyboard paths were verified.
- Screenshots/snapshots show the fixed states.
- `mise exec pnpm -- pnpm test` passes.
- Any new UI API method is documented in `src/resources/nevermind-extension-api.d.ts` and represented in `src/fixtures/`.

# Palette-first extension UI

Nevermind's palette is the canonical interaction surface, not a launcher that
opens conventional forms. Short input, choices, confirmation, and follow-up
actions should preserve the palette's single focus target and keyboard model.

## Rules

- Collect short scalar values with `ctx.input.prompt`. The host presents each
  field through the palette input and result rows rather than stacked controls.
- Represent choices and confirmations as result rows. `Enter` accepts the
  selected row, filtering stays in the palette input, and `Escape` goes back.
- Use `ctx.ui.editor` for long-form text or Markdown. Note-like documents should
  use `format: 'markdown'` for in-place rich Markdown rendering and
  `titleFromContent` instead of adding a separate title or rename workflow.
- Use native pickers for files, folders, permissions, and other OS-owned
  resources; return to the palette after the native interaction completes.
- Keep one primary action per step. Secondary actions belong in the action
  panel with visible shortcuts.
- Cmd+K action menus and nested submenus appear in a compact bottom-right
  overlay on every palette and independent window, preserving the current
  results, query, and selection. `actionPanelPresentation` is retained only for
  source compatibility and is ignored. Confirmations and prompts expand to the
  full keyboard-first surface after their action is chosen.
- List or choice results launched inside those windows may use
  `windowPresentation: 'compact'` to reuse the overlay without replacing the
  base editor; normal palette launches still render the list at full size.

## Exceptions

A dense dashboard, live media surface, or rich document may need another
host-owned view, but its commands, navigation, and follow-up choices still
belong in the palette. Custom webviews are a last resort, not a way to recreate
forms that the palette primitives already cover.

Legacy `ctx.ui.form` views remain supported for compatibility with genuinely
structured workflows. Do not use them for lightweight arguments that fit
`ctx.input.prompt`, an editor, result-row choices, or a native picker.

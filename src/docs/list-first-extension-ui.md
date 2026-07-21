# List-first extension UI

Use a `ctx.ui.list` or `ctx.ui.grid` for anything with selectable rows. The host
owns focus, filtering, keyboard navigation, and action panels; extension views
should use those primitives instead of reproducing them with checkboxes, button
grids, or custom controls.

## Rules

- A multi-item choice must be a list. `Enter` on a row performs its primary
  action, and `Cmd+K` exposes the same action plus any secondary actions.
- Keep the next consequential action as a normal, focusable list row (for
  example, **Review selected items**) so it is reachable by keyboard.
- Show selection state with row accessories such as `Selected`/`Optional`; use
  a `ctx.actions.run` handler and return an item patch to update only that row.
- Use stable IDs. The host preserves focus through patches, filters, and
  navigation.
- Include an intentional empty state for active result lists and make errors
  actionable. Do not use empty-state UI for passive notes or decoration.

## Allowed exceptions

- Short scalar input and booleans follow the sequential palette-input pattern
  in [Palette-first extension UI](./palette-first-extension-ui.md).
- `file`, `files`, and `folder` fields invoke the native OS picker. They are
  valid for choosing paths, but are not a substitute for a palette list of
  already-known candidates.
- A genuinely minor, non-selectable status or description may remain plain
  text. It must not look or behave like an interactive row.

## Selection pattern

The built-in Files extension is the baseline for file-result rows: stable file
identity, title/path subtitle, and actions in the palette action panel. For
multi-select flows, keep the selected IDs in the view action closure, render
each candidate as a list row, and return a patch that replaces that candidate
after its selection action runs. Convert the set into the service's explicit
selection payload only at the review/confirm step; this preserves the service's
snapshot validation for destructive work.

The uninstall candidate chooser and the **Dev UI · File Selection List**
fixture are reference implementations. Exercise the fixture with the keyboard
after changing this pattern.

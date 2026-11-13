# WP3.3 review sheet: declarative views drawn with palette widgets

Goldens are checked in under `src/native/internal/app/testdata/golden/views/` (56 PNGs: 14 views, each light and dark
at 1x and 2x). They are rendered by the L2 tests in `internal/app` (`extension_views_test.go` and
`extension_views_states_test.go`) through the real child process: each view comes from the dev fixture extension
`acme.views` (`internal/ext/testdata/extensions/acme.views`), runs through the manager, and is drawn by
`internal/palette/extview`.

## Goldens

| Golden | Covers |
| --- | --- |
| `list-detail` | List with the first row selected: primary action in the footer, accessory `Draft +1`, subtitle, icon tile, and the side detail pane with title, subtitle, Markdown body and metadata (text, separator, link). |
| `sections` | Two sections (`Open`, `Done`) with their headers over the first row of each run. |
| `grid` | Three columns: glyph tiles (wave selected) and an icon tile with its title under it. |
| `collection` | Host CRUD list: the add row first, two records, the second selected with Preview as its primary action. |
| `preview` | Read-only Markdown page (heading, list, fenced code). |
| `progress` | Step progress: label, `1 of 3 · 33%`, and the done, active and pending markers. |
| `file-preview` | File preview of `internal/app/testdata/views-fixtures/sample.txt`: the leading text in a fenced block, read by the host. |
| `source-list` | Data source loaded, immediate refresh patched a row (`Refreshed just now`), search accessory `All ⌄`, and the `Load more` row. |
| `loading` | A data source still loading: no rows and no empty state, only the search field. |
| `empty` | Empty list with the view's own empty title and subtitle. |
| `failure` | A failed load: the contextual reason (`Could not load these items` with the extension's message) and `Try again`. |
| `confirmation` | Destructive action asks first: `Delete` (red, Enter) and `Cancel`, under the question `Delete “Alpha note”?`. |
| `selected` | Selected item id honoured: the second row chosen, a disabled row muted with no actions, and a contextual notice. |
| `images` | Image row (decoded from a data URL) and the detail pane with the image and text. |

`rocket` is now in the Lucide set (`design/lucide_extra.go`); the grid fixture uses it.

## Checklist (PLAN.md section 2)

Ticked by the author from the goldens above, on Linux (the box), not on macOS or Windows.

- [x] Tokens only: new colours come from `design.Tokens`; no literal colours in `internal/palette/extview`.
- [x] Type scale: titles 13, subtitles, metadata and steps 11; contextual titles 13, reasons 11.
- [x] Icons: Lucide names through `design.Tile`, plus the emoji glyphs the extension sends.
- [x] Selection: the selected row, tile and grid cell use the palette's own plate (list-detail, grid, selected).
- [x] Disabled rows: muted with no plate and no actions (selected).
- [ ] Hover differs from selection; focus ring for keyboard focus only. Not captured.
- [ ] Contrast WCAG AA for the new text pairs. Uses existing token pairs; not computed per pair.
- [ ] Density: long lists scroll with hidden scrollbars. Not captured with more rows than fit.
- [ ] RTL and 200% text smoke test for these views. Not captured.
- [x] Motion: none added.

## Electron parity (per view type)

Reference: `1.x/palette/extension-view/*` and `1.x/electron/extension-host/extension-ui-api.ts` (behaviour
read in the WP3.3 review). "Same" means the same rows, actions and shortcuts for the same extension input.

| View | Same | Differences |
| --- | --- | --- |
| List | Rows, sections, empty state (`No items` default), loading shows no empty state, Enter runs the primary action, Cmd+K opens the action menu, patches (patch, replace, prepend, append, remove, unknown ids dropped) | Only the first accessory shows, with `+N` for the rest; accessory tones are not drawn (rows keep the extension purple); `selectedItemId` is honoured on the first show of the rows; `pageSize` is not used (Electron does not use it either); a patch replaces whole items where Electron merges fields |
| Search accessory | Options in the search row, a click cycles, the value reaches `OnChange` | Shown even when there are no items; no keyboard cycling |
| Pagination | `Load more` runs the next page and appends it | A row at the list end, not a button; an empty page ends the list |
| Data source | Loader runs off the UI thread; failure shows the reason; `Try again` when `retry` is set; stale-while-revalidate snapshot | The snapshot is in memory and is lost on restart (Electron persists it) |
| Refresh | Immediate refresh after the first paint; interval refresh; one refresh in flight | Interval runs on the host clock while the view is open |
| Grid | Columns, glyph over image over icon, title under the tile | `layout` (square, wide, compact) and `aspectRatio` are ignored; cells have a fixed height |
| Detail pane | Side placement, title, subtitle, Markdown, text, metadata (text, link, separator), image | Image drawn at 120 px; `tag` and `copyable` metadata drawn as text; `placement: bottom` draws as side (same as Electron); link card not drawn |
| Preview | Markdown, text, subtitle fallback | Images and video not drawn (Electron draws both) |
| Progress | Status aliases, `X of Y · P%`, step markers | No 4 px bar; the percentage is text only |
| Collection | Add row first, Preview primary, Edit, Remove with a confirmation | Shortcuts are Cmd/Ctrl+E and Cmd/Ctrl+Backspace (`CommandOrControl`); Electron's `Command` is Win on Windows and Linux, so Ctrl is used there |
| File preview | Leading 64 KB text in a fenced block, binary note, truncation note | Image files drawn; video not drawn; no thumbnails |
| Shortcuts | Action shortcuts (selected row only, as Electron's `selectedOnly`), item shortcut on the primary action | Global-scope shortcuts (`shortcutScope: global`) are not registered; Command+Enter (editor and form submit) belongs to WP3.4 |
| Navigation | Push, replace, pop, root | `root` returns to the palette's first page and then pushes the view |
| Confirmation | Destructive actions ask first, with a row to confirm or cancel | Confirmation is a palette page (same shape) |
| Images | Data URLs and local files, bounded | Decoded off the UI thread; data URLs over 4 MB and files over 8 MB are not decoded |
| Clone safety | Handlers are tokenized in the main process | Actions are handles issued per session; extension code never receives a palette object |

## Not verified

- macOS and Windows: goldens rendered on Linux in the box only.
- Rig: J15 and J16 on the private display `:82` (see the commit message for the run). J14 (AI builder) and J27 (screen reader pass) are not run.
- Human review: the reviewer must re-check the checklist and the parity table, and record "ok" or the diffs found.

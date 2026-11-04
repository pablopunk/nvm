# WP3.3 review sheet: declarative views drawn with palette widgets

Goldens are checked in under `src/native/internal/app/testdata/golden/views/`. They are rendered by the L2 tests in
`internal/app` (`extension_views_test.go`) through the real child process: each view comes from the dev fixture
extension `acme.views` (`internal/ext/testdata/extensions/acme.views`), runs through the manager, and is drawn by
`internal/palette/extview`. Each view has a light and a dark render at 1x and 2x.

## Goldens

| Golden | Covers |
| --- | --- |
| `list-detail.{light,dark}.{1x,2x}.png` | List with the first row selected: primary action in the footer, accessory `Draft +1` (first accessory plus a count), subtitle, icon tile, and the side detail pane with title, subtitle, Markdown body and metadata (text, separator, link). |
| `sections.{light,dark}.{1x,2x}.png` | Two sections (`Open`, `Done`) with their headers over the first row of each run. |
| `grid.{light,dark}.{1x,2x}.png` | Grid of three columns: glyph tiles (wave selected) and an icon tile with its title under it. The `rocket` icon is not in the Lucide set and falls back to the puzzle icon, as in WP3.2. |
| `collection.{light,dark}.{1x,2x}.png` | Host CRUD list: the reserved add row (`New snippet`, first), two records, the second selected with Preview as its primary action. |
| `preview.{light,dark}.{1x,2x}.png` | Read-only Markdown page (heading, list, fenced code). |
| `progress.{light,dark}.{1x,2x}.png` | Step progress: label, `1 of 3 · 33%` (done steps over the step count), and the done, active and pending markers. |
| `file-preview.{light,dark}.{1x,2x}.png` | File preview of `internal/app/testdata/views-fixtures/sample.txt`: the leading text in a fenced block, read by the host. |

Not golden yet: the empty state of a list, the loading state, a failure toast over a view, the confirmation page
for a destructive action, a patched row (covered by L2 tests), hover and focus rings, RTL and 200% smoke tests.

## Checklist (PLAN.md section 2)

Ticked by the author from the goldens above, on Linux (the box), not on macOS or Windows.

- [x] Tokens only: every new colour comes from `design.Tokens`; no literal colours in `internal/palette/extview`.
- [x] Type scale: titles 13, subtitles, metadata and steps 11. Grid titles 11 under 26 px glyphs.
- [x] Icons: Lucide names through `design.Tile`, plus the emoji glyphs the extension sends.
- [x] Selection: the selected list row, tile and grid cell use the palette's own plate (checked in the list-detail and grid goldens).
- [ ] Hover differs from selection; focus ring for keyboard focus only. Not captured.
- [ ] Contrast WCAG AA for the new text pairs. Uses existing token pairs; not computed per pair.
- [ ] Density: long lists scroll with hidden scrollbars. Not captured with more rows than fit.
- [ ] RTL and 200% text smoke test for these views. Not captured.
- [x] Motion: none added.

## Not verified

- macOS and Windows: goldens rendered on Linux in the box only.
- Rig: J15 and J16 re-run for the change to the root rows (see the PR for the run). J14 (AI builder) and J27 (screen reader pass) are not run.
- Human review: the reviewer must re-check the checklist and record "ok" or the diffs found.

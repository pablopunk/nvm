# internal/design

The design system of the Go app: the tokens every color comes from, the depth rules, the Lucide icons, the
components the palette is built from, and the gallery whose goldens guard all of them.

Layer: `design` (rank 2). It imports only `internal/platform`, the standard library and `github.com/egoist/mygo/ui`.
The `golden` subpackage is test tooling that renders views headlessly with `ui.NewTester`; no production code
imports it. `REVIEW.md` records which checklist items each component meets, with its golden evidence.

## Tokens

`tokens.go` holds the two token sets, ported from `src/app/palette/styles/theme.css`: warm paper and a gold accent
in light, ink blue-grey with the same gold in dark. Read them with:

- `design.Tokens(c)` for the appearance the context renders in (the feature-code path);
- `design.TokensFor(dark)` in tests, to name a set without a render.

Fields are named by role: `Bg`, `Panel`, `Raised`, `Surface`, `Interactive`, `Selected` (surfaces); `Strong`,
`Primary`, `Secondary`, `Tertiary`, `Muted`, `Faint` (text, from most to least emphatic); `Accent`, `AccentFill`,
`AccentText`, `AccentSoft`, `AccentMuted`, `AccentEmphasis`, `AccentContrast` (the gold); `EdgeSubtle`,
`EdgeStrong`, `Hairline`, `Border` (lines); `Hi`, `Lo`, `Wash`, `Shadow` (light and depth); `Danger`; and `Tones`,
the named tones of tinted titles and tiles (yellow, blue, purple, green, red, orange, pink, teal, indigo, gray).

Colors are spelled only here. The hex-literal lint (`hexlint_test.go`) fails on a six-digit hex or a `ui.Hex`,
`ui.RGB` or `ui.RGBA` call in any package outside `design`, and `internal/app` has no exemption.

`contrast.go` and `tokens_contrast_test.go` check WCAG 2.1 AA for every text color on every surface it is drawn
on, in both appearances. The known failures are listed there with their measured ratios; a waiver whose pair starts
passing fails the test, so the list only shrinks.

## Depth

The surfaces are told apart by light, not by heavy outlines:

- flat fills for every surface and control;
- a lit top edge (`Hi`, an inset highlight) on raised things, and a soft shadow below anything that floats;
- grooves for dividers (`Etched`): a dark hairline with a light one under it;
- no gradients, except the gold glossy key of a prominent control (`Glossy`, used by the prominent tile);
- a popover floats with a larger shadow than a control (`Popover`); nothing floats without one.

`SelectedFill` is the selection plate: warm, with a faint gold edge; a prominent selection takes the accent tint.
A plain row under the pointer takes `Wash`, a weaker plate than the selection.

## Type scale

Three sizes carry the palette. Everything else is set against them.

| Role | Size | Weight | Used by |
| --- | --- | --- | --- |
| Search field | 21 | regular | `SearchRow` |
| Row title | 13 | semibold (600) | `Row`, `ActionMenu` titles |
| Subtitle, section header | 11 | regular, section headers semibold | `Row`, `Section`, `Keycap` |

Body text in a detail pane (13), footer text (12), and the empty-state title (15) sit beside them. Titles and
subtitles are single lines with an ellipsis (`SingleLine`); a glyph is never cut in half.

## Spacing and sizes

The target is a 4 px grid, with icon tiles at 32 (`TileSize`), keycaps at 1 by 6 padding, and the pane radius
(`PaneRadius`, 18) shared by the window cut, the border and the clip. The palette does not yet keep every spacing on
the grid: 5, 6, 10 and 14 px paddings remain. They are listed as debt below, not changed, because changing them moves
the palette goldens.

## Components

Each component is one small file, takes plain data (no types from `internal/app`), and reads its tokens from the
context. The doc comment of each says what it draws and when to use it.

| File | Component | Data |
| --- | --- | --- |
| `pane.go` | `Pane` | the window pane, material or fallback |
| `search_row.go` | `SearchRow` | `SearchData`, returns `SearchEvents` |
| `row.go` | `Row` | `RowData` (section header, tile, title, subtitle, accessory, states) |
| `tile.go` | `Tile` | `TileData` (picture, file icon, Lucide icon, letters) |
| `section.go` | `Section` | a group header |
| `keycap.go` | `Keycap` | a key label, already spelled for the platform |
| `chip.go` | `Chip` | `ChipData` (icon, bold word, text) |
| `popover.go` | `Popover`, `Placement`, `ActionPlacement` | a raised surface at a placement |
| `action_menu.go` | `ActionMenu` | `[]MenuRow` |
| `footer.go` | `Footer` | `FooterData` (name, primary and secondary key hints, or a hint) |
| `empty_state.go` | `EmptyState` | `EmptyData` |
| `toast.go` | `Toast` | a message; a thin wrapper of the window's toast |
| `form.go` | `FormField`, `FormError` | `FieldData` |
| `user_bubble.go` | `UserBubble` | a chat message |
| `icons.go`, `lucide.go` | `Icon(name)` | the Lucide table, parsed once |
| `depth.go` | `Etched`, `Glossy`, `SelectedFill` | depth helpers |

`Markdown` replies are drawn by `internal/app/markdown.go`: the layer rules keep the Markdown renderer out of design.

## The gallery

`gallery.go` shows every component in each of its states: `Gallery` is the whole list in one scroll view, and the
goldens render its pages (`galleryRows`, `galleryControls`, `galleryStates`) in the palette's pane. States are fixed
values in `galleryState`, and pictures come from `galleryImage`, so a render never depends on a clock, a file or a
network.

Goldens live under `testdata/golden/design-gallery/`, in light and dark at 1x and 2x:

- `gallery`: the top of the full gallery;
- `rows`, `rows-hover` (the pointer moved onto a row), `rows-rtl` (right to left), `rows-zoom-200` (200 percent text);
- `controls`;
- `states`, `states-material` (the window with its system material).

## Adding a component with a golden

1. Write the component in its own file: a doc comment, plain data in, tokens from `Tokens(c)`, no literal color.
2. Add a section that shows each state to a gallery page (`gallery_rows.go`, `gallery_controls.go` or
   `gallery_states.go`). Use fixed data and the shared pictures.
3. If a state needs the pointer, focus or the material, set `golden.Options.Interact`; for scale or direction, use
   `Tester.SetZoom` or `SetDirection`. Add a `Test...` in `gallery_test.go` that calls `golden.AssertMatrix`.
4. Generate the goldens in the package directory, with the test name as `-run` and `-update` after the package:
   `go test -count=1 -run '^TestGallery' ./internal/design -update`. Review the PNGs before committing them.
5. Record the golden names and the checklist items the component meets in `REVIEW.md`.

Goldens compare with the default tolerance. A test that needs an exact frame sets `Strict: true`; the palette goldens
were confirmed exact this way, and the comparison runs in the stored PNG form so translucent pixels match exactly.

## Debt

Documented, not hidden. Each item is a place where the code does not yet meet the checklist in `PLAN.md` section 2.

- Focus ring: a focused `FormField` takes MyGo's default accent (blue), not the gold token. Moving it means setting the
  window theme's accent, which changes the form goldens; it is deliberately left for its own change.
- Spacing and radii literals in the palette views, listed by file and line below. Each moves to a token or a
  component constant when its view is next changed.
- Contrast: the known failures in `tokens_contrast_test.go` (11 pairs) carry over unchanged.
- Disabled state: no component has one yet (the palette has no disabled control); the gallery does not show it.
- `Toast` is not in the gallery: it draws through the window, not the view.
- Hover is implemented for rows only; other controls show none.
- Material: the tester's backdrop is opaque, so `states-material` shows the difference in the pane's fill, not the
  real glass. The macOS material is verified only on a macOS runner (L5), not here.

Numeric style literals left in `internal/app` (the color lint passes; these are geometry):

- `ai.go`: 115, 117, 125, 130, 135 (paddings, gaps, sizes)
- `clipboard.go`: 318, 328 (font sizes)
- `emoji.go`: 95, 101, 102 (paddings, cell size, radius, glyph size)
- `forms.go`: 54 (paddings, gaps; the form view), and the snippet and note views added with WP1.9 (`notes_editor.go`,
  `notes_floating.go`, `notes_view.go`, `snippets_view.go`): font sizes 11, 12, 13, 14 and 15 and margins 12 and 10,
  listed here so they move to tokens with the views' next change.
- `palette.go`: 154, 218, 225, 234, 235 (paddings, radius, border, width)

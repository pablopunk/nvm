# Design review: WP0.8

Evidence for `PLAN.md` section 2 (the UI validation checklist) on the design package and the palette views built on
it. Every golden is a PNG under `testdata/golden/`; "all four" means light and dark at 1x and 2x. Ticks are the
author's; no second reviewer has checked this yet, and the owner's sign-off on the sheet is still open.

Status words: **met** (evidence below), **partial** (some of it, debt in the README), **open** (not done here),
**n/a** (the item does not apply to a component).

## Commands run (box, Linux)

- `gofmt -l .`: clean.
- `go vet ./...`: clean.
- `go test -race -count=1 ./...`: all packages pass, including the layer test, the platform grep test, the size
  ratchet and the color-literal lint, which now has no exemption for `internal/app`.
- `go test -count=1 -run '^TestPalette' ./internal/app` with `Strict: true` on the palette goldens: 66 of 66 pass
  byte for byte. The flag was then reverted to the default tolerance.
- `go test -count=1 -run '^TestGallery' ./internal/design` at the default tolerance: all pass.
- `go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...`: clean, exit 0.
- Golden determinism: the strict palette compare ran three times in separate processes, 76 of 76 each time; the
  gallery compare ran three times at the default tolerance, all passing.

Not run: the rig (L4), macOS and Windows runs (L5, L6). Nothing was run on a desktop; all Go ran in the box.

## Section 2 checklist

| Item | Status | Evidence / note |
| --- | --- | --- |
| Golden PNGs, light and dark, 1x and 2x | **met** | palette: 15 views, `testdata/golden/palette/*` (60 files); gallery: 8 goldens, `testdata/golden/design-gallery/*` (32 files) |
| States: empty | **met** | `clipboard-empty`, `no-results`, `gallery.states` (EmptyState) |
| States: loading | **n/a** | no component loads; the chat spinner is in the app and is not captured |
| States: error | **met** | `gallery.states` (FormError); `snippet-form` |
| States: selected | **met** | `root-empty`, `clipboard-preview`, `gallery.rows` |
| States: hovered via tester | **met** | `gallery.rows-hover` (the pointer moved onto "Hovered row") |
| States: prominent | **met** | `gallery.rows` (Automate, prominent and selected) |
| States: long text | **met** | `gallery.rows` (ellipsis on title and subtitle); `root-cl`, `root-fallback` |
| States: focus ring | **partial** | `gallery.states` shows the focused FormField; the ring uses MyGo's accent, not the gold token (README debt) |
| States: disabled | **n/a** | no component has a disabled state |
| Tokens only (no color literals outside design) | **met** | `hexlint_test.go` passes with no `internal/app` exemption |
| Tokens only (geometry literals) | **partial** | 20 numeric style lines remain in `internal/app`, listed in the README |
| Type scale: 21 search, 13 semibold titles, 11 subtitles and sections | **met** | `SearchRow` 21; `Row` title 13/600; subtitle and `Section` 11 |
| Single line with ellipsis | **met** | `SingleLine()` on titles, subtitles, keycaps; `gallery.rows` long row |
| Numerals tabular where aligned | **open** | not checked; no aligned numeric column in the palette yet |
| Spacing on a 4 px grid | **partial** | tile 32 and pane radius 18 are constants; paddings of 5, 6, 10, 14 remain (README) |
| Row height constant across providers | **partial** | one `Row` shape; height varies with subtitle presence, not measured |
| Icon tile 32 | **met** | `TileSize = 32`; `gallery.rows` tile row |
| Keycap and chip heights consistent | **met** | one `Keycap` and one `Chip` padding each; `gallery.controls` |
| Selection: warm plate, faint gold edge | **met** | `SelectedFill`; `root-empty`, `gallery.rows` |
| Prominent rows use the accent tint | **met** | `Row` prominent; `gallery.rows` |
| Hover differs from selection | **met** | `Wash` plate, weaker than the selection plate; `gallery.rows-hover` |
| Focus ring for keyboard focus only | **partial** | MyGo draws the input ring on focus only; color is the default accent |
| Contrast: WCAG AA on every surface, both themes | **partial** | computed in `tokens_contrast_test.go`; 11 known failures carried over (below) |
| Depth: lit top edge, soft shadow, grooves | **met** | `Hi` inset on raised things, `Popover` shadow, `Etched` dividers; `gallery.controls` |
| Depth: gradients only the gold key | **met** | the only `Gradient` is in `Glossy`, used by the prominent tile |
| Corners agree: cut, border, clip | **met** | one `PaneRadius`; dark corners in `gallery.states` and `root-*` goldens |
| Material: no opaque fill with VibrancyGlass | **partial** | `gallery.states-material` differs from `states`; the tester's backdrop is opaque, so the glass itself is not seen (L5) |
| Motion: respects reduced motion | **open** | no animation in the components; the chat reveal is in the app and not checked |
| Density: 720 by 470, hidden scrollbars | **partial** | palette goldens at 720 by 470; scrollbar hiding not checked in a golden |
| RTL and 200% text smoke | **met** | `root-rtl`, `root-zoom-200` (shell); `gallery.rows-rtl`, `gallery.rows-zoom-200` (list) |
| Icons from Lucide via `design.Icon` | **met** | no SVG outside `Icon`; `lucide.go` is the table |

## Per component

| Component (file) | Goldens that show it | Meets |
| --- | --- | --- |
| `Pane` (`pane.go`) | `root-empty`, `gallery.states`, `states-material` | corners, clip, material vs fallback (material partial) |
| `SearchRow` (`search_row.go`) | `root-empty`, `root-cl`, `gallery.controls` (typed, empty, title with back, AI hint) | type scale, focus input, back and chip |
| `Row` (`row.go`) | `root-empty`, `clipboard-preview`, `gallery.rows`, `gallery.rows-hover`, `gallery.rows-rtl` | selected, prominent, hover, long text, section header, accessory, tinted title |
| `Tile` (`tile.go`) | `gallery.rows` (tile row, picture, file icon), `root-empty` | 32 size, glossy prominent key, letters fallback |
| `Section` (`section.go`) | every page of the gallery; `root-empty` ("Suggestions") | 11 semibold |
| `Keycap` (`keycap.go`) | `gallery.controls`, `root-empty` footer | consistent cap |
| `Chip` (`chip.go`) | `gallery.controls`, `root-cl` (AI hint) | icon, bold, text |
| `Popover` (`popover.go`) | `gallery.controls` (popover), `clipboard-actions` | raised, shadow, placement |
| `ActionMenu` (`action_menu.go`) | `clipboard-actions`, `gallery.controls` | selection, key and return glyph |
| `Footer` (`footer.go`) | every palette view; `gallery.controls` (actions, hint) | key hints and hint text |
| `EmptyState` (`empty_state.go`) | `clipboard-empty`, `no-results`, `gallery.states` | glyph, title, subtitle |
| `FormField`, `FormError` (`form.go`) | `snippet-form`, `quicklink-form`, `gallery.states` | label, input, focus, error |
| `UserBubble` (`user_bubble.go`) | `chat-transcript`, `chat-streaming`, `gallery.states` | right alignment, wrap at 520 |
| `Icon`, `lucide.go` (`icons.go`) | every view with an icon | Lucide only |
| `Etched`, `Glossy`, `SelectedFill` (`depth.go`) | `root-empty`, `gallery.rows`, `gallery.controls` | depth rules |
| Tokens (`tokens.go`) | contrast test; every golden | both sets |

## Contrast waivers carried over

From `tokens_contrast_test.go`, unchanged from the prototype's `contrast_test.go`:

- light tertiary on interactive: 4.37
- light muted on interactive: 4.00
- light tertiary on selected: 3.90
- light muted on selected: 3.57
- light green tone on selected: 4.41
- light orange tone on selected: 4.49
- light pink tone on selected: 4.34
- light red tone on selected: 4.48
- light yellow tone on selected: 4.47
- dark tertiary on selected: 4.44
- dark muted on selected: 3.79

The design owner decides whether the tokens change or these pairs stay.

## Harness changes made in this package

- `golden.Options.Advance` and `golden.Options.Interact`: additive; zero leaves every existing golden unchanged.
- `golden` compares a render in its stored PNG form. Before this, a strict comparison failed on identical frames:
  translucent pixels round-trip through PNG with rounding, so the raw render and the decoded golden differed by one
  level in a few channels. The default-tolerance results do not change.

## Not verified here

- L4 rig, L5 macOS (real glass material and Command shortcuts) and L6 Windows: not run.
- Owner sign-off on the screenshot sheet: open.
- The reviewer's re-check of this checklist: open.

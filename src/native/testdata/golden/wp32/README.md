# WP3.2 review sheet: extension manager and capability review

Goldens are checked in under `src/native/internal/app/testdata/golden/extensions/`. They are rendered by the
L2 tests in `internal/app` (`extensions_golden_test.go`) from fixtures only: no clock, no child process.
Each view has a light and a dark render at 1x and 2x.

## Goldens

| Golden | Covers |
| --- | --- |
| `manager-list.{light,dark}.{1x,2x}.png` | Extensions list with one extension per main state: **Running** (Good fixture), **Broken** (Broken source fixture, selected, so the detail pane shows the reason and the runtime diagnostic), **Disabled after failures** (Crashing fixture, reason in the subtitle), **Disabled** (Hanging fixture). Create and Reload commands on top. |
| `capability-review.{light,dark}.{1x,2x}.png` | Review before first enable: the **Enable** row (Enter, selected) and **Cancel**, a known capability (`desktop.apps`), an **Unknown** capability (`remote.control`), and the warning that capabilities are review metadata, not a sandbox. |

Not golden yet: the source pane, the empty "None declared" state on its own, the hovered and focused rows, the
loading message, the RTL and 200% smoke tests for these views. Behaviour for these is covered by L2 tests in
`extensions_view_test.go` and `extensions_review_test.go`.

## Checklist (PLAN.md section 2)

Ticked by the author from the goldens above, on Linux (the box), not on macOS or Windows.

- [x] Tokens only: the new code takes colors from `design.Tokens` and icons from the Lucide set (`power`, `x`, `lock`, `zap`, `rotate-ccw`, `puzzle`). No literal colors added.
- [x] Type scale: titles 13, subtitles and sections 11. Single line with ellipsis (the long subtitle of Crashing fixture ends in `...`).
- [ ] Type scale, source pane: the source text uses 12 pt, which is off the 11/13 scale. Not changed yet; decide whether source takes 11 or stays a code size.
- [x] Selection: the prominent Enable row uses the accent tint with the gold edge; the selected row is distinct from unselected rows.
- [ ] Hover differs from selection; focus ring for keyboard focus only. Not captured (no `Interact` in these goldens).
- [ ] Contrast WCAG AA on every surface in both themes: the new text uses existing token pairs, and `internal/design` tests check tokens. Not computed per new pair.
- [ ] Depth, lit top edge and shadow: taken from the palette row component; not separately reviewed.
- [ ] Corners with and without material (`VibrancyGlass`): not captured.
- [x] Motion: the new views add no animation.
- [ ] Density: long lists scroll with hidden scrollbars. Not captured with more rows than fit (the goldens hold six rows).
- [ ] RTL and 200% text smoke test for these views. Not captured.
- [x] Icons: Lucide names only, through `design.Icon`.

Pre-existing, not changed here: the failure row (`failureRow` in `extensions_rows.go`) uses the icon name
`triangle-alert`, which the Lucide set in `internal/design` does not hold, so it shows the puzzle fallback.

## Not verified

- macOS and Windows: the goldens were rendered on Linux in the box. No macOS or Windows screenshot exists for these views, and no reviewer has signed off the sheet.
- Rig screenshots for L4 to L6 (the real window, the real child processes): the rig agent owns them.
- Human review: the reviewer must re-check the checklist above and record "ok" or the diffs found.

# Main sync log: 1.x (`src/app`) to 2.0 (`src/native`)

Each sync appends a section. It lists every non-merge commit that `main` gained since the previous merge base, limited to
`src/app` and `src/backend`, and says where 2.0 stands on it.

**2.0 status**

- `already in 2.0`: the behavior exists in Go, by construction or by code. Named file given.
- `needs a port`: 2.0 lacks the behavior. Small items are ported in the sync; larger ones are listed as follow-ups.
- `not applicable`: the behavior is tied to Electron, CSS, or 1.x tooling, or 2.0 has no surface for it yet. Reason given.

## Sync 2026-11-04: main 4c4c0425

- Previous merge base: `77b9fe7b` (main at the prior sync).
- Branch: `native/main-sync` (worktree `/Users/pablopunk/.t3/worktrees/nvm/native-main-sync`), based on local
  `t3/investigate-mygo-docs` at `82e7c1d8`, which is nine commits ahead of `origin/t3/investigate-mygo-docs` (`2d7c3da2`).
- Merge commit: `529f3466` (`Merge origin/main into native/main-sync`).
- Commits in window: one squash commit, `4c4c0425` (PR #281, five sub-commits). Split below by behavior.

| Date | Main SHA | Commit | Summary | 2.0 status | 2.0 file or fix |
|---|---|---|---|---|---|
| 2026-11-04 | 4c4c0425 | #281 a | Clipboard history removal actions show only in the Command-K menu, never as inline rows in an empty search (`actionPanelVisibility: 'menu'`). | already in 2.0 | `internal/features/clipboard/view.go` (`BuildRow`: removal actions are row menu actions); `internal/app/clipboard_view.go` (`clipboardView` has no inline actions, only `EmptyTitle` and `EmptySub`). `features/clipboard.BuildView` (which carries the list-level removal actions) has no caller outside tests. |
| 2026-11-04 | 4c4c0425 | #281 b | Empty state and inline actions share one container, so the side-preview card shrinks to fit when nothing previews. | not applicable | Electron CSS grid rule (`.sidePreviewCard:has(.extensionListEmpty)`). 2.0 draws no side-preview presentation yet: `palette.SidePreview` is declared in `internal/palette/view.go` but nothing draws it (PLAN WP3.3). The empty state is `design.EmptyState` in `listBody` (`internal/palette/render.go`). Revisit with the WP3.3 side-preview work. |
| 2026-11-04 | 4c4c0425 | #281 c | Inline view action panel hidden for `menu` and `hidden` visibility, on list empty states and preview views. | not applicable | 2.0 draws no inline view action panel; the only action menu is the Command-K popover (`internal/palette/render.go`, `actionPanel`). The visibility field is not read outside the API type. See F1. |
| 2026-11-04 | 4c4c0425 | #281 d | Import React in JSX files for the classic runtime; narrow the clipboard detail type in a test; lower file-size ratchet baselines. | not applicable | Electron and TypeScript tooling. Go enforces `internal/layers` and `internal/layers/size-baseline.json` instead. |

Counts for this sync (four rows): already in 2.0: 1. needs a port: 0. not applicable: 3. Follow-ups: 1 (F1, found while
checking #281, but it predates this window).

No port was needed from this window, so this sync adds no Go code or tests.

### Follow-ups

- **F1. Extension action visibility is not consumed in 2.0 (WP3.3, pre-window).** 1.x honors `actionPanelVisibility`:
  an item marked `hidden` loses its Command-K action menu except the persistent action (`src/app/palette/App.tsx`,
  `itemActionPanelIsVisible`); a view marked `hidden` drops its view actions from the menu
  (`src/app/palette/local-actions.ts`, `viewActionsForItem`); `menu` and `hidden` drop inline panels
  (`src/app/palette/extension-view/inline-view-actions.tsx`). 2.0 reads neither `ActionPanelVisibility` nor a view-level
  `ActionPanel` in `internal/palette/extview`; `itemActions` in `rows.go` builds from `Primary` and `Actions` only.
  Plan: first define `menu` and `hidden` for items and views in `docs/extension-api.md` and the API types
  (`internal/ext/api/enums.go`); then, in `extview/rows.go` `row()`, keep the primary action for `hidden` and drop the
  menu; map view-level `ActionPanel` into the list's Command-K menu; cover each with an L2 test and the `acme.views`
  fixture.
- **F2. Side-preview presentation (WP3.3).** Clipboard history and extension views declare `side-preview`, but 2.0
  draws only the plain list. When WP3.3 draws it, the empty state must sit in one container with its actions, as in
  #281 b. Add an L2 golden for the empty side-preview list at that point.

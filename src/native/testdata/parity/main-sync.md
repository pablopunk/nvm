# Main sync log: 1.x (the 1.x app) to 2.0 (`src/native`)

Each sync appends a section. It lists every non-merge commit that `main` gained since the previous merge base, limited to
the 1.x app and `src/backend`, and says where 2.0 stands on it.

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
  an item marked `hidden` loses its Command-K action menu except the persistent action (`1.x/palette/App.tsx`,
  `itemActionPanelIsVisible`); a view marked `hidden` drops its view actions from the menu
  (`1.x/palette/local-actions.ts`, `viewActionsForItem`); `menu` and `hidden` drop inline panels
  (`1.x/palette/extension-view/inline-view-actions.tsx`). 2.0 reads neither `ActionPanelVisibility` nor a view-level
  `ActionPanel` in `internal/palette/extview`; `itemActions` in `rows.go` builds from `Primary` and `Actions` only.
  Plan: first define `menu` and `hidden` for items and views in `docs/extension-api.md` and the API types
  (`internal/ext/api/enums.go`); then, in `extview/rows.go` `row()`, keep the primary action for `hidden` and drop the
  menu; map view-level `ActionPanel` into the list's Command-K menu; cover each with an L2 test and the `acme.views`
  fixture.
- **F2. Side-preview presentation (WP3.3).** Clipboard history and extension views declare `side-preview`, but 2.0
  draws only the plain list. When WP3.3 draws it, the empty state must sit in one container with its actions, as in
  #281 b. Add an L2 golden for the empty side-preview list at that point.

## Sync 2026-11-04: main e5f13458

- Previous merge base: `4c4c0425` (main at the prior sync).
- Branch: `native/main-sync` (worktree `/Users/pablopunk/.t3/worktrees/nvm/native-main-sync`).
- Merge commit: `e5f6d9a7` (`Merge origin/main into native/main-sync`). Clean, with no conflicts: main changed only
  the 1.x app and the root docs, and this branch changed only `src/native`.
- Commits in window: one, `e5f13458` (PR #283: AI-ranked second batch in emoji and symbol search). It touches
  the 1.x app and `src/backend`; the rest of its files are `.agents/`, `scripts/file-size-baseline.json` and tests.
- `scripts/lint-changed.cjs origin/main` (Biome on changed files, the Windows platform inventory and the extension
  fixture coverage check) passed on the merged tree.

| Date | Main SHA | Commit | Summary | 2.0 status | 2.0 file or fix |
|---|---|---|---|---|---|
| 2026-11-04 | e5f13458 | #283 a | Views take `onSearchChange`: the host runs it for the trimmed search text once typing rests for 350 ms, and the patch's `searchResultIds` show after the local matches, once each, unfiltered by the query, dropped when the text changes. Failures stay silent, empty text never asks. | needs a port (ported) | `internal/ext/api/view.go` (`OnSearchChange`, list and grid only), `patch.go` (`SearchResultIds`), `internal/palette/extview/search_change.go` and `session.go` (`withSecondBatch`). `internal/app/extension_views.go` gives the views the app's `onUI`. Commit `0d97ada5`. |
| 2026-11-04 | e5f13458 | #283 b | Emoji & Symbols second batch: the decision model is asked in three steps (categories, choice shortlist of 31 plus none of these, one predicate per candidate), matches at 0.5 and up, best first, cached for 32 searches, text under three characters never asks, Flags, Technical and Math are never asked. Signed in only. Each decision batch counts against 1.x's limit of 30 a minute for the extension. | needs a port (ported) | `internal/ai/charsearch` (funnel, matcher, provider, rate limit in `rate.go`); `internal/app/emoji_meaning.go` (account gate, rows score zero so the catalog stays first, `characters()` reads the app clock); `internal/app/emoji_view.go` (`Providers`). Commits `ee64d678`, `79516ced`, `67d307c8`. |
| 2026-11-04 | e5f13458 | #283 c | The shared decision contract sends `probabilities` and `confidence` on choice answers, which the second batch ranks by. 2.0's client dropped them. | needs a port (ported) | `internal/backend/decisions_wire.go` (`DecisionAnswer.Probabilities`, `Confidence`); the recorded contract test now checks them. Commit `8ce856f1`. |
| 2026-11-04 | e5f13458 | #283 d | The host's merge rule for a second batch: local rows first, the batch after them without repeats, and the batch takes its room from the limit or `MaxVisibleItems` first. | needs a port (ported) | `internal/search/append.go` (`AppendedProvider`), `merge_appended.go` (`mergeAppended` replaces `merge`), `internal/palette/extview/search_change.go` (`searchChangeRows`). Commit `afaebdb5`. |
| 2026-11-04 | e5f13458 | #283 e | Dev UI fixture "Grid Search Results" (typing "yellow" adds two fruit after a second). | needs a port (ported) | `internal/ext/testdata/extensions/acme.views` command `search-results` and handler `search-meaning`; L2 test and golden in `internal/app/extension_search_change_test.go`. Commits `0d97ada5`, `e3db780b`. |
| 2026-11-04 | e5f13458 | #283 f | The emoji picker declares the `ai` capability and runs the second batch only for an account that can ask. | needs a port (ported) | 2.0 built-ins have no capability review; the account gate in `internal/app/emoji_meaning.go` (`characterDecider`) is the equivalent. A signed-out picker has no second batch. Commit `1e948a62`. |
| 2026-11-04 | e5f13458 | #283 g | The builder's API reference lists `onSearchChange` as an AI capability note. | needs a port (ported) | 2.0's reference is generated from the Go API (`internal/ai/builder/apiref`). Regenerated `reference_gen.go` on the box; the drift test passes. Commit `0d97ada5`. |
| 2026-11-04 | e5f13458 | #283 h | Type and doc updates: `nevermind-extension-api.d.ts` (`searchResultIds`, `onSearchChange`, `Invocation.value`), and `model.ts` (`CommandView.onSearchChange`, `CommandViewPatch.searchResultIds`). | needs a port (ported) | Go TSDoc in `internal/ext/api` and `docs/extension-api.md` (section 7, the `OnSearchChange` bullet and the `Invocation` comment). Commit `0d97ada5`. |
| 2026-11-04 | e5f13458 | #283 i | `selectionAfterPatch` moved from `App.tsx` to `extension-view/view-patches.ts`. | not applicable | Refactor only: the rule is pre-window 1.x behavior and moved without change. The 2.0 cursor after patches is the palette's (`internal/palette/search.go`, `showRows`), a question from before this window, not new work here. |
| 2026-11-04 | e5f13458 | #283 j | `view-normalizer.ts` normalizes the `onSearchChange` action for the Electron IPC action cache. | not applicable | Electron IPC clone-safety. 2.0's `ActionRef` is data in the Go view and is looked up by ID in the child, so nothing needs normalizing. |
| 2026-11-04 | e5f13458 | #283 k | Formatting: numeric separators in a test, import order in `emoji-symbols.ts`, `actionPanelFromActions` brace style, `use-view-search-results` hook placement in `App.tsx`. | not applicable | TypeScript and Biome formatting only; no behavior. |
| 2026-11-04 | e5f13458 | #283 l | `.agents/skills/nevermind-box-signin/SKILL.md` (a skill for running 1.x signed in on a disposable box). Outside the the 1.x app and `src/backend` filter. | not applicable | 1.x agent tooling for the Electron app. The 2.0 equivalent of a signed-in run is the rig (`src/native/PLAN.md`, WP0.3). |
| 2026-11-04 | e5f13458 | #283 m | `scripts/file-size-baseline.json` raised one entry and lowered another. Outside the filter. | not applicable | 1.x size ratchet. 2.0's ratchet is `internal/layers/size-baseline.json` (empty; no file over 400 lines). |

Counts for this sync (thirteen rows): already in 2.0: 0. needs a port: 8 (all ported in this branch). not applicable: 5 (a
refactor, Electron IPC, formatting, outside-filter agent tooling and size ratchet).

### Ports and commits

Work on the emoji matcher (`native/grid-keys`) is in this branch as two cherry-picks, `f18244c4` (1.x matcher and ranking,
160-row cap) and `a577199f` (recents removed, so an empty query lists the catalog as 1.x does). The full merge of
`native/grid-keys` was started and aborted: 11 files conflict, and several of those are product decisions on the other branch
(the update channel toggle, the restart copy and wording, the search row's caret and key hooks), not emoji code. They stay
with that branch's owner. The grid arrow-key commits (`ad4bf350`, `c68fb951`) also stay there: they conflict with this
branch's search row, and the test that needs them came off the emoji test file in `cca0e78b`.

Decisions in 1.x have no tier: `ai.decide` sends the input, the questions and an optional abort signal, and the backend picks the
route. The 2.0 decisions client does the same, so the port names no tier and no model. The 1.x loading state is none: the
second batch arrives without a spinner, and the 2.0 picker does the same. Its copy: 1.x's action title "Search by Meaning" is
never drawn, so the port has no copy to match.

The two layers of 1.x's limit match: a search that is refused by the limit shows no meaning match and is not cached, as
1.x's uncaught error does.

- `8ce856f1` backend: choice and score decisions keep `probabilities` and `confidence` (row c).
- `afaebdb5` search: appended providers, the second-batch merge rule (row d).
- `0d97ada5` ext: views take `onSearchChange` and `searchResultIds`; API reference regenerated; docs; acme.views fixture (rows a, g, h, e in part).
- `ee64d678` ai: `internal/ai/charsearch`, the three-step funnel with its matcher and provider (row b).
- `1e948a62` emoji: the picker adds the meaning matches after the catalog's matches, golden `emoji-meaning` (rows b, f).
- `e3db780b` app: the search-results fixture's second batch through the child, golden `extension-search-grid` (row e).
- `f18244c4`, `a577199f` cherry-picks from `native/grid-keys`: the 1.x emoji matcher and the recents removal (row b's base).
- `67d307c8` ai and app: the 30-a-minute decision limit, and L2 tests on a real `backend.Client` over the fake HTTP port,
  with the decisions route answering the way the model does (row b).
- `cca0e78b` app: the emoji tests follow the grid-keys matcher, and the arrow-key test is dropped (see above).

Beyond the rows, one fix found while porting: `extensionViewDeps` (`internal/app/extension_views.go`) did not pass
`OnUI`, so a view's interval refresh ran its handler on the timer goroutine instead of the UI thread. It now passes
`p.onUI`, as the Deps comment requires. This predates the window.

### Checks

- On the box (slot `mainsync`): `go build ./... && go vet ./... && go test -count=1 ./...` exited 0, 79 packages ok, no FAIL,
  on the tree at `cca0e78b` (the code this log describes). The log commit after it changes no Go file.
- The box disk was full (28 MB free) at the first Go run. Only the `mainsync` slot and go-build cache files older than 60
  minutes were removed, as the brief allows.
- Locally: `mise exec -- pnpm check:changed origin/main` (Biome changed files, Windows inventory, fixture coverage) passed.
- Not run: any app launch, GUI, or macOS or Windows run (CLAUDE.md). The emoji and extension goldens were rendered
  on the box and inspected here (light and dark, 1x and 2x).

### Follow-up (new)

- **F4. Grid arrow keys and the rest of `native/grid-keys`.** Its palette commits (`ad4bf350`, `c68fb951`, and the rest of its
  30 commits) are not in this branch. The conflicts are product decisions the owner of that branch should settle: the
  update channel (`Other()` toggle or `Next()` cycle, with Dev), the restart copy ("Restart" or "Restart Computer"), and the
  search row's caret and key hooks. The emoji arrow-key test came off with it. Open because a merge needs those decisions.

### Deliberate difference

- Equal shortlist probabilities keep the order the options were offered in. 1.x keeps the order of the model's JSON
  answer. The 2.0 client decodes the answer into a Go map, which has no order, so the offered order stands in for it. Only
  characters the model rates equally can change order.

### Coordination with other native branches

- `native/grid-keys` (not yet in `native/mac-preview`): its emoji commits are in this branch (see above). Its other 28
  commits are not, because of the conflicts listed above; they need that branch's owner to settle the product decisions
  before a merge. The emoji row limit is 160 (1.x `MAX_VISIBLE_ITEMS`), and the appended-provider rule (row d) keeps the
  meaning batch inside that limit.
- `native/ext-ai` (not in `native/mac-preview`) serves `ctx.ai` decisions to extensions. Its mapping from the backend
  answers must carry `Probabilities` and `Confidence` on choice answers, which this sync adds to the backend type. When
  that branch merges, the mapping needs the same two fields, or 1.x extension choices lose their probabilities. The
  extension decision limit of 30 a minute (`ext-ai`'s quota) and this picker's limit are separate counts today; they
  should share one when both land.
- `native/views-parity`, `native/ext-api-gaps`, `native/cmdk-parity`, `native/chat-ux`: no commits in the paths this sync
  touches (`internal/palette/extview`, `internal/ext/api`, `internal/search`, `internal/ai/filesearch`, `internal/app`
  emoji files).

### Follow-ups

- **F1. Extension action visibility (from the 4c4c0425 sync; not in this window).** 1.x honors
  `actionPanelVisibility` for items and views. 2.0 reads neither field. Open because the semantics of `menu` and `hidden`
  for items and views need an owner decision recorded in `docs/extension-api.md` first, as the prior plan says.
- **F2. Side-preview presentation and the render cap (from the 4c4c0425 sync, WP3.3).** 2.0 draws no side-preview
  presentation, and `MaxVisibleItems` is stored but not drawn (lists or grids). The second-batch room rule (row d) is in
  place for when the cap is drawn. Open because it is WP3.3 work (XL in PLAN.md), outside this window.

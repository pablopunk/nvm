# Extension API 1.x to 2.x parity plan

Status: proposed, 2026-11-09, against `20ba2e70`. Extends `src/native/PLAN.md` with WP3.11 to WP3.17.

## 1. Outcome

The 2.x extension API replaces 1.x with no loss. Each of the four promises below is a check that fails in CI, so it stays true after this plan ends.

| Promise | Gate | Fails when |
|---|---|---|
| Everything that was possible still is, or has a better alternative | G1 ledger | A member of the 1.x API has no ledger row, or a row names a Go target that does not exist |
| Everything is as fast as it can be | G2 budgets | A hot path misses its budget or regresses against its baseline |
| Everything is discovered by the AI builder | G3 discovery | A supported capability is missing from the reference, has no working example, or fails its builder eval |
| Everything is implemented as intended | G4 conformance | An accepted input has no reader, a service has no route from child to real port, or a surface has no fixture |

"As fast as it can be" cannot be proven, so G2 means this: every hot path has a budget and a benchmark, and the profile shows no avoidable cost above 10% of that path. Anything left is listed with the reason it stays.

## 2. Where it stands

From a static comparison of `src/app/resources/nevermind-extension-api.d.ts` and `internal/ext/api`, plus one measurement on the owner's Mac. The comparison matched names, so a match does not prove equal behavior. WP3.11 settles that.

**Covered by name or a direct rename:** all view kinds, form field types, trigger types, menu bar rows and popovers, windows, shell, files, clipboard, OCR, dictation, indicators, AI, global shortcuts.

**Performance, measured 2026-11-09 on the owner's Mac (M4 Pro), latest build:**

| Measure | Value |
|---|---|
| Host RSS | 298 MB |
| Extension child processes | 29 (of 58 entries in the extensions folder) |
| Children RSS, summed | 1,211 MB (about 42 MB each, max 47 MB) |

RSS counts shared pages once per process, so the sum overstates real memory, and the first step of WP3.14 is measuring physical footprint. The B4 idle budget is 100 MB, though it was written for built-ins only. `internal/ext` has no benchmarks, and B1 to B8 are still marked proposed. The spike's child numbers (spawn 7 to 11 ms, realistic round trip 274 µs p50) come from a shared Linux box and trivial extensions.

**Stale documents to fix in WP3.11:** `testdata/parity/parity-review.md` lists indicators, dictation and extension global shortcuts as missing, and all three exist now. `docs/extension-api.md` says section 14 maps every TypeScript type, but section 14 is author examples.

## 3. Decision for every known gap

Decided here so that the work packages can proceed. The owner can overrule any row.

### 3.1 Missing from 2.x

| 1.x | Decision | Result |
|---|---|---|
| `ctx.state` | Better alternative | In 1.x it carries view state such as `selectedItemId`. Use the invocation for view state, package variables for per-process state and `Cache` for shared state. A fixture proves each one, and the reference names them |
| `ctx.extension.rename(...)` | Add | Title, subtitle, command title and command subtitle through the existing authoring path. The authoring code handles titles only today, so it gains subtitles |
| `ctx.actions.refreshContributions()` | Add | `Views().RefreshCommands()`, with the same rate limits as `InvalidateRoot`, returning what changed |
| `actions?(ctx)` and `ctx.action()` | Already better | Durable actions are `Commands`. Ledger row and a fixture for dynamic commands |
| `ctx.desktop.selection.read()` | Add | Returns access state, text and files in one call |
| `ActionSection.lazyActions` | Better alternative | 1.x only appends these to `actions`, so nothing is lost by using `Actions`. Real laziness is covered in WP3.14 |
| Folder field `canCreateDirectories` | Add | Passed to the native picker, default true |
| Preview `clipboardType` | Add or map | Follow what the 1.x palette shows for text, image and video |
| `previewIntent` | Add or map | Must keep the selected item's in-palette preview behavior |
| `ctx.actions.native` | Drop deliberately | Replaced by named actions and built-in-only host actions. Every use in 1.x gets a named replacement in the ledger |
| `ownership.claim`, `remove` | Drop deliberately | Host-only in 1.x. The host keeps enforcing ownership |
| 1.x JS and TS extensions | Add a converter | No TS-to-Go converter exists in the Go app, only `convertedFrom` state inheritance. See WP3.16 |

### 3.2 Accepted by 2.x but ignored by the host

Each one is implemented or removed. Nothing stays accepted and ignored.

| Field | Decision |
|---|---|
| `EditorSpec.LivePreview` (1.x `preview`) | Implement |
| `EditorSpec.TitleFromLine` (1.x `titleFromContent`) | Implement |
| `Image.Shape`, `Tint`, `Mask` | Implement in rows, grids and details through design tokens |
| `WindowOptions.IgnoreMouseEvents` | Implement where the UI toolkit allows. Otherwise report `unsupported` through capabilities |
| `PasteOptions.KeepPaletteOpen` | Implement |
| `Pagination.PageSize` | Implement |
| `TemplateOptions.IncludeSelectedText`, `PromptMissing` | Implement |
| `Confirmation.Destructive` | Implement in the confirmation row |

## 4. Work packages

Order: WP3.11 first, then WP3.12 to WP3.16 in parallel, then WP3.17. Each one follows the PLAN.md ladder, and runtime checks run in CI or the rig only.

### WP3.11 Parity ledger and gates `[M-L]`

- A script uses the TypeScript compiler to list every member of the frozen 1.x `.d.ts` into `testdata/parity/api-1x-members.json`.
- `testdata/parity/api-ledger.json` has one row per member: `same`, `renamed`, `better` or `dropped`, the Go target, a one-line note, and the fixture or test that proves it.
- G1 test: every member has a row, every Go target resolves through `go/types`, and every `dropped` row has a reason.
- Add the G4 static checks from WP3.13 and the G3 checks from WP3.15 as failing or allowlisted tests, so the gaps are visible from day one. Allowlist entries can only be removed.
- Fix the stale documents listed in section 2.
- **Done when:** the ledger is complete, each name match in section 2 has been read for behavior, and new differences are added to section 3.

### WP3.12 Close the API gaps `[M]`

- Build every `Add` and `Better alternative` row in section 3.1 through the normal path: API type, child proxy, service handler, fixture, reference.
- **Done when:** G1 has no unresolved rows and each new surface has a fixture running in a real child process.

### WP3.13 Make every accepted field real `[M-L]`

- Build section 3.2.
- G4 input check: a `go/ast` test requires every field of an input type to be read outside `ext/api`, with output-only fields exempt.
- G4 route check: every `Context` method has a child proxy, a service handler and a port wired in the real app. A release build must not return "not available in this build" for a capability it reports as present.
- **Done when:** the allowlist is empty, apart from fields reported as unsupported per OS.

### WP3.14 Extension runtime speed `[L]`

Work from measurements, in this order:

1. **Measure.** Add benchmarks for the extension path: spawn to ready, search round trip, service call, view first paint, action panel open. Record physical footprint on the Mac with a large extension set. Ratify B1 to B8 and add budgets for the rows below.
2. **Stop running what is not needed.** Start a child on first use. Keep warm only extensions with search providers, menu bar items or triggers, and reap idle ones. Serve commands and root rows from cached snapshots, so the palette never waits for a spawn.
3. **Cut per-child cost.** Reduce the interpreter symbol table each child loads, and measure one shared host for trusted built-ins against one child each.
4. **Stable results.** Make one contract for search updates: a result row is replaced only by a newer answer for the same query, never by a partial or empty one. This is the root search flashing, reported again on 2026-11-09. A test replays the slow-provider sequence and asserts that no frame shows a row set that later reverts.
5. **Remaining hot paths.** Confirm that view snapshots persist across restarts, fetch action panels only when opened, and count budget kills in diagnostics. A `Search` over 100 ms currently kills the child, so check that this is not a cost on slow machines.

Proposed budgets, to be ratified after step 1:

| Path | Budget |
|---|---|
| Hidden idle, host and all children, physical footprint, owner's extension set | ≤ 300 MB |
| Hidden idle CPU | ≤ 0.3% of one core |
| Keystroke to frame with 30 extensions and one slow provider | B2 unchanged |
| Cached view first paint | One frame |
| Cold child to first result | ≤ 50 ms on the Mac |
| App cold start with the owner's extension set | B3 unchanged |

- **Done when:** G2 runs in CI against baselines, and the profile review is written down for each hot path.

### WP3.15 AI builder discovery `[M-L]`

- G3 reachability: every `same`, `renamed` and `better` row appears in the generated reference, and every `dropped` row is absent from it.
- Add a 1.x-to-2.x name index from the ledger, because models and users know the 1.x names. Keep it in the on-demand reference and watch the compact size budget.
- Add task recipes ("add a menu bar timer", "watch a folder", "stream AI text") that lead to the right API. Every recipe is an example that type-checks and runs in the interpreter, not only under `go/types`.
- Tell the builder what the interpreter cannot do, and reject those patterns before the user sees the result: methods on interpreted structs, `return <-ch`, missing standard library packages.
- Generate an OS availability table from capabilities, so the builder does not offer something the user's OS lacks.
- Build the eval harness that WP3.9 left open: at least one task per ledger area, scored on compiling, running against a fake host and choosing the intended API. Model IDs stay out of code.
- **Done when:** G3 is green and the eval pass rate is recorded, with each failure fixed or explained.

### WP3.16 Conformance across surfaces and platforms `[L]`

- A fixture for every view kind, action kind and service, in the rules of `docs/extension-api-ui-fixtures.md`, each exercised through a real child.
- Behavior checks against 1.x for navigation, action panels, shortcuts, dismissal and focus return, with the journeys still open (J14, J27 and the open rows in WP3.3).
- macOS first: run the fixtures on a CI-built package, with owner checks for anything permission-bound.
- Windows: implement or explicitly mark each omission in WP5.3, so the capability table is truthful.
- Converter: a palette command that converts a 1.x extension to Go through the builder, using the ledger, and sets `convertedFrom`. The owner's Go-converted extensions are the regression set.
- **Done when:** every ledger row has proof on macOS and Linux, and Windows rows are proven or marked deferred.

### WP3.17 Sign-off `[M]`

- Run the 1.x and 2.x side by side for each ledger area, and update `parity-review.md`.
- Review the remaining `better` and `dropped` rows with the owner.
- Make the Go API the source of truth in `AGENTS.md`, replacing the `.d.ts`.
- **Done when:** G1 to G4 are required CI checks and the owner accepts the review. Packaged-app, permission-bound and Windows checks that did not run stay listed as outstanding.

## 5. Risks

- **Name matches hide behavior gaps.** The ledger is expected to find more than section 3 lists, so WP3.12 and WP3.13 are sized as minimums.
- **Lazy start can make first use slower.** The cold-child budget and cached snapshots guard against it, and an extension that misses the budget stays warm.
- **The reference has a size budget.** New index and recipe text goes in the on-demand reference, with only pointers in the compact one.
- **The interpreter stays a limit.** The Yaegi table stops at Go 1.22 in the spike's version, so the recipes and preflight must be checked after any Go or Yaegi bump.

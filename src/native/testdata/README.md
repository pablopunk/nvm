# testdata

Checked-in fixtures that tests and reviews read. Nothing here is generated at test time, except the baseline that
`scripts/rig/perf.sh --write-baseline` writes.

| Path | What it holds | Read by |
| --- | --- | --- |
| `budgets.json` | Performance budgets B1 to B7 and their thresholds (PLAN.md section 3.2). | `internal/perf`, `cmd/perfcompare`, the rig's perf test |
| `perf-baseline.json` | The last recorded budget run: the environment and the measured values. | `cmd/perfcompare`, `internal/perf` |
| `contracts/desktop-v1/` | Copies of the backend's recorded desktop-v1 bodies: the compatibility manifest, the active-model descriptor, a decision, and the error envelopes. The originals are in `src/backend/src/fixtures/contracts/desktop-v1`. | `internal/backend` contract tests |
| `queries.txt` | The 20 canned queries of the parity review, one per line. The first line is the empty query. | `internal/app` root table test, which compares the results with `internal/app/testdata/queries-expected.json` |
| `golden/wp32/`, `golden/wp33/` | Review sheets for WP3.2 and WP3.3: what each golden shows and the checklist. The PNG goldens are under `internal/app/testdata/golden/`. | Reviewers |
| `reference/` | Screenshots of the Electron palette, for side-by-side review with Go renders. They are not compared pixel for pixel. The README lists how each was made. | Reviewers |
| `parity/parity-review.md` | The first-pass parity sheet from Electron 1.x to Go 2.0: each feature, its Go status, the platforms it ran on and its gap. | Owner and the parity review (cutover.md section 6) |

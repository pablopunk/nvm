# Reference capture: regenerating the Electron palette references

Purpose: `testdata/reference/` holds screenshots of the Electron palette that the owner compares by eye with
the Go rewrite (PLAN.md section 2). This note says how those screenshots are produced today, what has to be
added to regenerate the full set, and the one open problem. It is documentation only; no script exists yet.

## What the repository does today

1. **Producer: `tests/electron/palette.smoke.spec.ts`.** Its test "searches and invokes the safe built-in
   action, then hides and shows" runs only on Linux. It types `Open Settings` into the palette input, waits
   for the row, and calls `page.screenshot` to `<NVM_TEST_ARTIFACT_DIR>/linux-palette.png`. That is the only
   palette screenshot the suite takes; it shows one state (one query, one result list).
2. **Runner: `scripts/electron-test.cjs`** (`pnpm test:electron`). It creates a temporary user-data directory,
   sets `NVM_TEST_MODE=1`, `NVM_TEST_USER_DATA_DIR` and `NVM_TEST_ARTIFACT_DIR=test-results/electron`, and runs
   Playwright on `palette.smoke.spec.ts` and `clipboard-concealed.smoke.spec.ts`. It needs a display, so CI
   wraps it in `xvfb-run -a`.
3. **Artifact: `.github/workflows/ci.yml`, job `electron-smoke` ("Linux Electron smoke").** After a successful
   run it uploads `test-results/electron/linux-palette.png` as the artifact `linux-palette-screenshot`
   (7-day retention). On failure it uploads `electron-smoke-artifacts` instead.
4. **Reporter: `.github/workflows/report-linux-palette-screenshot.yml`** with
   `.github/scripts/report-linux-palette-screenshot.cjs`. It runs on `workflow_run` of CI, finds the
   `linux-palette-screenshot` artifact of that run, validates it (PNG signature, CRC, 912 x 672 pixels, under
   5 MiB), commits it to the `ci-screenshots` branch under `pr-<number>/linux-palette.png`, and creates or
   updates one PR comment marked `<!-- linux-palette-smoke -->`.

Pieces 1 to 3 produce the screenshot; piece 4 publishes it to a PR. Nothing in the repository writes
`testdata/reference/`; those files are copied by hand from a capture.

## Finding: the PR path does not produce screenshots

`electron-smoke` has `if: github.event_name == 'workflow_dispatch'`. Pull request CI runs therefore skip the
job, so no `linux-palette-screenshot` artifact exists for them. The reporter then logs "No Linux palette
screenshot; smoke was not applicable" and stops. The reporter itself requires `workflow_run.event ==
'pull_request'`, so a manually dispatched run (which has a different event) is not reported either. As
configured, the PR comment is never published. Fixing this is a CI decision for the owner: either run the
job on pull requests, or report dispatch runs by their PR number.

## Regenerating the references

Runtime launches of the Electron app belong in CI or a rig container, never on a maintainer's desktop
(PLAN.md section 1; `CLAUDE.md`). To get a capture today:

1. Dispatch CI for the branch (`workflow_dispatch`), so `electron-smoke` runs (see the finding above).
2. Download the `linux-palette-screenshot` artifact from that run.
3. Check its size is 912 x 672, then copy it into `testdata/reference/` under a descriptive name and add a row
   to `testdata/reference/README.md` (what it shows, appearance, date, how produced).

To capture the states the references need (root search with the `Automate` fallback, clipboard history with a
preview, AI chat, the Tab hint, dark appearance), the suite must take more screenshots. That work is not done.
The shape of it: add a spec in `tests/electron/` that drives each state through the same `input.fill` and
`page.screenshot` calls, write each file to the artifact directory, and extend the upload step and the reporter
to accept them. The dark appearance needs the system theme set in test mode; check how `NVM_TEST_MODE` handles
it before relying on it.

## Open problem: the transparent palette does not show on Linux

The palette window is created with `transparent: true` (`1.x/electron/app/palette-window.ts`, line 136), so
its rounded corners should show the desktop outside the pane. In the Linux box runs during the Electron
investigation that did not happen: `root-search-automate.png` shows an opaque paper pane, with the wallpaper
visible only outside it. The same investigation used a minimal probe (a `BrowserWindow` with `transparent: true`,
`frame: false` and a translucent `rgba(...)` background, the scratchpad's `elhello.js`) on Xvfb; its output is not
kept in the repository. `src/native/README.md` states that a compositor is needed for transparency on Linux.

What is known: the palette requests transparency, and under Xvfb it did not show it. What is not known: the cause. The likely candidate is that Xvfb provides no compositing (ARGB) visual, but that has
not been checked. CI runs under the same `xvfb-run`, so CI captures will show the opaque fallback too.

This is recorded, not fixed. Do not change the window setup to chase it from this document. The next step, when
someone takes it up, is to test under a real compositor (for example a Wayland or X11 session with a compositing
manager) and compare with the same probe, before changing the palette.

# scripts/rig: the L4 rig

Runs the real app on a desktop that has what the app needs (Xvfb, openbox, picom, an xfce4-panel tray host, dbus, a PulseAudio virtual microphone), drives it through the MyGo automation plugin and xdotool/xclip, and writes screenshots and logs. Nothing here runs on a maintainer's desktop; CI runs it in the container (`.github/workflows/native-rig.yml`), and the shared test box runs it without Docker.

## Files

| File | What |
| --- | --- |
| `Dockerfile` | Debian trixie image: desktop, input tools, ImageMagick, the GTK/GL runtime MyGo loads, and the Go version from `go.mod`. |
| `entrypoint.sh` | Starts the desktop, runs the command given as arguments, then stops only the PIDs it started. |
| `rig.sh` | Driver: `build`, `run`, `up`, `shot`, `logs`, `down`. |
| `run-journeys.sh` | Builds the app with `-tags automation` and runs the rig tests. Both the container and the box run it. |
| `window-x11.sh` | Private Xvfb with openbox and a panel, a feh probe window, and the X11 window engine check (WP4.1b). Needs `RIG_ARTIFACTS`; leaves before, left-half and after screenshots there. |
| `assets/wall.png` | The fixed wallpaper (the shared test box's `/root/wall.png`), so pixels are stable across runs. |

The Go side is `internal/rigtest` (build tag `rig`) and the journeys in `internal/rigtest/journeys_test.go`. The app is built with `-tags automation`, which links the MyGo automation plugin (`cmd/nvm/automation_on.go`). Default builds do not contain the plugin.

## Run in Docker

```sh
scripts/rig/rig.sh build                 # image, Go from go.mod
scripts/rig/rig.sh run -run Journey -v   # builds the app, runs the journeys in a fresh container
```

Artifacts go to `$RIG_ARTIFACTS` (default `$TMPDIR/nvm-rig-artifacts`): `tests/<test name>/` holds `app.log`, `*-palette.png` (the palette window as rendered) and `*-desktop.png` (the whole display); the desktop service logs are in `services/`.

For manual work, `rig.sh up` starts a long-lived container, `rig.sh shot out.png` captures its display, `rig.sh logs` tails it, and `rig.sh down` removes it.

## Run in the shared test box (no Docker)

The box's `:99` display is shared with other agents, and an app running there may already hold the palette hotkey. Use a private display number that nothing else uses:

```sh
scripts/boxgo.sh wp03 'RIG_DISPLAY=:95 RIG_ARTIFACTS=/root/wp/wp03-logs NVM_RIG_BIN=/root/wp/wp03/nvm-rig NVM_RIG_ARTIFACTS=/root/wp/wp03-artifacts bash scripts/rig/entrypoint.sh bash scripts/rig/run-journeys.sh -run Journey -v'
```

`entrypoint.sh` starts Xvfb on `RIG_DISPLAY` itself. If the number is taken, Xvfb fails and the run stops; it never attaches to someone else's server. `run-journeys.sh` is the same build-and-test step the container runs.

## Helpers (`internal/rigtest`)

| Helper | What it does |
| --- | --- |
| `Launch(t, Options)` | Starts `$NVM_RIG_BIN` detached (own session) on `$DISPLAY` with an isolated `HOME`, `XDG_RUNTIME_DIR` (which keys MyGo's single-instance socket) and automation socket. Waits for the `ready` log line and for the socket. `Options.Hotkey` writes an isolated `settings.json`. |
| `Stop`, `KillHard` | SIGTERM then SIGKILL for the launched pid only; `KillHard` is SIGKILL for crash tests. |
| `SummonPalette`, `DismissWithEscape`, `ClickAway` | Hotkey summon that waits for visible and focused; Escape and desktop click that wait for hidden. |
| `PressHotkey`, `Type`, `Key`, `Click` | xdotool input on the app's display. `PressHotkey` converts `Alt+Shift+Space` to xdotool keys and rejects unknown names. |
| `WaitFor(text, timeout)` | Polls the palette's elements (after `idle`) for text in a label, value, placeholder or description. Fails with the labels it saw. It never compares screenshots. |
| `FindNode`, `SearchValue`, `Windows`, `PaletteWindow`, `WaitPalette`, `Tree` | Automation queries for assertions on roles, values and window state. |
| `Screenshot(name)`, `CapturePalette(name)` | Whole display (`import`) and the palette window (automation `capture`) into the test's artifacts. |
| `ClipboardSet`, `ClipboardGet` | xclip on the CLIPBOARD selection. `ClipboardSet` returns once xclip's child is serving the selection, and the child has no inherited pipes. |
| `TrayItems` | Best effort: lists the status notifier items registered with the session's watcher. No journey depends on it yet. |
| `ArtifactsDir(t)` | `$NVM_RIG_ARTIFACTS/<test name>`. |

## Known flakiness and what is done about it

- **First hotkey press after `ready` can be lost.** `Launch` waits one second after `ready`, and `SummonPalette` presses again if the palette is still hidden after three seconds. A press is repeated only while hidden, because a press on a shown palette hides it. Nine full journey runs (five on the box, four in the container) needed no retry, so the retry is a safety net that has not been shown to be needed.
- **The hotkey grab is per display.** Another client holding `Alt+Shift+Space` on the same display receives the key instead of the app. Use a private `RIG_DISPLAY`, or `Options.Hotkey` for a rig that must avoid a shared grab.
- **The single-instance lock is per user.** MyGo keys it by app name under `XDG_RUNTIME_DIR`, and a second instance would hand over to the first and exit. Each `Launch` uses its own runtime directory.
- **The tray host must exist before the app starts.** `entrypoint.sh` waits up to 20 seconds for `org.kde.StatusNotifierWatcher` and warns if it never appears.
- **Software GL.** Xvfb renders with llvmpipe. Frame timings here are not GPU timings, and budgets measured in the rig are not the product's.
- **Fixed wallpaper and fonts.** The wallpaper is `assets/wall.png`. The image installs DejaVu, Liberation and Noto Color Emoji, the core fonts of the box; text metrics are only checked against those fonts.

## Screenshots and work packages

Each journey test saves `*-palette.png` and `*-desktop.png` per step (names start with the journey id, for example `j01-1-summoned`). A WP that touches a journey attaches the palette PNGs of that journey to its PR, from the `linux-rig-artifacts` artifact of the CI run, and names the commit and run id they came from. Screenshots from a local run are not evidence for a PR.

## What the rig does not cover yet

Scripted journeys are J01, J02, J03, J05, J06, J08, J09, J10, J15, J16 and J22. Each covers the smoke part of its pass condition. Not covered: no focus restoration, frecency, paste into another app, pinning, or restart durability. J15 checks the failed rows and the reasons the palette shows, but after a child's first failure the reason is the generic `not running: failed`, and J15 does not check the first failure's own reason. J16 covers a folder dropped before launch, not one dropped while the app runs. The macOS and Windows jobs in `native-rig.yml` launch the app and capture the desktop, and they are `continue-on-error` until hotkey injection and the palette assertions exist there (Accessibility permission on macOS, SendInput on Windows).

## Performance budgets (WP0.5)

`perf.sh` measures budgets B1 to B7 of PLAN.md section 3.2 on a rig desktop, writes `perf-report.json`, and compares it with `testdata/budgets.json` and `testdata/perf-baseline.json`. The thresholds are ratified (`"ratified": true`); on noisy shared runners judge a run by the median of three. B8 (extension call budget) belongs to the extension host (WP3.2) and is not measured here.

```sh
# in the shared test box, on a private display (never :99 while other agents use it)
scripts/boxgo.sh wp05 'RIG_DISPLAY=:96 RIG_ARTIFACTS=/root/wp/wp05-logs bash scripts/rig/entrypoint.sh bash scripts/rig/perf.sh --soak=2m'
```

Options: `--soak=<duration>` (default 2m; PLAN's full soak is 30m), `--warn-only` (print violations, exit 0, still write the report), `--write-baseline` (store this run as `testdata/perf-baseline.json`). Exit 1 means a budget was violated (or the measurement itself failed); the table shows each budget, its value, the limit, the status, and the delta from the baseline.

| Budget | Measured as | Where |
| --- | --- | --- |
| B1 | Hotkey to first frame, p50/p95 over 20 presses: from the `xdotool key` call (including its spawn) until the automation `windows` answer says visible and `idle` returns (no frame pending). A press repeated after 1 s is counted in the sample. | `perf_measure_test.go` |
| B2 | Keystroke to updated results, p50/p95 over 30 letters: from the return of `xdotool key` until the palette's result rows (its elements minus the search field) change. The spawn of xdotool is excluded. | `perf_measure_test.go` |
| B3 | Process start to the `ready` log line, polled every 5 ms. | `perf_test.go` |
| B4 | CPU of the process tree over 30 s with the palette hidden; PSS summed over the tree at the end. | `perf_measure_test.go` |
| B5 | CPU of the process tree over 10 s with the palette shown, focused, query empty. The idle answer afterwards is a diagnostic. | `perf_measure_test.go` |
| B6 | Size of the default build (what ships). The rig build is recorded for information. | `perf_test.go` |
| B7 | Soak loop (summon, type, clear, dismiss) for `-soak`; counters from the `perfdebug` endpoint after each cycle. Goroutine and FD growth are the difference of medians of the last five samples and of five baseline samples taken after three warm-up cycles. Heap growth is the same on post-collection heap; the rising fraction is the share of samples where the heap went up. | `perf_soak_test.go` |

The J29 data set is seeded into the app's isolated HOME before each launch (`perf_seed_test.go`): 5,000 clipboard items and 50,000 files (the app indexed 54,824 files on the box). The 500 applications are not seeded; the rig desktop's own list is what the palette shows, so application-heavy numbers are only in the Go benchmarks.

The app is built with `-tags automation,perfdebug`. The `perfdebug` tag compiles the counters endpoint (`internal/perf/debug_perfdebug.go`), which listens only on a loopback address given in `NVM_PERF_DEBUG`. Default builds link nothing for it.

The renderer in the report is the one the app logs (`OpenGL draws on the CPU (...)`) when it logs one, and `unknown` otherwise.

### Benchmarks

`scripts/rig/bench-summary.sh [benchtime]` runs the app benchmarks (`internal/app/*_bench_test.go`: ranking over 10k, 50k and 100k names; the root search on the real root view for `cl`, `clip` and `show the time`; the clipboard view's filter over 5k items; the file index over 50k paths) and prints a markdown table for a PR. Run it in the box: `scripts/boxgo.sh wp05 'bash scripts/rig/bench-summary.sh 1s'`. The benchmarks use the same fixture generators as the rig, but the application list is 500 fixtures and the stores are the fixture stores, so they measure search, not rendering, the extension host or the clipboard watcher.

### Comparing and the CI note

`cmd/perfcompare` does the comparison; `perf.sh` calls it. A budget with no measurement fails as `MISSING`. For CI, the intended step is `perf.sh --warn-only` on pull requests that touch `src/native` (on a rig runner, not a maintainer's desktop), with the table posted as a PR comment that shows deltas from the baseline stored on `main`. The workflow change is not in WP0.5; it belongs with the rig job in `native-rig.yml`.

### Limits

- Timing comes from xdotool and the automation socket, which add a few milliseconds of latency of their own. B2 is therefore an upper bound, and B1 includes the xdotool spawn as PLAN's "key press" wording implies.
- The rig renders with Mesa software GL under Xvfb (llvmpipe). Frame times are not GPU frame times; B5 is about whether frames stop, which holds regardless.
- Other processes on a shared box change CPU and latency numbers. Run the perf script when the box is quiet, and repeat it before comparing numbers across runs.
- The first hotkey press after `ready` can be lost; B1 repeats it after one second and counts the repeat, so a lost press shows in the percentiles.
- macOS and Windows numbers (the 100 ms B1 budget there) are not measured by this harness.

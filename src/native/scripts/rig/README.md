# scripts/rig: the L4 rig

Runs the real app on a desktop that has what the app needs (Xvfb, openbox, picom, an xfce4-panel tray host, dbus, a PulseAudio virtual microphone), drives it through the MyGo automation plugin and xdotool/xclip, and writes screenshots and logs. Nothing here runs on a maintainer's desktop; CI runs it in the container (`.github/workflows/native-rig.yml`), and the shared test box runs it without Docker.

## Files

| File | What |
| --- | --- |
| `Dockerfile` | Debian trixie image: desktop, input tools, ImageMagick, the GTK/GL runtime MyGo loads, and the Go version from `go.mod`. |
| `entrypoint.sh` | Starts the desktop, runs the command given as arguments, then stops only the PIDs it started. |
| `rig.sh` | Driver: `build`, `run`, `up`, `shot`, `logs`, `down`. |
| `run-journeys.sh` | Builds the app with `-tags automation` and runs the rig tests. Both the container and the box run it. |
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

Only J01, J02 and J05 are scripted, and each one covers the smoke part of its pass condition: no focus restoration, frecency, paste into another app, pinning, or restart durability. The macOS and Windows jobs in `native-rig.yml` launch the app and capture the desktop, and they are `continue-on-error` until hotkey injection and the palette assertions exist there (Accessibility permission on macOS, SendInput on Windows).

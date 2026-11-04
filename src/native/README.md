# Nevermind Go app (2.0, MyGo)

The Go rewrite of Nevermind on [MyGo](https://mygo.egoist.dev): a native command palette with no Electron and no web
view. It ships as **Nevermind Next** beside the Electron app (1.x), with its own bundle identifier, data folder, keychain
entry and update feed. It never reads or writes 1.x data. It is not shipped yet. The current state and gaps are in
[PLAN.md](PLAN.md); the feature-by-feature comparison with 1.x is in
[testdata/parity/parity-review.md](testdata/parity/parity-review.md).

## Features

Working on Linux in the rig. macOS and Windows have compiled but not been run (see Status).

- **Palette.** Summon with the hotkey (default `Alt+Shift+Space`). Results are grouped by provider, cached first and
  refreshed in place. Command-K opens the action menu. Ranking, usage learning and query history.
- **Apps.** Index with icons, launch, running status, Force Quit, Quit All, and an uninstall review.
- **Files.** Index, preview, reveal, open, thumbnails. Find Files by Meaning runs only while signed in.
- **Clipboard.** Text and image history, pin, delete, paste into the app in front, preview. Concealed copies are skipped.
- **Calculator, emoji and symbols, quick notes** (autosave, floating windows), **system commands** (lock, sleep, restart,
  shut down, log out, empty Trash; all behind confirmation), **keyboard shortcuts** (recorder and conflicts), and
  **window management** (X11 engine: halves, thirds, cycling, undo).
- **Account and AI.** Device sign-in, environment switch, BYO key, chat with streaming and tool approvals, AI commands on
  selected text (Fix, Triage, Classify), dictation (Linux capture; macOS and Windows pending).
- **Extensions.** Go source bundles, reviewed before they run and run in a supervised child process. Declarative views,
  forms, editors, menu bar items, background triggers and jobs, and host services (storage, secrets, settings,
  clipboard, desktop, OCR). **Automate** asks the AI to write a bundle, checks it and installs it.
- **Supportability.** Diagnostics page with error references, opt-out technical analytics, the updates service (stable
  and nightly channels, MyGo's signed updater), and the Background Tasks page.

## Extensions

An extension is a folder under `~/.config/nvm-native/extensions/` with `extension.json` (the manifest the host reviews:
capabilities, commands, triggers) and `main.go` (`package main`, importing the `api` package). Dropped-in extensions stay
disabled until reviewed. The API design is in [docs/extension-api.md](docs/extension-api.md); the Go types are in
`internal/ext/api`.

## Build

Go is not installed on maintainers' desktops. Build and test in the shared test box with `scripts/boxgo.sh`, which syncs
this tree and runs the command there. Use a slot named for your work, for example your WP id:

```sh
src/native/scripts/boxgo.sh wp55 'go vet ./... && go build ./...'
```

The module needs Go 1.27 and MyGo v0.7.4 (`go.mod`). Packaged builds use `go tool mygo build` with the release flags; see
[docs/packaging.md](docs/packaging.md). macOS packages need a Mac.

## Test

The validation ladder is in PLAN section 1. Each rung runs where it is safe:

| Rung | What | Where |
|---|---|---|
| L0 | `gofmt`, `go vet`, `staticcheck`, `govulncheck`, layer and cycle test (`internal/layers`), file-size ratchet | CI (`.github/workflows/native.yml`) and `boxgo.sh` |
| L1 | Unit tests with `go test -race` | CI and `boxgo.sh` |
| L2 | View tests with `ui.NewTester` (no window, fake time) | CI and `boxgo.sh` |
| L3 | Goldens, light and dark at 1x and 2x, in `testdata/golden` and `internal/app/testdata/golden`. Update one with `go test -run '^TestName$' -update .` in its package | CI and `boxgo.sh` |
| L4 | The real app in the rig: Xvfb, tray host, dbus, virtual microphone, automation socket (`-tags automation`) | the rig container or the box, never a maintainer's desktop |
| L5, L6 | macOS and Windows runners | CI scaffolds (`native-rig.yml`); not yet driving the palette |
| L7 | Installed packages, sign-in against staging, updates from the previous build | release candidates only |

Rig journeys run on the box with a private display, so they do not collide with other sessions:

```sh
src/native/scripts/boxgo.sh wp55 'RIG_DISPLAY=:95 RIG_ARTIFACTS=/root/wp/wp55-logs NVM_RIG_BIN=/root/wp/wp55/nvm-rig NVM_RIG_ARTIFACTS=/root/wp/wp55-artifacts bash scripts/rig/entrypoint.sh bash scripts/rig/run-journeys.sh -run Journey -v'
```

The journeys are in `internal/rigtest/journeys_test.go`. The helpers, the Docker path and the known flakiness are in
[scripts/rig/README.md](scripts/rig/README.md). Artifacts (palette and desktop screenshots, app logs) go to
`$RIG_ARTIFACTS`. Budgets B1-B8 are in `testdata/budgets.json`; the last measured baseline is in
`testdata/perf-baseline.json`.

## Status

- **Verified on Linux:** L0-L3 for every feature in the list above. Rig journeys for the palette, shortcuts, apps,
  clipboard, extensions, tray, X11 windows and AI commands. Account sign-in has L1-L3 coverage only.
- **Compiled, not run:** the macOS and Windows builds of `cmd/nvm`.
- **Not verified anywhere yet:** macOS and Windows runtime behavior (L5, L6), published updates (L7), live backend
  sign-in and streaming, and the macOS and Windows dictation and window-management backends.
- **Known gaps:** the Gap column of the parity sheet lists them. The largest are the 1.x built-ins not yet moved to
  extensions (WP3.10: one of five), the extension chat and progress views, the extension dictation and updates services,
  and macOS window management.

## Layout

```
cmd/nvm/          composition root: builds the platform ports and starts the app
internal/
  platform/       ports (interfaces) and per-OS implementations (real, fake)
  store/          versioned JSON stores: atomic writes, backups, quarantine, leases
  logx/ safego/   logging with redaction; panic isolation
  design/         tokens, depth, components, icons, golden harness
  search/         palette provider interface, coordinator, ranking, usage
  features/       apps calc clipboard emoji files notes snippets system settings ocr
  backend/        typed client for the Nevermind backend, sign-in, compatibility
  ai/             chat, agent loop, tools, commands, builder
  ext/            extension API, bundles, host, child process, services, built-in host
  palette/        palette shell, views, extension views, account, updates, diagnostics
  jobs/ shortcuts/ windowmgmt/ dictation/ update/ diag/ menubar/
  app/            app wiring; it still holds older features, and new code goes in the packages above
  layers/         layer order and the file-size ratchet tests
  fixtures/       deterministic data and the fake backend
  rigtest/        rig helpers (build tag rig)
testdata/         goldens, reference screenshots, budgets, contracts, parity sheet
scripts/          boxgo.sh, rig/, packaging/, natives/
packaging/        icons and entitlements
docs/             design notes
```

## Docs

| Document | What it covers |
|---|---|
| [PLAN.md](PLAN.md) | Work packages, validation ladder, UX protocols, milestones, readiness |
| [docs/cutover.md](docs/cutover.md) | Moving users from 1.x to 2.0: install, scheme, data, backend, channels, rollback, owner decisions |
| [docs/release-checklist.md](docs/release-checklist.md) | Ordered steps from a version bump to a published release |
| [docs/release.md](docs/release.md) | The release run, feeds, update keys, rollback |
| [docs/packaging.md](docs/packaging.md) | What each package contains, signing and notarization, release flags |
| [docs/extension-api.md](docs/extension-api.md) | Extension API design (WP3.1) |
| [docs/extension-runtime-spike.md](docs/extension-runtime-spike.md) | Runtime numbers and the D1 decision |
| [docs/agent-loop-spike.md](docs/agent-loop-spike.md) | Agent loop spike and the D2 decision |
| [docs/mygo-practices.md](docs/mygo-practices.md) | What the open-source MyGo apps do and what that changes in the plan |
| [scripts/rig/README.md](scripts/rig/README.md) | The L4 rig: Docker, the box, helpers, flakiness |
| [scripts/reference-capture.md](scripts/reference-capture.md) | How the Electron reference screenshots are made |
| [internal/store/MIGRATING.md](internal/store/MIGRATING.md) | Moving each store onto `internal/store`: file names and schema versions |
| [testdata/parity/parity-review.md](testdata/parity/parity-review.md) | Feature-by-feature parity with 1.x |
| [../../docs/backend-api-compatibility.md](../../docs/backend-api-compatibility.md) | Backend rules for every desktop client, this one included |

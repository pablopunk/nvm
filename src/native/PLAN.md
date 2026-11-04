# Plan: rewrite Nevermind in Go, safely

Companion to the parity checklist in the PR description (#280). The checklist says **what** is left; this says **how to do it without breaking things**: the validation ladder, the UX and UI validation protocols, the safety rules, and the work packages (WPs), each sized and written so one agent can take it with no other context.

If you are an agent picking up a WP: read sections 0 to 4, then your WP, then only the files its "Read first" lists. Do not widen scope.

---

## 0. Principles

1. **Electron keeps shipping until cutover.** The Go app lives in `src/native`, ships as a separate channel (`next`) with its own bundle id and data directory, and is promoted only at WP5.6. Cutover is a release decision. There are no feature flags (`CLAUDE.md`); the only switch we allow is an emergency kill switch.
2. **The Go app never touches 1.x data.** It uses its own directories (config `~/.config/nvm-native` for now, final name decided at WP5.5) and never reads or writes `state.json`, `~/.config/nevermind/extensions`, or the Electron keychain entries. A read-only importer is optional and comes last.
3. **No claim without evidence.** Every WP closes with the validation artifacts listed in section 1 attached to its PR: test output, golden diffs, screenshots from the rig, budget numbers. "Verified on macOS" means a macOS run, not a Linux run.
4. **Small, independently mergeable slices.** A WP merges alone, leaves `main` of the Go tree building and green, and ticks its boxes in the checklist.
5. **Ports and fakes first.** Anything that touches the OS, network, clock, disk or a model goes through an interface in `internal/platform` so tests are deterministic (WP0.2). No test sleeps; time is advanced (`ui.Tester.Advance`).
6. **Data safety beats features.** Persistence goes through the one store from WP0.6 (atomic, versioned, backed up). Nothing deletes user data on an error path.
7. **Palette-first UX** (`CLAUDE.md`, `docs/palette-first-extension-ui.md`, `docs/list-first-extension-ui.md`): no settings windows, no confirm dialogs, short input and choices in the palette, show cached snapshots first and refresh in place, never block first paint or typing on enrichment.
8. **Capabilities, not platform branches** (`docs/os-architecture.md`): OS differences live in `internal/platform` behind intent-named capabilities; unsupported capabilities are omitted, not shown disabled.
9. **Dogfood the extension API.** Built-ins end up as extensions on the same API (WP3.10). Until then each built-in implements the `search.Provider` interface from WP1.1, shaped to map 1:1 onto the API.

### Decisions and the defaults agents should assume

| # | Decision | Default until the owner says otherwise | Affects |
|---|---|---|---|
| D1 | Extension runtime | **Spiked (WP3.1, `docs/extension-runtime-spike.md`):** Yaegi-interpreted Go in a supervised child process (per-call deadline 20/100 ms, kill and restart, address-space cap); sobek JS is the fallback behind the same protocol; in-process Yaegi cannot be interrupted or memory-capped. Final runtime choice at the author-experience gate | WP3.* |
| D2 | AI agent loop | **Spiked (WP2.4a, `docs/agent-loop-spike.md`):** native Go loop behind `ai.Agent`; the proxy forwards tools and tool-call deltas; owner must check each tier's `api` via `GET /api/v1/active-model` (Responses-format tiers need an adapter); pi sidecar stays a builder-only fallback; pi-in-JS-runtime rejected | WP2.4, WP3.9 |
| D3 | Backend | Unchanged TypeScript/Astro; Go is a client | WP2.* |
| D4 | macOS helper | Reuse `src/app/resources/macos-helper/*.swift` as a child process, same CLI contract | WP1.6, WP2.5, WP4.1 |
| D5 | 1.x state | No migration | WP0.6, WP5.5 |
| D6 | Architecture order | Design the extension API early (WP3.1) and keep providers shaped for it; migrate built-ins at WP3.10 | WP1.*, WP3.* |

### Research addendum (from `docs/mygo-practices.md`, read it before any WP)

`src/native/docs/mygo-practices.md` summarizes what the open-source MyGo apps on the showcase do. Findings that change this plan:

- **Dictation (WP2.6):** MyGo's `audio` package is output only and its permission API covers web pages only, so microphone capture needs a native path (a helper or purego bindings to AVFoundation / WASAPI / PulseAudio). Size goes up and a spike comes first.
- **Agent and extension runtimes (D1, D2):** two shipped apps embed pure-Go JS engines (Godwit runs its pi agent in `moejs`, mixmusic runs plugins in `sobek`); none uses Yaegi. Both are options for the WP2.4a and WP3.1 spikes, next to the native Go loop and the Node sidecar.
- **Rig (WP0.3):** MyGo's automation socket drives built apps by element key, and MyGo's own CI runs GUI tests on GitHub's hosted macOS and Windows runners, so a self-hosted runner may not be needed for L5/L6 at first (to verify).
- **Linux (WP5.4):** `linux.webview: false` removes the WebKitGTK dependency for native-only apps; the tray needs `libayatana-appindicator3`; Wayland global shortcuts need the XDG portal.
- **Clipboard (WP1.6):** MyGo has no change event and no concealed-type handling; we own both.
- **Persistence (WP0.6):** Godwit's atomic write and flock lease and mixmusic's "never overwrite a file you cannot decode" store are the references.
- **Signing (WP5.1):** bundled native libraries must be signed for the hardened runtime and pinned by digest.
- **Testing:** adopt cloudmusic's `AGENTS.md` test rules (test only what can break, no wall clock).

---

## Progress

| WP | Status | Notes |
|---|---|---|
| WP0.1 Workspace, layering, CI | done | single `internal/app` package for the prototype; layer and size guards in `internal/layers` |
| WP0.2 Ports, fakes, fixtures | done | `internal/platform`, `platform/fake`, `fixtures`; prototype not migrated yet |
| WP0.3 Rig | done (Linux) | J01, J02, J05 pass in the box and in Docker; macOS/Windows jobs are scaffolds |
| WP0.4 Goldens and contrast | done | 11 contrast waivers await an owner decision |
| WP0.5 Perf harness | done | budgets ratified and tightened (B1 60 ms Linux, B2 8 ms, B3 400 ms, B4 100 MB, B6 45 MB); prototype passes all |
| WP0.6 Store | done | prototype not migrated; lease takeover semantics await review |
| WP0.7 Logging, safego, diag | done | prototype goroutines not converted |
| WP0.8 Design system | done | prototype now renders through `internal/design`; goldens pixel-identical |
| WP1.9 Notes and snippets | ready for review (Linux, L1-L2, L4 smokes) | `features/notes` and `features/snippets` on `internal/store`; 15 of 15 ported 1.x test cases; crash-recovery and two-instance tests; floating windows restored on displays that are connected; J09 and J10 smokes green in the rig on `:86`; real paste is not implemented (copy fallback); macOS and Windows not run |
| WP1.11 Keyboard shortcuts | ready for review (Linux) | Summon (WP1.3) binds the palette shortcut, recovers from conflicts and drives the banner; `internal/shortcuts` Manager and Document own the other shortcuts and the palette shortcut's stored value in shortcuts.json (canonical form, labels, binding plan with conflict table, per-action choices with conflict rules, registration lifecycle, suspend while recording). The Keyboard Shortcuts list is a root command, and the recorder is the only UI that changes the palette shortcut (its saves go through Summon.Change, a taken key restores the old one and shows the conflict toast). Root table rows added in queries-expected.json and queries-learned-expected.json. L1 and L2 green; goldens: shortcuts-list, shortcuts-recorder and shortcuts-recorded added. Rig J01, J11 (both halves) and J22 green, and all other journeys green on the earlier tree. Not done: extension-declared shortcuts (no extension host in Go yet; the model is in place), J11 launch-at-login, macOS and Windows run (L5/L6; cross-compiles only), Spotlight and Linux recovery in the rig only where WP1.3 covers them |
| WP1.12 Menu bar, diagnostics, analytics, updates stub | ready for review (Linux: L1, L2, goldens, rig tray registration; macOS and Windows: compile only) | `internal/menubar` (Open, Check for Updates, Settings, Quit in the 1.x order) over `platform.TrayHost`, with the icon and timer title; `palette/diagnostics` Recent Errors page (copy reference, copy safe diagnostics, reporting label) and Report a Problem over `internal/diag`; `internal/diag/analytics`: consent switch "Send Technical Diagnostics" (default on, dropped at send time when off), typed events with per-event key allow-lists and a schema test, canary redaction, bounded offline queue, 3 tries on 5xx, endpoint only in release builds; `internal/update` port with `NoUpdates` and a fake; log file under the config folder. Rig J-tray: registration and the four menu rows pass; choosing a row is not verified (the D-Bus click never reached the item). Not done: performance traces into WP0.5, `$pageview` and sessions, owner events, extension trays (WP3.5), the update-available row (WP5.2), J25 and J28 in the rig, live ingest smoke, macOS and Windows runs |
| WP3.7 Context services I: storage, secrets, cache, settings, logs, text, data | ready for review (Linux: L1 and `-race` on `internal/ext`, `go vet` and `go test` over the Go packages (explicit list, see notes), darwin and windows builds of `cmd/nvm`; L2 not applicable, no view yet; rig J15 and J16 run on :78, see the PR) | Host services in `internal/ext/services` (one handler per extension; namespaced store folder, keychain for secrets, declared settings, logs stamped with the id, text templates, public-address network with manual redirects, data source cache with stale fallback); the child calls them over a service exchange (`ServiceCall`/`ServiceReply`) inside the request budget; `network`, `secrets` and `settings.write` are enforced, undeclared use is refused with a named capability; quotas per service; `acme.services` and `acme.hostile` fixtures run through the child process. Not done: settings view goldens (WP3.3/3.4), Drafts (WP3.4), view patching of data sources (WP3.3), desktop, clipboard history, OCR, dictation, shortcuts, indicators, system, updates, AI and windows services (WP3.8, WP3.9); macOS and Windows runs; keychain on the box is the fake port (MyGo secret-service not exercised) |
| WP4.1a Window geometry | done (Linux, L1) | `internal/windowmgmt/geometry`: pure layouts, gaps, cycling, exact values, neighbors, display moves, verification; goldens replay the TypeScript engine; engine (4.1b) done pending L5, UI (4.1c) pending |
| WP1.1 Palette core | done | `internal/search` + `internal/palette`; root providers in `providers_wiring.go`; 52/60 frames strict-identical, 8 identical to base render |
| WP1.2 Usage learning, query history | done | `search/usage`, history in palette; rig re-verified after merge |
| WP1.3 Panel window and hotkey per OS | ready for review (Linux: L1, L2, L3, L4 J01 and J11; macOS and Windows: compile and fakes only, L5/L6 pending) | `shortcuts.Summon` (bind, Linux recovery list, 8 retries at 400 ms on the clock, all-or-nothing change); `palette.PlaceOnWorkArea` (Electron placement on the cursor display); palette notice banner for unset or unavailable shortcut; hotkey view saves through the registrar; palette opens at launch when the shortcut stays unbound; `window.placement` capability (off on Wayland). Not done: remembered offset (Electron recenters on every show, so there is nothing to port; owner decision), saving a taken shortcut in the rig, J25/J26 (multi-display, full-screen, Spaces), focus return to the previous app, AeroSpace check, and the palette hotkey not hiding a shown palette (also in the baseline) |
| WP1.4 Apps (Linux) | in progress | `features/apps`: index, icons, running status, quit, uninstall library, and palette wiring with Force Quit Apps, Quit All Apps and the uninstall review. Linux J22 passes on the rig. Not done: app icons still use the prototype's synchronous icon path, the uninstall review and confirm goldens, the palette L2 tests, macOS and Windows discovery (macOS is ported and tested with fakes only; Windows is unavailable) |
| WP1.6 Clipboard engine | WP1.6b wired; ready for review (Linux) | engine in the app on the one shared store (history, watcher, paste through the palette-hide wait, link previews over the public port, file-list copies); read-only instances keep it in memory and do not watch; Linux box: `go vet` and `go test ./...` green, rig J01, J02, J03, J05, J06, J08, J09, J10, J22 green on a private display; not verified: paste into another app on the rig, live link fetch, macOS paste and concealed markers (untested), Windows paste (unsupported) |
| WP1.7 File index engine | done (library) | wiring, real watchers, previews pending (WP1.7b) |
| WP1.7b Files wiring | partial (Linux; L1, L2, box checks) | index at launch, root and Files view, previews, thumbnails, icons, drag-out (untested at runtime); open-with, Quick Look, folders, rig J07 not done; macOS/Windows watch unverified |
| WP2.1 Backend client | done | live smoke not run; `/api/me` and `/api/v1/models` findings open |
| WP2.4a Agent loop spike | done | D2: native Go loop; owner checks tier `api` via `active-model` |
| WP3.1 Extension API + runtime spike | done | D1: supervised child process; Go/Yaegi provisional |
| WP3.2 Host runtime: loader, trust, isolation, errors | partial (Linux: L1, L2 and L3 goldens, rig J15 and J16 green on :88; macOS and Windows not run) | `internal/ext/{bundle,install,enablement,protocol,child,supervisor,manager}`; extension source imports the typed `api` package through a generated Yaegi table (ratchet exempts generated files); the app runs extensions through the manager (root rows, commands, failure rows, AI builder checks the child); manager list and capability review with goldens; fixtures good/slow/crash/spin/panic/bomb/huge/flood/init/syntax. Not done: macOS/Windows memory cap (resident watch only), macOS/Windows runs, menu-bar extensions and HTTPGet are not in the new API, memory-bomb and three-failure disable paths not rig-tested, a file dropped while the app runs not rig-tested, 100 ms keystroke budget not measured |
| WP4.1a Window geometry | done | 324/331 TS cases + 4,120 golden expectations from the TS engine |
| WP4.1b Engine | done pending L5 | `internal/windowmgmt` over `platform.WindowManager`: verified placement (exact or partial, never success unless reached), per-process queue, history for cycle/restore/undo, opaque expiring handles, Electron failure taxonomy, access checks, focus rules. Linux X11 backend (xprop/xwininfo/xrandr/xdotool) passes `scripts/rig/window-x11.sh` on the box (left/right halves and restore, screenshot kept). Not verified: macOS helper backend (compiles for darwin, L5 pending); X11 full screen (unsupported: no installed tool sends the EWMH message); Windows has no backend (capability omitted) |
| WP4.1c UI and shortcuts | ready for review (Linux: L1, L2, goldens, `-race`, darwin and windows builds, rig J01, J11, J19 (palette and shortcut) on X11 green; macOS and Windows not verified) | `windowmgmt/commands` (catalog with stable `window.*` IDs, runner, all layouts, focus moves, explicit layouts for exact placements), `palette/windows` (Switch Window list with Focus and layout actions, Window Management Settings with number input, Move Window and Resize Window forms as one palette field, Custom Window Commands pointer), window commands and the layouts in the Keyboard Shortcuts list through the WP1.11 manager (bind by command ID; a shortcut acts on the app in front and never takes focus), window entries in the existing command list (phrase and word-prefix matching only, so the root is not flooded). Focus: one capture, the palette Focus port (Active on show, WP1.3); window commands read it through `Palette.Summoned` and restore through `Focus.Restore`, so no second capture runs on X11. The engine checks ownership by PID, the key the Focus port uses. macOS has no focus port, so it reads the frontmost app on show (one helper call, capped at 150 ms, not measured). Parity: only additions (`set` shows Window Management Settings and Move Window; `cal` shows Maximize Height through its "vertical" alias). Goldens: 9 window views, the Keyboard Shortcuts list (window rows added). Not done: extension API `windowManagement`, next and previous desktop (macOS AppleScript), macOS L5 for J19, Windows backend, sync of the palette Active capture off the first paint (it must run before the palette takes the keyboard) |
| WP1.5 Calculator and rates | ready for review | `features/calc` behind the root result row: 1.x actions and order, detail pane, `rates.json` through `internal/store`, background refresh, cache age in the subtitle. L1 and L2 green in the box; rig J03 smoke green. Not verified: macOS and Windows, live rate source |
| WP1.8 Emoji, symbols, Unicode | ready for review | catalog grid (`palette.Grid`, virtualized), categories with Tab, skin tone from the action menu (remembered in settings), recents in `emoji-recents.json`, Copy, Paste, Copy Unicode, keyboard grid with cell labels and announcements. L2 and goldens green; rig J08 smoke green. Not verified: macOS and Windows |
| WP1.* onward | in progress | M0 gate passed by owner direction to continue autonomously; open owner items: contrast waivers, lease takeover semantics |
| WP2.1 Backend client | ready for review | `internal/backend`: fake-backed and recorded-contract tests green under `-race`; live smoke (`-tags livesmoke`) written, not run. `/api/me` is cookie-session only and `GET /api/v1/models` is not served by the backend source; both need backend confirmation before the live smoke can pass |
| WP2.2 Sign-in, keychain, deep links, account view | ready for review (Linux: L1, L2, L3 goldens, `-race`, darwin and windows builds; L4-L6 and live sign-in not run) | `internal/backend/auth` (device and `nvm://` sign-in, one keychain token per origin, sign-out with revocation first, environment switch, BYO key, spend-cap notice), `internal/palette/account` (44 goldens), `platform.DeepLinks` port, prototype `auth.json` replaced. Backend `/api/me` is cookie-only, so plan shows only if the backend accepts bearer tokens; macOS deep links need `urlSchemes` in mygo.config.ts (WP5.x) |
| WP2.6 Dictation | ready for review (Linux: L1, L2, goldens, `-race`, and the virtual-mic check on a private PulseAudio; macOS and Windows compile only; L4 app journey and L5/L6 not run) | `internal/dictation`: one dictation at a time (start after the microphone opens, stop transcribes 60 s WebM/Opus segments in order, cancel, prepare-before-stop, failure copy). `internal/dictation/webm`: Opus-in-WebM muxer that the backend's route decodes. `internal/backend`: `Transcribe`, `PrepareTranscription`, `CancelTranscriptionPreparation` on `/api/v1/audio/transcriptions` and `transcription-preparations`. Ports: `platform.Microphone` (Linux pactl and parec; `microphone.capture` capability) and `platform.OpusCodec` (libopus via purego on Linux and macOS, unsupported on Windows). `palette/dictation` indicator: six states, goldens light and dark at 1x and 2x. Root command `Dictate` (shortcut-capable through the shortcuts manager's command list; rows added to both root tables). Paste through `Paster` after the palette hides, clipboard restored after 500 ms. Not done: macOS and Windows capture, libopus bundling (WP5.1), device picker and Dictation settings, history and recordings with retention, vocabulary terms, AI cleanup and screen context, preparation cancel on cancelled dictations, permission flow, live backend smoke, the rig journey J20 (only the capture, Opus and WebM layers are checked against a virtual microphone) |
| WP2.3 AI chat core and UI | partial (Linux: L1, L2, L3 goldens, `-race`, merged with WP3.2 at 117a1703; L4 rig not run) | `internal/ai/chat` (chats in `chats.json` schema v1; tiers fast and smart resolved by active-model and sent as `X-Nevermind-AI-Model`; copy-on-write chat cache so a read never copies the store, 755 ns per read at 500 chats; stream reader keeps the text of a broken reply as cut off; stop, regenerate, 24 h conversation retention; a notice for every refusal, spend cap noted on the account; model routes re-read on sign-in and sign-out), `internal/palette/aichat` (Markdown transcript, notice cards, model pill, Escape, ⌘R, ⌘⌥C, ⌘⇧M, chat list; 36 goldens rendered with `design.WindowTheme`). The extension builder is a smart-tier chat; install and load check stay on the WP3.2 manager. Not done: rig J13 and J28 (packaged builds refuse the fake backend origin and the rig has no backend override), image attachments and thumbnails, per-message copy and retry buttons, macOS and Windows runs, live backend smoke |

## 1. The validation ladder

Every WP states which rungs it must pass. Lower rungs are cheap and run on every commit.

| Rung | What | Where it runs | Tooling |
|---|---|---|---|
| L0 Static | `gofmt`, `go vet`, `staticcheck`, `govulncheck`, layer/cycle check, file-size ratchet | every PR, Linux | WP0.1 |
| L1 Unit | pure logic, stores, parsers, ranking, geometry, protocol clients with fakes; `go test -race` | every PR, Linux | plain `go test` |
| L2 View tests | whole views driven without a window: type, click, keys, `Announcements`, focus, light/dark, scales, no-sleep time | every PR, Linux | `ui.NewTester` |
| L3 Goldens | deterministic PNG renders of every view and component in light/dark at 1x/2x with fixtures; pixel diff with tolerance; contrast math on tokens | every PR, Linux | `ui.Render`, WP0.4 |
| L4 Rig | the real app in a container: Xvfb, compositor, tray host, dbus, virtual mic; hotkey, tray, clipboard, windows, screenshots, perf numbers | every PR for changed areas, nightly full | WP0.3, `plugins/automation` + `xdotool` |
| L5 macOS | the real app on a macOS runner with Accessibility pre-granted: glass material, `Cmd` shortcuts, window management, helper, signing | nightly + before milestone exit; `macos-selected-text.yml` already exists as a model | WP0.3 |
| L6 Windows | same on a Windows runner | nightly + milestone exit | WP0.3 |
| L7 Packaged | install the built artifact, launch, sign in against a staging backend, update from N-1 | release candidates | WP5.1, WP5.2 |
| L8 Humans | the UX protocol in section 3: dogfood gates and scripted sessions | milestone exit | section 3 |

Rules:
- A WP cannot be marked done if any rung it lists was skipped. If a rung is unavailable (say no macOS runner yet), the WP is "done pending L5" and the box stays unticked with a note.
- L4 to L7 failures attach the screenshot, the log and the seed/fixture id to the PR. Flaky is a bug in the harness, fixed first.
- Repo rule `CLAUDE.md` ("CI-only runtime verification") was written for the Electron app. For the Go app, **runtime verification happens only in the rig container or CI, never on a maintainer's desktop**: L0-L3 are safe anywhere (`ui.NewTester` opens no window); L4+ launches run in the container. WP0.1 writes this into `AGENTS.md`.

### Fixtures

One package, `internal/fixtures`, owns deterministic data: fake apps with icons, 5k files in a virtual tree, 200 clipboard entries including images and concealed ones, chats with streamed transcripts, an extension set (good, slow, crashing, hostile), a fake backend with recorded responses and failure modes (401, 402 insufficient credits, 429, 5xx, timeout, malformed SSE). Every view must be renderable from fixtures alone; this is the Go equivalent of `docs/extension-api-ui-fixtures.md` and is required for any API surface that renders UI.

---

## 2. UI validation protocol

Goal: the product looks like Nevermind (warm paper and gold, ink and gold in dark; flat fills with a lit top edge and soft shadow; no gradients beyond the faint gold key) and feels native on each OS.

**Reference.** WP0.4 captures reference sheets of the Electron palette (the repo already has `report-linux-palette-screenshot.yml` and `tests/electron/palette.smoke.spec.ts`; reuse them) and stores them under `testdata/reference/`. References are for human review, not pixel diffing across toolkits. Our own goldens are the regression net.

**Per view or component, a PR must include:**
1. Golden PNGs: light and dark, 1x and 2x, plus the states that matter (empty, loading, error, long text, selected, hovered via tester, focus ring, disabled). Fixtures only, no clock or random.
2. A review sheet entry (`testdata/golden/<wp>/README.md`): screenshot grid plus the checklist below, ticked by the author, re-checked by the reviewer.
3. Rig screenshots for L4/L5/L6 where the WP touches windowing, material, tray or popover behavior.

**Checklist (every view):**
- Tokens only: colors, radii, shadows come from `internal/design`; no literals in feature code (a test greps for hex literals outside `design`).
- Type scale: 21 search, 13 semibold titles, 11 subtitles and sections; single line with ellipsis, never clipped mid-glyph; numerals tabular where aligned.
- Spacing on a 4 px grid; row height constant across providers; icon tile 32; keycap and chip heights consistent.
- Selection: warm plate with faint gold edge; prominent rows use the accent tint; hover differs from selection; focus ring visible for keyboard focus only.
- Contrast: text and icons meet WCAG AA on every surface in both themes (computed in a test from tokens; fails the build).
- Depth: lit top edge on raised surfaces, soft shadow below, grooves for dividers; nothing floats without a shadow, nothing has a shadow it does not need.
- Corners: the window cut, the pane border and the clip agree in light, dark, with and without material. (Regression: dark corners once rendered square.)
- Material: with `VibrancyGlass` the pane shows no opaque fill; without it the fallback is the see-through paper/ink pane. Both captured.
- Motion: respects reduced motion; no animation blocks input.
- Density: palette 720 wide, 470 tall; content never reflows the window; long lists scroll with hidden scrollbars.
- RTL and 200% text smoke test for the shell and one list view (tester `SetDirection`, `SetZoom`).
- Icons from the Lucide set via `design.Icon(name)`; app and file icons are system icons.

**Per platform sign-off** (milestone exit): a human reviews the platform's screenshot sheet next to the Electron reference and records "ok" or the diffs found in the PR. Required platforms by milestone: macOS from M1, Windows and Linux from M5 (earlier if a WP touches them).

---

## 3. UX validation protocol

### 3.1 Journeys (the product contract)

Each journey is a scripted scenario with a pass condition and a time budget. L2 automates the logic, L4 to L6 drive the real app, L8 is a human doing it. A WP lists the journeys it can break; they run before merge.

| ID | Journey | Pass condition |
|---|---|---|
| J01 | Summon with the hotkey, dismiss with Esc, click away, hotkey again | opens on the cursor's display over full-screen apps, previous app keeps focus and gets it back, state resets |
| J02 | Type 2-3 letters, Enter launches an app; frecency learns | the intended app is top hit after 3 launches |
| J03 | Calculator, units, percentages, currency | right answer, copy and paste actions, offline rates labeled stale |
| J04 | URL and web search fallback | `Open <site>` for URLs; web search and Automate otherwise; Enter opens default browser |
| J05 | Clipboard: copy 3 items and an image, find the old one, paste into the frontmost app, pin, delete | pasted text lands in the app that had focus; nothing lost across restart |
| J06 | Concealed copy from a password manager | never recorded, never shown |
| J07 | File find, preview, reveal, open, drag out | results in the first frame from cache, refreshed in place |
| J08 | Emoji and symbol paste with skin tone, by name and by Unicode | correct glyph pasted |
| J09 | Snippet create, find by keyword, paste | pasted text matches, edit and delete work |
| J10 | Note create, float, restart the app, crash it mid-edit | text recovered; conflict prompt only when versions differ |
| J11 | Change the hotkey (including a taken one), launch at login | conflict explained in the palette; old hotkey restored on failure |
| J12 | Sign in through the device flow, sign out, repeat offline | clear states for pending, success, failure; no token in logs |
| J13 | Tab to ask AI, stream, stop, continue the conversation tomorrow | first token latency shown as progress; history intact |
| J14 | Automate: type a request, model writes the extension, check/fix loop, install, use, tray item, delete; failure path | works first try or fails with an explanation and no half-installed file |
| J15 | An extension panics, hangs or floods results | palette stays responsive; extension is marked failed with a reason; others unaffected |
| J16 | Drop a file in the extensions folder | disabled by default; palette shows what it declares; enable after review |
| J17 | Menu bar extension refresh and click | title updates on schedule; click acts; no leak after disable |
| J18 | Trigger extension (`files.changed`, `clipboard.changed`) | fires once per change, debounced, never overlaps itself |
| J19 | Window management: halves, thirds, cycling, undo, next display (macOS) | frame reached or honest partial/failed message; never steals focus |
| J20 | Dictation: start, speak, stop, paste; no permission; no microphone | text pasted; permission and device errors explained inline |
| J21 | Fix selected text with AI in another app | minimal edit applied or unchanged; indicator shows progress |
| J22 | Quit and force quit apps; uninstall an app | review list before Trash; nothing outside the app's own files without explicit confirmation |
| J23 | Lock, sleep, restart, shut down, empty trash | confirmations for destructive ones, in the palette |
| J24 | Update: check, download, restart; switch channel; downgrade guard | state kept; rollback possible |
| J25 | First run: Getting Started, permissions only when needed, hotkey notice | no prompt before a command needs it |
| J26 | Multiple displays, spaces, full-screen apps | appears where the user is looking |
| J27 | Keyboard-only session and screen reader pass | every action reachable; names announced; focus never stranded |
| J28 | Offline, backend down, token expired, credits exhausted, rate limited | each has a distinct, actionable message; local features unaffected |
| J29 | Scale: 50k files, 5k clipboard items, 500 chats, 100 extension items per keystroke | within the budgets below, no jank |
| J30 | Quit, relaunch, kill -9, corrupt a store file | state intact; corrupt file quarantined with a visible, non-blocking notice |

### 3.2 Budgets (proposed; WP0.5 measures and the owner ratifies)

| ID | Budget |
|---|---|
| B1 | Hotkey to first frame, p95 ≤ 50 ms (macOS and Windows), ≤ 60 ms Linux |
| B2 | Keystroke to updated results frame, p95 ≤ 8 ms and p99 ≤ 16 ms with the J29 data set; provider work off the UI thread, results streamed in |
| B3 | Cold start to ready ≤ 400 ms; palette usable before indexes finish |
| B4 | Idle hidden: RSS (PSS on Linux) ≤ 100 MB with all built-ins; CPU ≤ 0.3% of one core |
| B5 | Palette shown idle: no continuous repaint (frame requests stop) |
| B6 | Binary ≤ 45 MB; installer size tracked per release |
| B7 | No goroutine, FD or memory growth over a 30-minute J29 soak |
| B8 | Extension call budget: `Search` 20 ms soft / 100 ms hard per call, enforced by the host |

### 3.3 Human validation

- **Scripted session** at each milestone exit: the owner (and one other person when available) runs J01-J30 relevant to the milestone on each supported OS from a checklist in the PR; every failure becomes an issue with the journey id.
- **Dogfood gate:** the owner uses the Go build as the only launcher for 5 working days before each milestone exit. Crashes, wrong results and "I had to go back to 1.x" moments are logged with a one-line cause. Exit needs zero data-loss bugs and zero P1s open.
- **Parity review:** side by side with the Electron app, per feature in the checklist: same results order for 20 canned queries (`testdata/queries.txt`), same actions, same shortcuts, differences listed and accepted or fixed.
- **Copy review:** every user-facing string follows the product voice (specific, active, says how to fix errors). Reviewed in the PR.

---

## 4. Safety and robustness rules

**Data.** One store package (WP0.6): write to temp, fsync, rename; schema version in every file; keep the last N good copies; on parse failure quarantine the file (rename `*.corrupt-<ts>`), start from defaults, and show a non-blocking notice with the path. Files are `0600` where they hold anything private. A lease prevents two instances from writing the same store. Deletions of user data (clipboard, notes, chats, files, apps) require an explicit user action; background jobs only expire what retention settings say.

**Secrets.** Tokens and extension secrets live in the OS keychain (MyGo `Secrets`); never in JSON, logs, crash reports or analytics; redaction is tested with canary strings.

**Concurrency.** UI state is touched only on the UI thread; goroutines hand results back with `Window.Update`. `go test -race` is mandatory. Long work takes a `context.Context` and is cancelled when the query changes or the view closes.

**Failure isolation.** Every provider, extension call, trigger, job and helper process has a timeout, a panic guard (`safego.Run`) and a bounded output size. A failing component is disabled with a reason shown in the palette, not retried in a loop. Helper binaries are killed on timeout and their stdout is size-limited.

**Network.** Per-request timeouts, cancellation, jittered retries only for idempotent calls, a global offline state, TLS defaults untouched, the compatibility headers on every backend call, and one place (`internal/backend`) where URLs are built.

**Extensions.** Dropped-in code is disabled by default (`docs/extension-trust-model.md`); capabilities are declared and reviewed, not enforced as a sandbox, so the host assumes extensions can misbehave and protects the palette (timeouts, panics, memory watch, kill on repeat offenders). AI-generated code is validated by the interpreter before it is written next to the others, and written atomically.

**Privacy.** Analytics are opt-in or documented opt-out exactly as in `docs/posthog-observability.md`; no screen content, no clipboard content, no file names in diagnostics; error references carry ids only.

**Supply chain.** `go.sum` pinned, `govulncheck` in CI, new dependencies need a line in the PR (why, license, size). MyGo is pinned to a version; any local patch is upstreamed or vendored with a note. Yaegi is checked against the Go version we ship at every Go bump.

**Observability.** File-backed logs (the `.tmp/dev.log` convention), levels, ring buffer exposed in the Diagnostics view, error references, performance traces behind the same budgets as section 3.2.

**Rollback.** Every release candidate can be reverted to the previous Go build without data migration because stores are versioned and forward-compatible by one version; each WP that changes a store schema documents the downgrade behavior.

---

## 5. Repository layout and layering (WP0.1 creates it)

```
src/native/
  cmd/nvm/                 main: composition root only (wires factories)
  internal/
    platform/              ports (interfaces) + per-OS implementations (build tags)
    store/ logx/ safego/   persistence, logging, panic isolation
    design/                tokens, depth helpers, components, icons
    search/                coordinator, ranking, Provider interface
    features/              apps calc clipboard files emoji notes snippets system settings ...
    backend/               typed client for /api/v1, auth, me, compatibility
    ai/                    chat sessions, agent loop, tools
    ext/                   API types, host, runtime (Yaegi), views, jobs
    palette/               shell, navigation, actions, window
    jobs/ shortcuts/ windowmgmt/ dictation/ update/ diag/
    fixtures/              deterministic data and fakes
  testdata/                goldens, references, queries
  scripts/rig/             container image and drivers
```

Layer order, lowest first: `platform < store, logx, safego < design < search < features < backend < ai, ext < jobs, shortcuts, windowmgmt, dictation, update, diag < palette < cmd`. A Go test walks `go list -deps` and fails on upward imports and cycles (equivalent of `scripts/check-electron-structure.cjs`). A file-size ratchet lives next to it (lower baselines, never raise).

---

## 6. Agent brief (paste this when delegating a WP)

> You are implementing **WP<id>** of `src/native/PLAN.md` for the Nevermind Go rewrite (branch from `t3/investigate-mygo-docs` or its successor; PR into it). Read PLAN.md sections 0-5 and WP<id> only. Read the files under "Read first" for current behavior; port behavior, not structure. Work only in the packages named in "Build". Do not change other WPs' packages except through the interface already agreed in "Interfaces". Run the validation rungs listed; attach outputs (test logs, golden diffs, rig screenshots, budget numbers) to the PR. Do not run Electron or any GUI on a maintainer desktop: tests with `ui.NewTester` are fine; real-app runs happen only in the rig container or CI. Tick the matching boxes in the PR #280 checklist only when every listed rung passed; otherwise leave them and say what is missing. If a decision D1-D6 blocks you, use the default in section 0 and note it in the PR. Finish by listing: what you built, what you verified (and on which OS), what you did not.

**Definition of done for any WP:** builds and passes L0-L3; the listed L4-L6 runs attached; journeys listed ran green; goldens and review sheet added; docs updated (package doc comment, this plan's WP status line); no TODOs without an issue; no new dependency without justification; checklist boxes updated.

---

## 7. Milestones and exit criteria

| Milestone | WPs | Exit |
|---|---|---|
| **M0 Foundations** | WP0.1-WP0.8 | rig runs the current clone end to end in CI; goldens and budgets enforced; store and ports in place; design package replaces ad-hoc styling |
| **M1 Daily driver** | WP1.1-WP1.12 | J01-J11, J22, J23, J25-J27, J29, J30 green on macOS; dogfood 5 days; B1-B5 met |
| **M2 Account and AI** | WP2.1-WP2.6 | J12, J13, J20, J21, J28 green; chat and builder usable against production backend; costs and limits handled |
| **M3 Extension platform** | WP3.1-WP3.10 | J14-J18 green; at least 5 built-ins running as extensions with parity tests; API reference generated |
| **M4 OS depth** | WP4.1-WP4.3 | J19 green on macOS; OCR and semantic search behind capability checks |
| **M5 Ship** | WP5.1-WP5.6 | J24 green; all platforms signed and installable; two weeks of beta with no data-loss bugs; parity review complete; cutover plan approved |

Dependency sketch: `WP0.* → WP1.1 → (WP1.2-1.12 in parallel) ; WP0.6 → everything that persists ; WP2.1 → WP2.2-2.6 ; WP3.1 → WP3.2 → WP3.3-3.9 → WP3.10 ; WP1.6 + WP2.1 → WP2.5 ; WP3.* + WP2.4 → WP3.9 ; M1-M4 → WP5.*`.

---

# Work packages

Size: S ≤ 1 day, M a few days, L a week or two, XL several weeks. Sources are paths in this repo. "Rungs" are the validation ladder rows from section 1.

## M0 Foundations

### WP0.1 Workspace, layering, CI, agent rules  `[S-M]`
- **Goal:** a clean multi-package Go module, enforced layering, lint, CI, and written rules for agents.
- **Depends on:** none.
- **Read first:** `src/native/*.go` (current prototype), `scripts/check-electron-structure.cjs`, `scripts/check-file-size.cjs`, `.github/workflows/ci.yml`, `AGENTS.md`, `CLAUDE.md`.
- **Build:** the layout in section 5 (move prototype files under `internal/...` without behavior change); `go.mod` with a pinned Go version and `mygo` version; `internal/layers_test.go` (upward imports, cycles); file-size ratchet; `.github/workflows/native.yml` (L0-L3 on Linux, `-race`, `govulncheck`, `staticcheck`); `mise` tasks; a section in `AGENTS.md` for the Go tree including the "runtime verification only in the rig" rule; package doc comments.
- **Acceptance:** CI green on the prototype; a deliberately bad import fails the layer test; existing screenshots unchanged (goldens come in WP0.4, so compare with the saved `docs/*.png` by eye until then).
- **Rungs:** L0, L1.
- **UX / UI validation:** none beyond "behavior unchanged" (J01, J02 smoke in the rig once WP0.3 exists).
- **Watch out:** do the move in one mechanical commit; no logic edits in it.

### WP0.2 Ports, fakes and fixtures  `[M]`
- **Goal:** every external effect behind an interface with a fake, and a deterministic fixture set.
- **Depends on:** WP0.1.
- **Read first:** prototype uses of `mygo.Clipboard`, `mygo.Shell`, `mygo.Screen`, `exec.Command`, `os`, `http`, `time` in `src/native`; `docs/os-architecture.md`.
- **Build:** `internal/platform` ports: `Clock`, `FS`, `HTTP`, `Clipboard`, `Shell` (open, reveal, trash, apps, icons), `Screens`, `Notifier`, `Keychain`, `Processes`, `Helper` (child processes), `Hotkeys`, `Tray`, `Autostart`; real implementations wrapping MyGo and the OS; fakes in `internal/platform/fake`; `internal/fixtures` (section 1). Composition root builds real or fake sets.
- **Acceptance:** no feature package imports `os/exec`, `net/http`, `time.Now` or `mygo` directly (grep test, allow-list for `platform` and `cmd`); a view runs in `ui.NewTester` using only fakes.
- **Rungs:** L0-L2.
- **UX / UI:** none; enables the rest.
- **Watch out:** keep ports intent-named ("reveal file"), not mechanism-named.

### WP0.3 Rig: containers and CI runners for L4-L6  `[L]`
- **Goal:** reproducible real-app runs with screenshots on Linux, macOS and Windows.
- **Depends on:** WP0.1.
- **Read first:** the prototype's ad-hoc scripts described in the PR (Xvfb, openbox, picom, XFCE panel as tray host, dbus, PulseAudio virtual mic, xdotool, xclip); `.github/workflows/report-linux-palette-screenshot.yml`, `.github/workflows/macos-selected-text.yml`; MyGo `docs/plugins/automation.md`, `docs/ui/testing.md`.
- **Build:** `scripts/rig/Dockerfile` and a driver (`rig up|run|shot|down`) wrapping the above; a test build tag linking the `automation` plugin (`MYGO_AUTOMATION`) so tests find elements by label/key instead of coordinates; helper library `rigtest` (press hotkey, type, wait for frame idle, screenshot, read clipboard, list tray items, kill -9); jobs for macOS and Windows runners (build, launch, grant permissions via MDM/tccutil where possible, screenshot, upload); artifact upload; nightly schedule.
- **Acceptance:** J01, J02, J05 scripted and green on Linux in CI; macOS and Windows jobs launch the app and screenshot the palette; a failing step uploads screenshot + log.
- **Rungs:** L4-L6.
- **UX / UI:** produces the platform screenshot sheets used for sign-off.
- **Watch out:** tray icons need a tray host running before the app starts; wallpaper must be fixed for stable goldens; permissions on macOS runners may need a self-hosted machine.

### WP0.4 Golden harness, references, contrast tests  `[M]`
- **Goal:** visual regression net and the Electron reference sheets.
- **Depends on:** WP0.2, WP0.8 can follow in parallel.
- **Read first:** `src/app/palette/styles/theme.css`, `palette.css`; `tests/electron/palette.smoke.spec.ts`; MyGo `ui.Render`, `Tester.Image()`.
- **Build:** `internal/design/golden` (render a view at light/dark × 1x/2x with a fake clock and fixtures, compare with tolerance, `-update` flag writes new goldens, diff image on failure); `testdata/reference/` Electron screenshots via a script that drives the Electron app in the rig; token contrast test (WCAG AA for each text/surface pair in both themes); hex-literal lint outside `design`.
- **Acceptance:** goldens for the current palette root, search, clipboard + preview, action menu, form, chat; a one-pixel change fails; updating is one command.
- **Rungs:** L2, L3.
- **UI validation:** this WP defines the checklist enforcement; reference sheet reviewed by the owner.

### WP0.5 Performance and budget harness  `[M]`
- **Goal:** enforce section 3.2 numbers.
- **Depends on:** WP0.2, WP0.3.
- **Read first:** prototype `bench.py` approach (PSS summed over the process tree, CPU ticks over a window), `docs/` perf notes in `electron/diagnostics/performance-trace.ts`.
- **Build:** Go benchmarks for ranking and providers on the J29 dataset (`-benchmem`); rig scripts measuring B1, B3-B5, B7 (PSS, CPU, frame requests, goroutine and FD counts via a debug endpoint compiled only in test builds); thresholds file; CI comment with deltas against `main`.
- **Acceptance:** numbers recorded for the prototype as the baseline; a deliberate regression fails the job.
- **Rungs:** L1, L4.
- **UX validation:** B1-B8 table filled and sent to the owner for ratification.

### WP0.6 Persistence layer v2  `[M]`
- **Goal:** the only way data reaches disk.
- **Depends on:** WP0.2.
- **Read first:** `src/app/electron/platform/atomic-file.ts`, `private-file.ts`, `state-write-guard.ts`, `shared-state-lease.ts`, `electron/state/state-safe-quit.ts`, `docs/solutions/` (search for state corruption notes).
- **Build:** `internal/store`: typed JSON collections with schema version, atomic write, N backups, quarantine on corrupt, debounced writes with flush on quit and on crash signal, single-writer lease, per-file mode, change events; migration framework (version N → N+1 functions, tested); replace the prototype's `store.go`.
- **Acceptance:** tests for crash between write and rename (fault-injecting FS fake), corrupt file, two processes, unknown newer version (refuse to overwrite, warn), large file performance; J30 in the rig.
- **Rungs:** L1, L4.
- **UX validation:** J30; the quarantine notice copy reviewed.
- **Watch out:** never log contents; redaction tests use canaries.

### WP0.7 Logging, panic isolation, diagnostics core  `[S-M]`
- **Goal:** `logx` (levels, file ring, redaction), `safego.Run` (recover, report, backoff), error references.
- **Depends on:** WP0.2.
- **Read first:** `docs/logging.md`, `electron/diagnostics/diagnostic-store.ts`, `diagnostic-reporting.ts`, `platform/logger.ts`.
- **Build:** the three packages; every goroutine in the prototype moves to `safego`; a `Diagnostics` data source for the view in WP1.12.
- **Acceptance:** panics in providers do not crash the app and are recorded with a reference id; redaction canary test; log files rotate.
- **Rungs:** L1, L4 (kill and panic injection).

### WP0.8 Design system package  `[M-L]`
- **Goal:** all styling in one package with a component gallery.
- **Depends on:** WP0.4 (goldens) in parallel.
- **Read first:** prototype `theme.go`, `depth.go`, `palette.go` rendering, `forms.go`; `theme.css`, `palette.css`, `views.css`.
- **Build:** `internal/design`: tokens (light/dark), depth helpers, components (`Pane`, `SearchRow`, `Row`, `Tile`, `Section`, `Keycap`, `Chip`, `ActionMenu`, `Popover`, `EmptyState`, `Form`, `Field`, `Toast`, `Markdown`), Lucide icon table, a hidden `Gallery` view listing every component and state; platform variants for material vs fallback.
- **Acceptance:** palette renders identically (goldens) after refactor; no style literal outside the package; gallery goldens for all components and states.
- **Rungs:** L2, L3, L4 (+L5 for material).
- **UI validation:** full section 2 checklist on the gallery; owner sign-off on the sheet.

## M1 Daily driver

### WP1.1 Palette core: navigation, actions, search coordinator  `[L]`
- **Goal:** the architecture every feature plugs into.
- **Depends on:** WP0.2, WP0.7, WP0.8.
- **Read first:** `src/app/palette/App.tsx` (navigation, query handling), `palette/model.ts`, `electron/search/search-coordinator.ts`, `search-snapshot.ts`, `action-ranker.ts`, `palette/search-session.ts`, `palette/shortcuts.ts`, `palette/action-panel.tsx`, `electron/actions/*`.
- **Build:** `internal/search`: `Provider` interface (`Root(ctx)`, `Search(ctx, q) <-chan Batch`), result snapshots (cached first, refreshed in place), scoring merge (ranking from the prototype), cancellation on keystroke, off-UI-thread execution with partial results; `internal/palette`: view stack with presentations (root, stacked, preview, side-preview), view patching (patch/replace/prepend/append), pending-view loading, back/forward, focus restoration; `Action`/`ActionPanel` with sections, local vs global shortcuts, confirmation as a palette view, `dismissAfterRun` policy; keyboard contract (Command-K never strands focus).
- **Interfaces:** `Provider`, `Item` (maps to `ExtensionItem`), `Action` (maps to `ExtensionAction`), `View` (maps to `ExtensionView`); WP3.1 reviews these before they freeze.
- **Acceptance:** L2 suite for navigation, focus, shortcuts, Esc semantics, action panel; provider that blocks 2 s does not delay typing (tester with fake clock); stale results never overwrite newer queries.
- **Rungs:** L1-L4.
- **UX validation:** J01, J02, J27, J29 (B2 measured).
- **UI validation:** goldens for each presentation and the action panel.

### WP1.2 Ranking, usage learning, query history  `[S-M]`
- **Goal:** results that improve with use, identical ranking contract to 1.x.
- **Depends on:** WP1.1, WP0.6.
- **Read first:** `palette/search-ranking.ts` and its test, `electron/state/learning-store.ts`, `extensions/_context.ts` (`usageBoost`, `recentBoost`), `palette/App.tsx` query history.
- **Build:** usage and recency boost with decay, per-item learning store, query history (100 entries), bounded memory; port the TypeScript ranking tests to Go table tests (same cases).
- **Acceptance:** all ported cases pass; a golden query list (`testdata/queries.txt`) produces the same top 5 as the Electron build on shared fixtures (documented differences only).
- **Rungs:** L1, L2. **UX:** J02, parity review of 20 queries.

### WP1.3 Panel window and hotkey per OS  `[M-L]`
- **Goal:** Spotlight-like window behavior everywhere.
- **Depends on:** WP0.3, WP1.1.
- **Read first:** `docs/window-management.md` ("Palette window" section), `electron/app/palette-window.ts`, `palette-window-geometry.ts`, `palette-shortcuts.ts`, `palette-hotkey-notice.tsx`, `electron/os/palette-spaces.ts`, `shortcuts/global-shortcut-registration.ts`.
- **Build:** MyGo `Panel` window (nonactivating, all spaces, over full-screen), placement on the cursor's display, remembered offset, hotkey registration with retry and recovery list (Linux `Control+Alt+K`...), hotkey notice and "No shortcut set" banner, tray fallbacks, accessory activation policy, behavior with tiling managers (AeroSpace note in the doc).
- **Acceptance:** rig tests for show/hide/focus return; macOS runner test of full-screen and Spaces behavior; failure paths (hotkey taken) produce the banner.
- **Rungs:** L2, L4, L5, L6. **UX:** J01, J11, J25, J26.
- **UI validation:** material on macOS/Windows, fallback elsewhere captured.
- **Watch out:** do not remove the settings the doc says to keep without verifying with a tiling manager.

### WP1.4 Apps provider, running apps, uninstall  `[L]`
- **Goal:** apps parity including the safe uninstall flow.
- **Depends on:** WP1.1, WP0.2.
- **Read first:** `extensions/apps.ts`, `electron/app/app-index-service.ts`, `app-icon-cache.ts`, `app-bundle-icons.ts`, `running-app-status.ts`, `app-uninstall-*.ts` (all), `extensions/quit-apps.ts`, `electron/os/app-quit.ts`, `windows-app-icons.ts`.
- **Build:** apps index with watcher and persisted cache, icon cache sized to the display scale, first-seen, running status, launch; force quit, quit all (with confirmation view); uninstall flow: discover related paths, guards (never outside known roots, never system apps), review list, Trash, result view; per-OS implementations behind `platform`.
- **Acceptance:** uninstall guard tests with hostile paths (symlinks, `..`, system apps); fixtures for 500 apps; no UI-thread I/O (race and tester time tests).
- **Rungs:** L1, L2, L4-L6. **UX:** J02, J22.
- **UI validation:** goldens for the review list and confirmation; icons crisp at 1x/2x.
- **Watch out:** uninstall is destructive; ship it last in the WP behind the guards' tests.

### WP1.5 Calculator and rates  `[M]`
- **Goal:** full parity of `calculator.ts` including currency.
- **Depends on:** WP1.1, WP0.2 (HTTP fake).
- **Read first:** `electron/search/calculator.ts` and tests, `extensions/calculator.ts`, `state` `rateCache`.
- **Build:** parser and units (port the test table), rate expressions with cached rates, offline stale label, copy/paste actions, detail view; replace the prototype `calc.go`.
- **Acceptance:** all ported TypeScript test cases pass; rates fixture and offline behavior; locale number formats.
- **Rungs:** L1, L2. **UX:** J03. **UI:** golden for result row and detail.

### WP1.6 Clipboard v2  `[L]`
- **Goal:** the must-have feature, robust per OS.
- **Depends on:** WP1.1, WP0.6, WP0.2.
- **Read first:** `electron/clipboard/*.ts`, `extensions/clipboard.ts`, `palette/clipboard-history-range.ts`, `electron/os/electron-clipboard.ts`, `linux-x11-clipboard.ts`, `macos-selected-text.ts`, `desktop-selection.ts`, `desktop-text-access.ts`, `tests/electron/clipboard-concealed.smoke.spec.ts`, `macos-helper/selected-text.swift`.
- **Build:** watcher per OS (native change notification where MyGo/OS offers it, otherwise adaptive polling), formats (text, HTML/RTF, image, files), concealed/transient flags honored, source app, dedupe, image store with GC and thumbnails, retention, history range filters, search actions, paste into frontmost app per OS (helper on macOS, `SendInput` on Windows, X11/Wayland strategies on Linux), pin/delete/clear, "ask AI" action, desktop selection reader through the helper.
- **Acceptance:** fixture tests for 5k entries; concealed canary never stored (also in logs); GC removes only unreferenced images; rig tests for copy → history → paste; macOS test for paste into TextEdit.
- **Rungs:** L1, L2, L4-L6. **UX:** J05, J06, J29.
- **UI validation:** goldens for text, image, pinned, long text, empty state; preview pane states.
- **Watch out:** never read the clipboard on a timer faster than needed; privacy copy for the Settings toggle reviewed.

### WP1.7 Files v2  `[L]`
- **Goal:** a real index and previews.
- **Depends on:** WP1.1, WP0.6.
- **Read first:** `electron/files/file-index*.ts`, `file-index-roots.ts`, `ignore-patterns.ts`, `directory-tree-watch.ts`, `find-files.ts`, `thumbnails.ts`, `quick-look-and-open-with.ts`, `text-file-preview.ts`, `start-file-drag.ts`, `extensions/files.ts`.
- **Build:** configurable roots and ignore patterns, FS watcher with debounce, incremental commit, persisted index with background refresh, sort by recency/name, kinds (image, video, media), search with streamed partial results; preview (text bounded to 64 KB with binary detection, images, video), thumbnails (Quick Look on macOS, native on others), open-with list, reveal, trash, drag-out.
- **Acceptance:** index of 50k fixture files under B2/B3 budgets; watcher test with fake FS events; ignore pattern table tests; memory test for the index.
- **Rungs:** L1, L2, L4-L6. **UX:** J07, J29. **UI:** goldens for preview types.
- **Watch out:** index must never delay first paint; start serving from the persisted copy.

### WP1.8 Emoji, symbols, Unicode  `[M]`
- **Goal:** the real catalog and search.
- **Depends on:** WP1.1.
- **Read first:** `extensions/emoji-symbols.ts`, `emoji-symbol-data.ts`, `electron/app/emoji-symbol-catalog.ts`, tests.
- **Build:** generate the Go catalog from the same source data (script, checked in), search by name/keyword/char/Unicode value, skin tones, recent, categories, copy/paste actions; grid view with keyboard navigation and screen reader labels.
- **Acceptance:** ported search tests; catalog generation is deterministic; `Announcements` test for the grid.
- **Rungs:** L1-L3. **UX:** J08, J27. **UI:** goldens for grid, skin tone picker, no results.

### WP1.9 Notes and snippets  `[M-L]`
- **Goal:** floating notes and snippets with draft safety.
- **Depends on:** WP1.1, WP0.6, WP0.8.
- **Read first:** `extensions/floating-notes.ts` and tests, `extension-draft-store.ts`, `extension-draft-persistence.ts`, `extension-window-restore.ts`, `palette/extension-view/markdown-editor.tsx`, `editor-title.ts`; prototype `forms.go`.
- **Build:** Markdown notes with the MyGo editor/rich text, autosave through drafts with conflict detection and the "keep saved version / resolve unsaved changes" flow, floating windows restored at launch, list actions; snippets with keywords, placeholders, paste; delete confirmations.
- **Acceptance:** crash test (kill during edit) restores text; conflict test with two instances; restore geometry on a changed display layout.
- **Rungs:** L1, L2, L4-L6. **UX:** J09, J10, J30. **UI:** goldens for editor, conflict view, list.

### WP1.10 OS capabilities: system, places, settings, autostart, permissions  `[M-L]`
- **Goal:** the capability layer from `docs/os-architecture.md`.
- **Depends on:** WP0.2, WP1.1.
- **Read first:** `docs/os-architecture.md`, `docs/windows-platform-inventory.json`, `extensions/system.ts`, `settings.ts`, `permissions.ts`, `getting-started.ts`, `electron/os/os.ts`, `system-settings.ts`, `palette-permissions.ts`, `electron/app/settings.ts`, `settings-service.ts`.
- **Build:** `platform` capabilities with per-OS implementations (lock, sleep, restart, shut down, log out, empty trash, open system settings, appearance, autostart, places); each reports `Supported()` so providers omit unsupported items; Settings view (all real settings) with confirmations in the palette; Permissions view (mic, camera, accessibility, screen recording, selected text, files and folders) prompting only when a command needs it; Getting Started guide and "Welcome" row.
- **Acceptance:** table test: for each OS the supported set equals the documented inventory; destructive commands require confirmation views; fakes verify the exact commands issued.
- **Rungs:** L1, L2, L4-L6. **UX:** J11, J23, J25. **UI:** goldens for Settings, Permissions, confirmation.

### WP1.11 Keyboard shortcuts manager and view  `[M-L]`
- **Goal:** global and local shortcut ownership with a recorder.
- **Depends on:** WP1.1, WP1.3, WP0.6.
- **Read first:** `electron/shortcuts/*.ts`, `extensions/keyboard-shortcuts.ts`, `palette/shortcut-manager.tsx`, `shortcut-label.test.ts`, `shared/shortcut-utils.ts`.
- **Build:** canonical storage format (`Command+Alt+K`) and symbol display, binding plan with conflicts and ownership, per-action customization, OS-reserved list, recorder view, registration lifecycle, extension shortcuts.
- **Acceptance:** ported tests for labels and binding plans; conflict resolution table; recorder tester test.
- **Rungs:** L1, L2, L4-L6. **UX:** J11, J27. **UI:** goldens for the list and the recorder.

### WP1.12 Menu bar item, diagnostics, analytics, updates stub  `[M]`
- **Goal:** the app's own menu bar item and the supportability surface.
- **Depends on:** WP0.7, WP1.1.
- **Read first:** `extensions/menubar.ts`, `electron/app/menubar-composition.ts`, `extensions/diagnostics.ts`, `electron/diagnostics/*`, `docs/posthog-observability.md`, `extensions/feedback.ts`.
- **Build:** tray/status item with Open, Settings, Check for Updates, Quit; Diagnostics view (recent errors with copyable references, report a problem without screen content); analytics client exactly per the observability doc with opt-out and redaction tests; performance traces feeding WP0.5.
- **Acceptance:** canary redaction; offline queueing; no event contains content fields (schema test).
- **Rungs:** L1, L2, L4-L6. **UX:** J25, J28. **UI:** goldens for Diagnostics.

## M2 Account and AI

### WP2.1 Backend client  `[M]`
- **Goal:** the one typed gateway to the backend.
- **Depends on:** WP0.2, WP0.7.
- **Read first:** `src/app/electron/backend/*.ts` (all), `docs/backend-api-compatibility.md`, `docs/production-auth-proxy-smoke.md`, `src/backend/src/pages/api/**` (routes: `v1/chat/completions`, `messages`, `responses`, `decisions`, `audio`, `models`, `active-model`, `me`, `auth`, `compatibility`), `src/backend/src/pages/api/contract-routes.test.ts`.
- **Build:** `internal/backend` with typed methods, environment selection (development, production, PR preview, custom), compatibility headers and sunset handling, SSE reader with robust parsing (comments, partial lines, error events, usage), retries for idempotent calls, mapped errors (401, 402 with credits info, 429, 5xx, offline); a fake backend server for tests with recorded responses.
- **Acceptance:** contract tests run against recorded fixtures derived from the backend's own contract tests; fuzzed SSE input; `deployed-smoke` equivalent in Go (opt-in, uses a staging token).
- **Rungs:** L1, L4, plus a live smoke (documented credentials flow, not in default CI).
- **UX:** J28 (each error class has a message in the copy sheet).

### WP2.2 Sign-in, keychain, deep links, account view  `[M]`
- **Goal:** replace the prototype token file.
- **Depends on:** WP2.1, WP0.6.
- **Read first:** `nevermind-sign-in.ts`, `device-exchange.ts`, `nevermind-auth.ts`, `nevermind-deep-links.ts`, `electron/app/deep-link.ts`, `extensions/account.ts`, `byo-key.ts`, `free-spend-cap-notice.ts`.
- **Build:** device flow with status polling and cancel, token in keychain, deep link `nvm://` handling, account view (log in/out, plan, credits via `/api/me`, backend environment switcher), BYO key set/clear, spend-cap notices.
- **Acceptance:** fake backend covers pending/approved/denied/expired; token never in logs (canary); deep link tests; keychain fake and real.
- **Rungs:** L1, L2, L4-L6. **UX:** J12, J28. **UI:** goldens for each sign-in state.

### WP2.3 AI chat core and UI  `[L]`
- **Goal:** the full chat experience.
- **Depends on:** WP2.1, WP1.1, WP0.6, WP0.8.
- **Read first:** `src/app/electron/ai/ai-chat-*.ts`, `ai-limit-*.ts`, `ai-pricing-notice.ts`, `ai-model-route-eligibility.md`, `palette/ai-chat/*`, `extensions/ai-commands.ts`, `docs/ai-model-route-eligibility.md`.
- **Build:** sessions store with retention and migration of schema, model tiers via active-model, streaming render with the MyGo markdown, stop/regenerate, message actions (copy, retry), attachments and images (store, thumbnails), limit and pricing notices, "Continue conversation" items, conversation list view, keyboard flow; chat composer pill per design.
- **Acceptance:** scripted SSE transcripts (including malformed and mid-stream error) render correctly; retention deletes only per settings; L2 tests for stop and resume; golden for long markdown, code block, table.
- **Rungs:** L1-L4. **UX:** J13, J28. **UI:** goldens for each message type and notice.

### WP2.4 Agent loop and tools  `[XL]` (starts with a spike)
- **Goal:** the tool-using assistant that powers the builder and chat actions.
- **Depends on:** WP2.1, WP2.3.
- **Read first:** `electron/ai/pi-runtime.ts`, `pi-provider.ts`, `pi-session-events.ts`, `pi-session-tool-policy.ts`, `ai-tool-policy.ts`, `ai-credential-path-guard.ts`, `ai-session-outcome.ts`, `ai-one-shot-result.ts`, `ai-learning-review.ts`, `docs/` pi notes, `.github/workflows/pi-updates.yml`.
- **WP2.4a Spike (≤ 3 days, report only):** compare (1) native Go loop on chat completions with tool calls and (2) Node `pi` sidecar over stdio. Criteria: parity of tools and events, failure isolation, install size, update path, test determinism. Output: a recommendation and a thin interface `ai.Agent` both could implement.
- **WP2.4b Build:** chosen option behind `ai.Agent`: sessions, tool registry with policy (what each tool may touch), credential path guard, cancellation, token/credit accounting, event stream to the UI, deterministic mock model for tests (scripted tool-call sequences).
- **Acceptance:** golden transcripts (input script → exact event sequence); tool policy table tests including path traversal and credential files; cancellation leaves no running subprocess.
- **Rungs:** L1, L2, L4. **UX:** J13, J14 (shared with WP3.9). **UI:** tool-call rendering goldens.
- **Watch out:** this is the largest risk; do not start WP3.9 on an unreviewed spike.

### WP2.5 AI commands on selected text and images  `[L]`
- **Goal:** Fix/Transform/Triage Selected Text, Classify Image.
- **Depends on:** WP2.1, WP1.6 (selected text), WP1.1.
- **Read first:** `extensions/ai-commands.ts` (proofreading guard `proofreadingOutputPreservesInput`), `ai-decision-triage.ts`, `ai-image-decision.ts`, `electron/ai/nevermind-decision-client.ts`, `extension-ai-decision-request.ts`, `macos-helper/selected-text.swift`, `electron/os/selected-text.ts`, `windows-desktop-text.ts`, `linux-desktop-text.ts`.
- **Build:** selected-text read/replace through the helper per OS, indicator window with status, the same edit-distance and word-overlap guards, decision API calls, error and unchanged outcomes.
- **Acceptance:** ported guard tests (same vectors); fake helper tests; macOS runner test with TextEdit.
- **Rungs:** L1, L2, L5, L6. **UX:** J21. **UI:** indicator goldens (loading, done, failed).

### WP2.6 Dictation  `[L-XL]`
- **Goal:** cloud dictation end to end.
- **Depends on:** WP2.1, WP1.10 (permissions), WP1.6 (paste).
- **Read first:** `electron/dictation/*`, `extensions/dictation.ts`, `palette/dictation/*`, `palette/indicator/*`, `ipc/dictation-ipc.ts`, `electron/os/system-audio.ts`, MyGo `docs/audio.md`.
- **Build:** microphone capture through a native path (MyGo's `audio` package is output only: spike a helper binary vs purego AVFoundation / WASAPI / PulseAudio first, report before building), device list and selection, level meter, encoding, upload to `/api/v1/audio`, vocabulary terms (10/30/50), history and recordings store with retention, paste into the frontmost app, indicator window, permission flow.
- **Acceptance:** virtual-mic rig test (PulseAudio loop with a known WAV) on Linux; fake backend; macOS permission prompt flow on the runner; denied and missing-device paths.
- **Rungs:** L1, L2, L4-L6. **UX:** J20. **UI:** indicator and history goldens.

## M3 Extension platform

### WP3.1 Extension API v1 design and spikes  `[L]`
- **Goal:** the Go API surface, reviewed before anyone builds on it.
- **Depends on:** WP1.1 (interfaces), none otherwise.
- **Read first:** `src/app/resources/nevermind-extension-api.d.ts` (all 2,400 lines), `docs/extension-trust-model.md`, `palette-first-extension-ui.md`, `list-first-extension-ui.md`, `extension-api-ui-fixtures.md`, `extension-host/extension-manifest.ts`, `extension-capabilities.ts`.
- **Build:** a design doc `src/native/docs/extension-api.md` and Go types in `internal/ext/api`: items, accessories, metadata, detail, appearance, sections, actions and panels, shortcuts, views (list, grid, collection, preview, progress, form with 16 fields, editor, camera, webview), commands, triggers, menu bar contributions, context services, capabilities. Version field and compatibility rules. Spikes with numbers: Yaegi stdlib coverage and generics, per-call overhead, memory per extension, isolation options (in-process vs subprocess with RPC), what happens on Go version bumps; decision recorded for D1.
- **Acceptance:** owner review of the doc; sample extensions (10 real use cases from existing built-ins) written against the types and compiled; spike report.
- **Rungs:** L1.
- **UX validation:** author-experience test: an agent given only the API doc writes 5 extensions from prompts; success rate and error clarity recorded.
- **Watch out:** this API is the product's contract; prefer fewer, composable primitives (`CLAUDE.md`: fix missing primitives in the API, do not bypass it).

### WP3.2 Host runtime: loader, trust, isolation, errors  `[L]`
- **Goal:** load, enable, run, and contain extensions.
- **Depends on:** WP3.1, WP0.6, WP0.7.
- **Read first:** `extension-host/extension-loader.ts`, `extension-manager.ts`, `extension-module-loader.ts`, `extension-identity.ts`, `extension-runtime-state.ts`, `extension-runtime-transaction.ts`, `extension-source-files.ts`, `extension-rate-limits.ts`, `extension-capabilities.ts`, `extensions/extensions.ts`.
- **Build:** manifest/identity, enablement state (dropped-in default disabled), capability review view, runtime per extension with per-call timeouts (B8), panic recovery, memory watch, disable-on-repeat-failure with a reason, hot reload, atomic install/uninstall, extension manager view (enable/disable/update/delete/show source), error surfaces in the palette.
- **Acceptance:** hostile extension fixtures (infinite loop, panic, memory bomb, huge results, slow init) leave the palette responsive; enable/disable race tests; install atomicity.
- **Rungs:** L1, L2, L4-L6. **UX:** J15, J16. **UI:** goldens for manager, review, failure states.

### WP3.3 Declarative views to native widgets  `[XL]`
- **Goal:** render list, grid, collection (CRUD), preview, file preview, progress, details.
- **Depends on:** WP3.2, WP0.8.
- **Read first:** `palette/extension-view/extension-view.tsx`, `preview-view.tsx`, `progress-view.tsx`, `list-item-class-name.ts`, `pane-navigation.ts`, `electron/actions/view-normalizer.ts`, `view-results.ts`, `view-types.ts`, `extension-ui-api.ts` and `extension-ui-preview.ts`, `docs/list-first-extension-ui.md`.
- **Build:** renderers mapping each view type to design components; pagination and search accessory; item accessories (tones), metadata, detail panes; view patching; clone-safe payloads (no functions across the host boundary; actions referenced by ids/tokens, per the `CLAUDE.md` IPC rules); fixtures for every endpoint that renders UI.
- **Acceptance:** one fixture per API view type with goldens; clone-safety tests (payloads serialize; privileged actions tokenized); routing test for the overloaded `kind` field the `CLAUDE.md` warns about.
- **Rungs:** L1-L4. **UX:** J14, J15, J27. **UI:** full section 2 checklist per view type.

### WP3.4 Forms, editors, drafts, pickers  `[L]`
- **Goal:** the 16 field types and editor views.
- **Depends on:** WP3.3.
- **Read first:** `palette/extension-view/palette-form.tsx`, `form-fields.ts`, `electron/app/form-field-paths.ts`, `palette-prompt.tsx`, `extensions` form usage, `extension-draft-*.ts`, `markdown-editor.tsx`.
- **Build:** text, textarea, password, email, url, number, date, checkbox, dropdown, select, multiselect, file, files, folder, description, separator; validation messages; palette-prompt (single-field flows in the search field); editor view (text/markdown) with drafts and conflict resolution; native pickers via MyGo `Dialog`.
- **Acceptance:** per-field tester tests (focus order, validation, keyboard); password never logged; draft conflict tests.
- **Rungs:** L1-L4. **UX:** J09, J10, J27. **UI:** goldens for every field and error state.

### WP3.5 Menu bar contributions and status items  `[L]`
- **Goal:** rich tray/menu bar items from extensions.
- **Depends on:** WP3.2.
- **Read first:** `extension-host/extension-menubar.ts`, `electron/os/menubar-status-items.ts`, `menubar-status-item-model.ts`, `status-item-anchor.ts`, `electron/app/extension-menubar-service.ts`, `menubar-visibility.ts`, `menu-bar-window-placement.ts`, `ExtensionMenuBar*` types, MyGo `examples/menubar-native`, `examples/menubar-flyout`.
- **Build:** icon sets, title, rows with sections and actions, refresh scheduling, visibility settings, flyout windows where supported, per-OS behavior (tray hosts on Linux).
- **Acceptance:** fake tray tests; refresh/leak tests; rig with a tray host; macOS runner screenshot of the status item.
- **Rungs:** L1, L2, L4-L6. **UX:** J17. **UI:** menu and flyout goldens.

### WP3.6 Triggers and jobs  `[L]`
- **Goal:** background execution with guarantees.
- **Depends on:** WP3.2, WP1.6, WP1.7.
- **Read first:** `extension-host/extension-triggers.ts`, `host-jobs.ts`, `clipboard-image-gc-job.ts`, `frontmost-app-polling.ts`, `platform/jobs.ts`, `job-types.ts`, `extensions/background-tasks.ts`.
- **Build:** triggers (startup, interval, `clipboard.changed`, `files.changed` with debounce/no-overlap/bounded context, `app.frontmost.changed`, wake, login), background modes, rate limits, job registry, Background Tasks view (inspect, run, enable), sleep/wake handling.
- **Acceptance:** fake clock tests for intervals and debounce; no overlap under slow runs; wake handling with simulated suspend; resource soak (B7).
- **Rungs:** L1, L2, L4-L6. **UX:** J18. **UI:** Background Tasks goldens.

### WP3.7 Context services I: storage, secrets, cache, settings, logs, text, data  `[L]`
- **Goal:** per-extension persistence and helpers.
- **Depends on:** WP3.2, WP0.6.
- **Read first:** `extension-storage*.ts`, `extension-json-store.ts`, `extension-secrets.ts`, `extension-memory-cache.ts`, `extension-text-template.ts`, `extension-draft-store.ts`, `ExtensionStorage|Secrets|RuntimeCache|Settings|Text|Data|Logs` types.
- **Build:** namespaced stores with quotas, keychain-backed secrets, TTL cache, settings declarations with UI, text templates, data loaders with refresh, logs per extension.
- **Acceptance:** quota and isolation tests (one extension cannot read another's keys); secrets canary; template table tests.
- **Rungs:** L1, L2. **UX:** J14, J15. **UI:** settings view goldens.

### WP3.8 Context services II: desktop, clipboard, OCR, system, shortcuts, windows  `[L-XL]`
- **Goal:** the capability-gated host services.
- **Depends on:** WP3.2, WP1.4, WP1.6, WP1.7, WP1.10, WP1.11.
- **Read first:** `extension-context*.ts`, `extension-desktop-window-management.ts`, `extension-window-*.ts`, `extension-capability-validation.ts`, `os/extension-window-capabilities.ts`, `Extension*` types for desktop/files/shell/apps/ocr/system/shortcuts/windows.
- **Build:** adapters from extension calls to the feature packages with capability declaration checks (declared, reviewed, logged), extension windows with restore, shell execution limits, update state.
- **Acceptance:** capability table tests (undeclared use logged and flagged per the trust model); extension windows survive restart; shell output size and timeout limits.
- **Rungs:** L1, L2, L4-L6. **UX:** J14-J18.

### WP3.9 Extension builder (AI)  `[L-XL]`
- **Goal:** the Automate flow at full fidelity.
- **Depends on:** WP2.4, WP3.1-WP3.4, WP3.7.
- **Read first:** `extensions/ai-builder.ts`, `electron/ai/ai-builder-*.ts`, `extension-ai-builder.ts`, `ai-extension-ownership.ts`, `ai-created-actions.ts`, `extension-guidance-updates.ts`, `ai-chat/builder-preview.ts`, `resources/skills/nevermind-extension-builder/SKILL.md`, `extension-host/extension-proposals.ts`, `extension-pr-submitter.ts`.
- **Build:** builder tools (read page, read/write extension files, validate, preview-run observer, request guidance), staging with atomic promotion, auto-fix loop with bounded retries, ownership (which extensions the AI may edit), builder context generated from the Go API types, proposals and PR submitter, preview before enable, window-management guidance.
- **Acceptance:** eval harness of 30 prompts → extensions with pass criteria (loads, runs, does what was asked on fixtures); regression set grows with every bug; no half-written files on any failure.
- **Rungs:** L1, L2, L4-L6. **UX:** J14 (author-experience metrics: first-try success rate, average fix loops). **UI:** builder preview goldens.

### WP3.10 Migrate built-ins onto the API  `[L-XL]`
- **Goal:** dogfood; built-ins are extensions.
- **Depends on:** WP3.3-WP3.8.
- **Read first:** `extensions/index.ts` (`INTERNAL_EXTENSION_FACTORIES`), `extension-host/internal-extension.ts`, `internal-extension-requirements.ts`.
- **Build:** one built-in per PR (calculator, web, emoji, system, places, notes, snippets, apps, files, clipboard...), each with its parity tests unchanged; internal extensions get the same API plus an allow-list of host-only capabilities documented in the PR.
- **Acceptance:** goldens and journeys unchanged after each migration; any missing primitive is added to the API (not bypassed) in the same PR.
- **Rungs:** L1-L6. **UX:** the migrated feature's journeys. **UI:** goldens identical.

## M4 OS depth

### WP4.1 Window management  `[XL]` (split in three PRs)
- **Goal:** macOS window management with the same guarantees.
- **Depends on:** WP1.11, WP3.8 (for the extension API), D4.
- **Read first:** `docs/window-management.md`, `electron/os/window-*.ts` (≈50 files), `extensions/window-management*.ts`, `macos-helper/window-*.swift`, `os/macos-window-manager.ts`, tests `window-management*.test.ts`.
- **WP4.1a Geometry:** layout catalog, gaps, cycling, steps, exact values, neighbors as pure functions with the existing test vectors ported.
- **WP4.1b Engine:** helper bridge (list, place, raise, full screen, displays), frame verification ("never report success unless reached"), queue, history/undo, failure taxonomy, access checks.
- **WP4.1c UI and shortcuts:** commands, settings (gap, cycling, sizes), custom commands pointer, global shortcuts that never steal focus, extension API `windowManagement`.
- **Acceptance:** ported vectors pass; simulated-window harness (as `window-management.simulated-window.ts`) for engine tests; macOS runner tests on a scripted app window; honest partial/failed messages.
- **Rungs:** L1, L2, L5 (required). **UX:** J19. **UI:** settings and result goldens.

### WP4.2 OCR and semantic file search  `[M-L]`
- **Goal:** `ctx.ocr` and "Find Files by Meaning".
- **Depends on:** WP1.7, WP2.1.
- **Read first:** `electron/files/ocr.ts`, `extensions/files-semantic-search.ts` and tests, `ExtensionOcr*` types.
- **Build:** OCR through a helper (Vision on macOS; decide Windows/Linux, omit where none), region/screen capture permissions, semantic search through the backend, capability checks.
- **Acceptance:** fixture images with known text; unsupported OS omits the command; privacy review of what is sent to the backend.
- **Rungs:** L1, L2, L5. **UX:** J07. **UI:** result goldens.

### WP4.3 Camera and webview views  `[M]`
- **Goal:** the two special view types.
- **Depends on:** WP3.3.
- **Read first:** `palette/extension-view/camera-view.tsx`, `camera-capture.ts`, `extension-webview.tsx`, `extension-webview-permissions.ts`, MyGo `docs/ui/web-view.md`.
- **Build:** camera capture view (permissions, frame capture to a file), webview view with a permission model and navigation policy (`electron/app/window-navigation-policy.ts`).
- **Acceptance:** permission denial paths; navigation policy table tests; resource cleanup.
- **Rungs:** L1, L2, L4-L6. **UI:** goldens.

## M5 Ship

### WP5.1 Packaging, signing, notarization  `[L]`
- **Goal:** installable, signed builds on all OS.
- **Depends on:** M1 at least.
- **Read first:** `electron-builder.yml`, `scripts/after-sign.cjs`, `build/entitlements.mac*.plist`, `.github/workflows/nightly-release.yml`, `promote-*.yml`, `docs/release-promotion.md`, `docs/packaged-app-size.md`, MyGo `docs/distribution.md`, `cmd/mygo` build docs.
- **Build:** `mygo build` pipelines: macOS (arm64+x64, Developer ID, notarization, entitlements for mic/camera/accessibility/automation, helper binary bundled and signed), Windows (installer or portable, signing), Linux (AppImage/deb or MyGo's formats), SBOM, `govulncheck` gate, size tracking (B6), reproducible builds.
- **Acceptance:** L7: install from artifact on clean VMs, launch, sign in against staging, hotkey works, helper executes, Gatekeeper/SmartScreen pass.
- **UX:** J25 on a clean machine. **UI:** installer and first-run screenshots.

### WP5.2 Updater and channels  `[L]`
- **Goal:** safe self-update.
- **Depends on:** WP5.1, WP2.1.
- **Read first:** `electron/app/update-*.ts`, `updates-service.ts`, `update-channel*.ts`, `update-downgrade.ts`, `windows-portable-updater.ts`, `docs/update-channels.md`, MyGo `updater.go`/`docs/updates.md`.
- **Build:** channel selection (stable/nightly/next), signature verification, downgrade guard, staged rollout, rollback to previous, portable updater on Windows, state compatibility check at launch, Updates view and menu item.
- **Acceptance:** L7 update from N-1 on each OS; interrupted download; bad signature rejected; downgrade refused; stores forward-compatible by one version.
- **UX:** J24. **UI:** update states goldens.

### WP5.3 Windows pass  `[L]`
- **Depends on:** M1-M3.
- **Read first:** `docs/windows-platform-inventory.json`, `docs/windows-release-readiness.md`, `electron/os/windows-*.ts`, `.github/workflows/` Windows jobs if any.
- **Build/validate:** every checklist feature on Windows (acrylic material, tray, hotkeys, `SendInput` paste, app icons, autostart, helper `windows-desktop-text`), fixes, per-feature "supported/omitted" decisions recorded in the inventory.
- **Acceptance:** J01-J30 relevant subset green on the Windows runner; sign-off sheet.

### WP5.4 Linux matrix  `[L]`
- **Depends on:** M1-M3.
- **Build/validate:** GNOME, KDE, Wayland and X11 matrix for hotkeys (portal or fallbacks), clipboard, tray hosts (AppIndicator), layer-shell panel, transparency without compositor, autostart; documented limitations; `linux-desktop-session.ts` behaviors ported.
- **Acceptance:** rig variants per DE (VM images) for J01, J05, J11, J26; sign-off sheet.

### WP5.5 Release engineering, docs, cutover plan  `[M-L]`
- **Goal:** everything around the code.
- **Depends on:** WP5.1, WP5.2.
- **Build:** CI for nightly and promote flows for the Go app, `AGENTS.md`/`CLAUDE.md` rewrite for Go, generated extension API reference replacing the `.d.ts` as source of truth, data directory naming and coexistence rules with 1.x, optional read-only importer (extensions source, snippets, notes) with a dry-run report, user-facing release notes and 2.0 announcement, rollback procedure, support playbook.
- **Acceptance:** dry-run of promote and rollback; importer never writes to 1.x data (test); docs reviewed.

### WP5.6 Beta, dogfood gate, parity review, cutover  `[L]`
- **Goal:** the decision to replace Electron.
- **Depends on:** all.
- **Do:** two-week beta on the `next` channel with the owner and volunteers; telemetry on crashes and budgets (within privacy rules); full J01-J30 on all OS; parity review of every checklist box (accepted differences listed); data-loss bug count must be zero; budgets B1-B8 met; go/no-go meeting; cutover or extend.

---

## Risk register

| Risk | Impact | Mitigation | WP |
|---|---|---|---|
| Agent loop (pi) has no Go equivalent | builder and AI actions slip | spike first, sidecar fallback, interface `ai.Agent` | WP2.4 |
| Yaegi gaps or breakage on Go bumps | extensions fail to load | stdlib coverage report, version pin and CI matrix, subprocess option | WP3.1 |
| MyGo is young (v0.7) | missing OS features, API changes | pin version, contribute upstream, keep platform ports thin | WP0.1, WP0.2 |
| macOS-only features untestable without runners | unverified claims | self-hosted macOS runner, "done pending L5" rule | WP0.3 |
| Data loss in stores or uninstall | trust | WP0.6 first, guards and fault injection, destructive flows last in their WPs | WP0.6, WP1.4 |
| Visual drift between OS | inconsistent product | goldens per platform, tokens only, review sheets | WP0.4, WP0.8 |
| Scope: 100+ checklist items | never finishes | milestones with exit criteria, daily-driver M1 first | section 7 |
| Hidden behavior in 1.x not in the checklist | regressions | parity review per feature, 20-query comparison, 1.x tests ported as vectors | WP1.2, WP5.6 |
| Performance regressions | the reason for the rewrite is lost | budgets enforced in CI from M0 | WP0.5 |
| Clipboard and screen privacy | trust and legal | redaction canaries, concealed handling, analytics schema test | WP1.6, WP1.12 |

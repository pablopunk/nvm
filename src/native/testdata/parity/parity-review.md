# Parity review sheet: Electron 1.x to Go 2.0

First pass, built from the progress table in [PLAN.md](../../PLAN.md) and by reading `src/app` (the 1.x source) and
`src/native/internal`. It is the input to the parity review of PLAN section 3.3 and cutover.md section 6. It is not
the review itself: no side-by-side run, no 20-query comparison and no owner sign-off has happened yet.

## Legend

**Go status**

- `done`: the PLAN row lists no gap for this feature, including macOS and Windows verification. Infrastructure WPs only.
- `partial`: the Go code exists, but the PLAN row lists a gap, or the proof is Linux-only, or the extension surface is
  unfinished.
- `missing`: no Go code path for the feature or surface.
- `unknown`: the PLAN does not track it, and this pass found no test or WP evidence. Needs a check before the review.

**Platforms run** says where the feature has run in the real app or in a test. `Linux` means the rig or the box.
`compile` means a macOS or Windows build only. `none` means nothing ran there.

**Electron feature** names the 1.x behavior, with its 1.x source file in parentheses. Paths are relative to the repository root.

## Summary

| Status | Count |
|---|---|
| done | 5 |
| partial | 79 |
| missing | 6 |
| unknown | 4 |

The five `done` rows are three palette core rows (A2, A3, A5), the generated API reference (E14) and the reference
screenshots (F2). Every other row lists a gap. Most gaps are missing macOS or Windows proof (PLAN M1 and M5 exits). Read the
Gap column before deciding a row.

## A. Palette core

| ID | Electron feature (1.x source) | Go status | Platforms run | Go code / WP / proof | Gap |
|---|---|---|---|---|---|
| A1 | Palette window, summon hotkey, placement on the cursor's display (`src/app/electron/app/palette-window.ts`, `palette-shortcuts.ts`) | partial | Linux; macOS and Windows compile | WP1.3; `internal/shortcuts`, `internal/palette/placement.go`; rig J01, J11 | J25 and J26 (multi-display, Spaces, full screen), focus return to the previous app, remembered offset (owner decision), tiling-manager check, hotkey does not hide a shown palette |
| A2 | Command-K action panel and keyboard contract (`src/app/palette/action-panel.tsx`) | done | Linux (L1, L2) | WP1.1; `internal/palette/actions.go`, `keys.go`, `focus.go` | none listed |
| A3 | Search coordinator: cached results first, cancellation, stale-result guard (`electron/search/search-coordinator.ts`) | done | Linux (L2) | WP1.1; `internal/search`; provider that blocks 2 s does not delay typing | none listed |
| A4 | Ranking and the 20 canned queries (`src/app/palette/search-ranking.ts`, `testdata/queries.txt`) | partial | Linux (L1) | WP1.2; `internal/search` ranking tests | The 20 queries are in `testdata/queries.txt`, read by the root table test; the 20-query comparison with 1.x has not run |
| A5 | Usage learning, frecency, query history of 100 entries (`electron/state/learning-store.ts`) | done | Linux (L1, L2) | WP1.2; `internal/search/usage` | none listed |
| A6 | "No shortcut set" banner (`palette-hotkey-notice.tsx`) | partial | Linux (rig) | WP1.3; `internal/palette/notice.go` | saving a taken shortcut in the rig is not done |
| A7 | Keyboard shortcuts manager and recorder (`electron/shortcuts/*`, `palette/shortcut-manager.tsx`) | partial | Linux (rig J01, J11, J22) | WP1.11; `internal/shortcuts` | extension-declared shortcuts (no extension host shortcut path), J11 launch-at-login, macOS and Windows runs |
| A8 | Rotating placeholders and empty states | unknown | none | not tracked by PLAN | check against `src/app/palette/App.tsx` |

## B. Built-in extensions (`src/app/extensions/index.ts`, 24 factories)

| ID | Electron feature (1.x source) | Go status | Platforms run | Go code / WP / proof | Gap |
|---|---|---|---|---|---|
| B1 | System: lock, sleep, restart, shut down, log out, empty Trash, System Settings, permission rows (`system.ts`) | partial | Linux (L1, L2, rig J25 in progress) | WP1.10; `internal/features/system`; goldens os-permissions | J23 not run (it reboots the box), Windows empty Trash missing, some icons missing (camera, accessibility, monitor, shield-check), macOS and Windows L5/L6 not run; not an extension yet (WP3.10 blocker: shut-down, log-out, empty-trash ops) |
| B2 | Places (`createPlacesExtension`) | partial | Linux | WP1.10 (`places` capability) | not an extension: needs an open-path host action and a places list (WP3.10) |
| B3 | Calculator, units, currency, copy and paste actions (`calculator.ts`, `electron/search/calculator.ts`) | partial | Linux (L1, L2, rig J03 smoke) | WP1.5; `internal/features/calc` | live rate source not verified; macOS and Windows not run; not an extension (needs a set-query action, root refresh, Network-backed rates) |
| B4 | Web search, Open URL, quicklinks, Create Quicklink (`web-search.ts`, `electron/search/search-utils.ts`) | unknown | Linux (not stated) | `internal/app/providers.go` (`builtinLinks`, query fallbacks); no WP row | no PLAN row and no test evidence found; check the `Search the web` and `Open <site>` rows against 1.x |
| B5 | Clipboard: history, pin, delete, paste, preview, concealed skip, ask AI (`clipboard.ts`, `electron/clipboard/*`) | partial | Linux (rig J01, J02, J03, J05, J06, J08, J09, J10, J22 on a private display) | WP1.6; `internal/features/clipboard`, `internal/app/clipboard*.go`; rig J05, J06 | paste into another app not verified on the rig, live link fetch unverified, macOS paste and concealed markers untested, Windows paste unsupported |
| B6 | Dictation: start, speak, stop, paste; settings page (`dictation.ts`, `electron/dictation/*`) | partial | Linux (rig J20 pipeline on a private PulseAudio) | WP2.6; `internal/dictation`, `internal/app/dictation_settings.go` | macOS and Windows capture, palette-to-paste journey, text target for paste, cancel control in the palette, history and recordings, vocabulary, AI cleanup, screen context, permission flow, live backend smoke |
| B7 | AI commands on selected text: Fix, Triage, Classify (`ai-commands.ts`, `ai-decision-triage.ts`, `ai-image-decision.ts`) | partial | Linux (rig J21, fake backend) | WP2.5; `internal/ext/builtins/aicommands` (Triage on the API), `internal/ai/commands` (Fix logic) | macOS accessibility read and L5, Windows, file picker for Classify, streamed text on the result page, full clipboard snapshot (formatted clipboards are refused) |
| B8 | Emoji and symbols: grid, categories, skin tone, recents, Copy, Paste (`emoji-symbols.ts`, `emoji-symbol-data.ts`) | partial | Linux (L2, goldens, rig J08 smoke) | WP1.8; `internal/app/emoji_*.go`, `internal/palette/emoji` | macOS and Windows not verified |
| B9 | Apps: index, icons, running status, launch, first seen (`apps.ts`, `electron/app/app-index-service.ts`) | partial | Linux (rig J22) | WP1.4 (in progress); `internal/features/apps` | app icons still on the synchronous path, palette L2 tests, macOS discovery tested with fakes only, Windows discovery unavailable |
| B10 | Quit apps: Force Quit Apps, Quit All Apps (`quit-apps.ts`, `electron/os/app-quit.ts`) | partial | Linux (rig J22) | WP1.4 | macOS and Windows quit not verified |
| B11 | Uninstall apps with a review list before Trash and hostile-path guards (`electron/app/app-uninstall-*.ts`) | partial | Linux (rig J22 path); macOS tested with fakes | WP1.4 | uninstall review and confirm goldens not done; Windows unavailable. Destructive: keep it behind the guard tests |
| B12 | Files: find, preview, reveal, open, drag out, Quick Look, open with (`files.ts`, `electron/files/*`) | partial | Linux (L1, L2, box checks) | WP1.7 (library), WP1.7b (wiring); `internal/features/files` | open-with, Quick Look, folders, rig J07 not run, drag-out untested at runtime, macOS and Windows watch unverified |
| B13 | Find Files by Meaning (`files-semantic-search.ts`) | partial | Linux (L1, L2 goldens) | WP4.2; `internal/app/files_meaning.go` | no embeddings (same as 1.x); sends the 16 most recent file names to the decision route; macOS L5 not run; root command only while signed in |
| B14 | Floating notes: Markdown, autosave, conflict resolution, floating windows restored at launch (`floating-notes.ts`) | partial | Linux (L1, L2; rig J09, J10 smokes) | WP1.9; `internal/features/notes` | real paste not implemented (copy fallback); macOS and Windows not run; crash-recovery and two-instance tests exist |
| B15 | Snippets: keywords, placeholders, paste (PLAN WP1.9 treats this as parity) | unknown | Linux (L1, L2; rig J09 smoke) | WP1.9; `internal/features/snippets` | no snippets built-in found in `src/app`; the 1.x API docs mention snippets only as extension examples. Owner to confirm whether 1.x had snippets. Real paste is not implemented (copy fallback); macOS and Windows not run |
| B16 | Extensions manager: list, enable, disable, review, delete, failure rows (`extensions.ts`) | partial | Linux (rig J15, J16 on :88) | WP3.2; `internal/app/extensions_*.go`, `internal/ext/manager` | macOS and Windows runs; memory-bomb and three-failure disable paths not rig-tested; a file dropped while the app runs not rig-tested; 100 ms keystroke budget not measured |
| B17 | AI builder: Automate, write, check, fix loop, install, Edit with AI (`ai-builder.ts`, `electron/ai/ai-builder-*.ts`) | partial | Linux (rig J14 and edit journey green) | WP3.9; `internal/ai/builder`, `internal/ext/manager` | 30-prompt eval harness not built, read-page, file and preview tools, proposals and PR submitter, window-management guidance, author-experience metrics |
| B18 | Permissions view and prompts (`permissions.ts`, `electron/os/*`) | partial | Linux (L1, L2) | WP1.10 | permission state reads are unknown on every OS; prompts only when a command needs them (J25) not verified |
| B19 | Updates: check, download in the background, restart prompt, channels, downgrade refusal (`updates.ts`, `electron/app/update-*.ts`) | partial | Linux (L1, L2; rig J24 on a loopback feed with a fake key) | WP5.2; `internal/update`, `platform/real` `NewSelfUpdater` | no published feed run, L7 from N-1 not run on any OS, macOS install waits for quit untested, Windows portable updater missing, Windows Authenticode for updates missing, no rollback |
| B20 | Keyboard shortcuts list and recorder (`keyboard-shortcuts.ts`) | partial | Linux (rig J01, J11, J22) | WP1.11 | extension shortcuts (see D19), macOS and Windows runs |
| B21 | Window management: halves, thirds, cycling, undo, displays, settings (`window-management*.ts`, `electron/os/window-*.ts`) | partial | Linux X11 (rig J01, J11, J19 on X11) | WP4.1a (done), WP4.1b (done pending L5), WP4.1c; `internal/windowmgmt` | macOS L5 for J19, next and previous desktop (macOS AppleScript), Windows backend missing, full screen unsupported on X11, extension API `windowManagement` not done |
| B22 | Settings: hotkey, engine, clipboard, launch at login, Welcome and Help (`settings.ts`, `electron/app/settings*.ts`) | partial | Linux (rig J11 for hotkey) | WP1.10, WP1.3; `internal/features/settings` | launch-at-login J11 not done, appearance not ported (no 1.x behavior), macOS and Windows runs |
| B23 | Background tasks: status, triggers, run history, Run Now, enable or disable (`background-tasks.ts`) | partial | Linux (rig J15, J16, J18 on :73) | WP3.6; `internal/jobs`, `internal/palette/background` | no-overlap guarantee only in unit tests, interval and wake triggers not in the rig, menu bar items on the trigger host, macOS and Windows runs, frontmost poll cost not measured |
| B24 | Diagnostics: recent errors, copy reference, report a problem, analytics consent (`diagnostics.ts`, `electron/diagnostics/*`, `docs/posthog-observability.md`) | partial | Linux (L1, L2, goldens) | WP1.12; `internal/diag`, `internal/diag/analytics` | performance traces, `$pageview` and sessions, owner events, live ingest smoke, rig J25 and J28 not run, macOS and Windows runs |
| B25 | Account: device sign-in, deep link, sign-out, environment switch, BYO key, spend-cap notice (`account.ts`, `nevermind-sign-in.ts`, `byo-key.ts`) | partial | Linux (L1, L2, L3 goldens) | WP2.2; `internal/backend/auth`, `internal/palette/account` | live sign-in not run; `nvm://` collides with 1.x (cutover.md section 2); the plan shows as unknown until the backend has a bearer route that reports it (cutover.md section 4) |
| B26 | Getting Started guide and the Welcome row (`getting-started.ts`) | partial | Linux (L3 goldens) | WP1.10 | rig J25 (first run, permissions only when needed) not run |
| B27 | Menu bar item: Open, Settings, Check for Updates, Quit, timer title (`menubar.ts`, `electron/app/menubar-composition.ts`) | partial | Linux (rig J-tray) | WP1.12, WP3.5; `internal/menubar` | popovers, subtitles, checkmarks, shortcuts, glyph icons (placeholder disc), triggers other than interval and startup, visibility toggle, macOS and Windows runs |

## C. Other 1.x features outside the built-in list

| ID | Electron feature (1.x source) | Go status | Platforms run | Go code / WP / proof | Gap |
|---|---|---|---|---|---|
| C1 | AI chat: Tab to ask, streaming, stop, regenerate, chat list, model tiers, 24 h retention (`src/app/electron/ai/ai-chat-*.ts`, `palette/ai-chat/*`) | partial | Linux (L1, L2, L3 goldens) | WP2.3; `internal/ai/chat`, `internal/palette/aichat` | rig J13 and J28 not run (packaged builds refuse the fake origin), images and thumbnails, per-message copy and retry, macOS and Windows runs, live smoke |
| C2 | Agent tools in chat: search and read files, open apps and links, read and copy the clipboard, move to Trash, with approvals (`src/app/electron/ai/ai-tool-policy.ts`) | partial | Linux (L1, L2, goldens) | WP2.4; `internal/ai/agent`, `internal/ai/tools` | web search and fetch with egress approval, Responses and Messages wires, compaction, window and quit tools, rig J13 and J14, live backend smoke |
| C3 | Compatibility and update-recommended notices from the backend (`src/backend/src/lib/compatibility.ts`) | partial | Linux (L1) | WP2.1; `internal/backend/compat.go` | live smoke not run; the manifest's `latestVersion` and `updateUrl` are not read until the 2.0 client name ships (cutover.md section 4) |
| C4 | Ask-AI image classification and OCR command (`electron/files/ocr.ts`) | partial | Linux (L1, L2, goldens; Tesseract when installed) | WP4.2; `internal/features/ocr` | macOS Vision and screen capture unrun (L5), Windows OCR missing, rig journeys |
| C5 | Camera and webview views (`camera-view.tsx`, `extension-webview.tsx`) | partial | Linux (rig camera refusal, webview paints) | WP4.3; `internal/palette/extview` | capture to file or clipboard (no 1.x parity), saved camera not kept on disk, macOS and Windows runs, camera permission copy (WP5.1) |
| C6 | Launch at login (`electron/app/settings-service.ts`) | partial | Linux (rig J11 not run) | WP1.10 (Autostart port) | J11 launch-at-login not run; macOS and Windows runs |
| C7 | Tray and menu bar placement per OS (`electron/os/menubar-status-items.ts`) | partial | Linux (rig tray registration) | WP1.12, WP3.5 | macOS and Windows runs; the 1.x menu bar window placement is not ported |
| C8 | Feedback indicator (`src/app/extensions/feedback.ts`, an extension indicator) | missing | none | no `indicators` service in `internal/ext/services` | extension indicators are not in the Go services (WP3.7 lists indicators as not done) |

## D. Extension API: context namespaces (`src/app/resources/nevermind-extension-api.d.ts`, `ExtensionContext`)

The 1.x context has 31 namespaces. The Go `api.Context` interface has the same set except `state`, `extensions` and
`ui` (its UI is `Views`, `Navigation`, `Input`). Go adds `Identity`, `Clock`, `Network` and `Indicators`, which 1.x
does not expose here. Service handlers live in `src/native/internal/ext/services`.

| ID | Electron namespace (`ctx.*`) | Go status | Platforms run | Go code / WP / proof | Gap |
|---|---|---|---|---|---|
| D1 | `extension` (identity, rename) | partial | Linux (rig J15, J16) | WP3.2; `internal/ext/manager`, `api/manifest.go` | rename is not described in PLAN |
| D2 | `command` (command bodies, aliases) | partial | Linux (L1, L2; rig J15, J16) | WP3.2; `internal/ext/api/command.go` | global shortcuts bound to commands (WP3.8 not done) |
| D3 | `launch` (launch context) | partial | Linux | WP3.2; `api.Context.Launch()` | host population of the launch context not verified in PLAN |
| D4 | `ui.list`, `ui.grid`, `ui.collection`, `ui.preview`, `ui.previewFile`, `ui.form`, `ui.editor`, `ui.camera`, `ui.webview`, `ui.empty`, `ui.loading`, `ui.error`, `ui.confirm` | partial | Linux (L1, L2, L3: 14 view states; forms and editor goldens; camera and webview goldens) | WP3.3, WP3.4, WP4.3; `internal/palette/extview`; fixture `acme.views` | grid layout and aspect options not done, progress bar not done, WP3.3 J14 and J27 not run; the form and editor goldens cover 8 states |
| D5 | `ui.chat` | missing | none | not in `extview`; the chat view is host-only (WP2.3) | an extension chat view is not built |
| D6 | `ui.progress` | partial | Linux (view exists, `extview/progress.go`) | WP3.3 | progress bar not done |
| D7 | `ui.indicator` (show, hide) | missing | none | no `indicators` service in `internal/ext/services` | extension-facing indicators not built (WP3.7 lists them as not done) |
| D8 | `actions` (run, open, copy, push, pop, confirm) | partial | Linux (L2, rig J15, J16) | WP3.3; `internal/palette/extview` action handles | run, open-url, copy-text, push-view, pop and camera only (WP3.10 notes): other host actions are not implemented |
| D9 | `system` (metadata, operations) | partial | Linux | WP3.8; `internal/ext/services/system.go` | metadata only; shut down, log out and empty Trash are not declared (WP3.10 blocker) |
| D10 | `text` (templates, text helpers) | partial | Linux (L1) | WP3.7; `internal/ext/services/text.go`, `template.go` | Gaps of WP3.7 apply (drafts, views of data sources) |
| D11 | `input` (prompts, one field at a time) | partial | Linux (L2; rig `TestJourneyWP34FormSubmitAndDraftSurvivesKill`) | WP3.4; `internal/ext/api/prompt.go` | `Result.Draft` navigation and the Autosave action not done |
| D12 | `windows` (list, focus, place, layouts) | partial | Linux (X11 rig) | WP3.8; `internal/ext/services/desktop_placement.go`, `internal/windowmgmt` | extension windows with restore, frontmost-window placement, neighbor focus; macOS and Windows runs |
| D13 | `clipboard` (text, history: list, get, remove, clear) | partial | Linux (rig J-desktop, fixtures acme.desktop, acme.undeclared) | WP3.8; `internal/ext/services/clipboard_history.go`, `desktop_clipboard.go` | non-text formats, keyboard and selection, rate limits per extension are in place; macOS and Windows runs |
| D14 | `ocr` (image, screen, region) | partial | Linux (L1, L2 goldens) | WP3.8; `internal/ext/services/ocr.go` | macOS L5 unrun, Windows OCR missing |
| D15 | `navigation` (root, push, replace, pop, run) | partial | Linux (L2) | WP3.3, WP3.4; `api/contract.go` Navigation | `Result.Next` runs as a follow-up only; some navigation tests cover the pop path only |
| D16 | `desktop` (apps, files, shell, clipboard, windows, selection) | partial | Linux (rig J-desktop) | WP3.8; `internal/ext/services/desktop_*.go` | keyboard, selection, non-text clipboard, frontmost app, process execution, file operations beyond open and reveal, neighbor focus |
| D17 | `storage` (namespaced store, quotas) | partial | Linux (L1, rig J15, J16) | WP3.7; `internal/ext/services/storage.go` | quota and isolation tests exist; macOS and Windows runs |
| D18 | `secrets` (keychain-backed, canary) | partial | Linux (fake keychain port) | WP3.7; `internal/ext/services/secrets.go` | MyGo secret service not exercised on the box; keychain fake only |
| D19 | `shortcuts` (read, declare global) | partial | Linux | WP3.8; `internal/ext/services/shortcuts.go` | declared global shortcuts (`Command.GlobalShortcut`) not done; read only |
| D20 | `logs` (write, recent) | partial | Linux (L1) | WP3.7; `internal/ext/services/logs.go` | settings view goldens not done (WP3.7) |
| D21 | `cache` (runtime cache, TTL) | unknown | none | no runtime-cache file in `internal/ext/services`; WP3.7 describes only the data-source cache (`data.go`) | check whether `ctx.cache` is served; WP3.7 does not say |
| D22 | `views` (refresh, invalidate) and `menuBar.refresh` | partial | Linux (rig J-tray) | WP3.3, WP3.5; `api/contract.go` Views | view patching of data sources not done (WP3.3) |
| D23 | `drafts` (save, conflict, resolve) | partial | Linux (rig TestJourneyWP34FormSubmitAndDraftSurvivesKill) | WP3.4; `internal/ext/services/drafts.go` | the Autosave action, `Result.Draft`, draft conflicts in the rig |
| D24 | `settings` (declared settings, write) | partial | Linux (L1; rig J15, J16) | WP3.7; `internal/ext/services/settings.go` | settings view goldens (WP3.3, WP3.4) |
| D25 | `dictation` (record, transcribe) | missing | none | interface in `api.Context.Dictation()`; no handler in `internal/ext/services` | the extension dictation service is not built (WP3.8 not done) |
| D26 | `updates` (getState) | missing | none | no `updates` service in `internal/ext/services` | not built (WP3.7 lists updates as not done) |
| D27 | `state` (`Record<string, unknown>`) | missing | none | no `State()` in `api.Context` | no Go counterpart; decide whether the API needs it before extensions depend on it |
| D28 | `data` (loaders, refresh, stale cache) | partial | Linux (L1, rig J15) | WP3.7; `internal/ext/services/data.go` | view patching of data sources (WP3.3) |
| D29 | `ai` (streams, models, attachments) | partial | Linux (L1, L2; rig J21 fake) | WP2.4, WP3.9; `internal/ext/api/services_ai.go` | live backend smoke; streams from the live backend not recorded |
| D30 | `aiBuilder` | partial | Linux (rig J14) | WP3.9; `internal/ai/builder` | eval harness and proposals (see B17) |
| D31 | `extensions` (ownership) | partial | Linux (L1) | WP3.9; `api.Context.Ownership()`, capability `extensions.ownership` | the eval harness and PR submitter (WP3.9) |

## E. Extension declarations and runtime behavior

| ID | Electron feature (1.x source) | Go status | Platforms run | Go code / WP / proof | Gap |
|---|---|---|---|---|---|
| E1 | Triggers: startup and interval (`electron/extension-host/extension-triggers.ts`) | partial | Linux (rig J15, J16, J18 on :73) | WP3.6; `internal/jobs/extjobs` | interval and wake triggers not in the rig |
| E2 | Triggers: `files.changed` and `clipboard.changed`, debounce, no overlap | partial | Linux (unit tests) | WP3.6 | no-overlap guarantee in the rig not done |
| E3 | Trigger: `app.frontmost.changed` | partial | Linux (unit tests) | WP3.6 (poll runs only while a command listens) | poll cost not measured |
| E4 | Triggers: `wake` and `login` | partial | Linux (simulated wake) | WP3.6 | real suspend and login not run |
| E5 | Background modes: view, noView, background | partial | Linux | WP3.6 | none beyond E1 to E4 |
| E6 | Forms: 16 field types with validation (`palette-form.tsx`, `form-fields.ts`) | partial | Linux (L2 per field, goldens 8 states) | WP3.4; `internal/app/testdata/golden/forms` | goldens for every field type, pickers on macOS and Windows |
| E7 | Editor view with drafts and conflict resolution (`markdown-editor.tsx`) | partial | Linux (L2) | WP3.4 | the editor is a plain text area: the code editor and the rich text editor are not used; Autosave action not done |
| E8 | File, files and folder pickers (1.x file, folder and files pickers) | partial | Linux (L2) | WP3.4 (`platform.FileDialog`) | native dialog in the rig not done; macOS and Windows dialogs |
| E9 | Menu bar contributions with icons, rows, refresh (`electron/os/menubar-status-items.ts`) | partial | Linux (rig J-tray) | WP3.5 | popovers, subtitles, checkmarks, shortcuts, glyph icons, visibility toggle |
| E10 | Capability declaration and review (`extension-capabilities.ts`) | partial | Linux (L2, goldens) | WP3.2, WP3.7, WP3.8 | undeclared use is refused and logged (WP3.8 fixtures); the review UI on macOS and Windows |
| E11 | Failure isolation: panic, hang, flood, memory (`extension-rate-limits.ts`) | partial | Linux (rig J15 fixtures) | WP3.2 | memory-bomb and three-failure disable paths not rig-tested; macOS and Windows memory cap missing (resident watch only) |
| E12 | Atomic install, uninstall and hot reload (`extension-runtime-transaction.ts`) | partial | Linux (L1) | WP3.2, WP3.9 | a file dropped while the app runs is not rig-tested |
| E13 | Dropped-in extensions disabled by default (`docs/extension-trust-model.md`) | partial | Linux (rig J16) | WP3.2 | enable after review on macOS and Windows |
| E14 | API reference and AI builder context (`src/app/resources/nevermind-extension-api.d.ts`, SKILL.md) | done | Linux (L1, drift test) | WP3.9; `internal/ai/builder/apiref` generated from the Go api, size budget and drift test | the 1.x `.d.ts` remains the source of truth in AGENTS.md until cutover (a doc decision, not a gap) |
| E15 | Shortcuts declared on extension commands | partial | Linux | WP1.11 (model in place) | extension shortcuts not wired to the host (no extension host shortcuts in the palette) |

## F. Test assets and evidence

| ID | Asset | Go status | Platforms run | Location | Gap |
|---|---|---|---|---|---|
| F1 | 20 canned queries for the parity comparison (PLAN WP1.2, section 3.3) | partial | none | `testdata/queries.txt`, read by the root table test in `internal/app` | the side-by-side comparison with 1.x has not run |
| F2 | Electron reference screenshots (WP0.4) | done | Linux | `src/native/testdata/reference/` (`ai-chat.png`, `clipboard-history-preview.png`, `palette-dotted-connectors-*.png`, `root-search-automate.png`, `root-automate-tab-hint.png`) | references are for human review only |
| F3 | Rig journeys J01-J32 (PLAN section 3.1) | partial | Linux | `src/native/internal/rigtest/journeys_test.go`, `scripts/rig/` | J13, J14 (partial), J23, J25, J26, J27, J28, J29, J30 (rig) not run or not green; macOS and Windows journeys not run |
| F4 | Performance budgets B1-B8 (WP0.5) | partial | Linux (box numbers) | `src/native/testdata/budgets.json`, `perf-baseline.json` | budgets measured on Linux; B1 not measured on macOS or Windows; B6 size tracking not done (WP5.1); B7 soak not run |
| F5 | Golden review sheets | partial | Linux | `src/native/testdata/golden/wp32`, `wp33`, `internal/app/testdata/golden/*` | macOS and Windows goldens not run; owner sign-off per platform not recorded |

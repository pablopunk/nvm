# Cutover: from the Electron app to Go 2.0

Status: draft for owner review (PLAN WP5.5, milestone M5). Nothing here has run. No native release has been cut, no
backend change has shipped, and the owner decisions at the end are open. Cutover itself is a release decision at
WP5.6, not a code change.

Names used below. **1.x** is the Electron app. Its released strings are date-stamped (`2026.1104.1`, see
`docs/release-promotion.md`), so "1.x" is the product label, not the version. **2.0** is this Go app, shipped as
"Nevermind Next" (`src/native/mygo.json`) until the owner says otherwise.

## 1. Side-by-side install

2.0 installs next to 1.x and never touches its data. The two apps differ in every identity the OS and the backend
can see:

| Item | 1.x | 2.0 (today) | Source |
|---|---|---|---|
| Bundle identifier | `com.pablopunk.nvm` | `com.pablopunk.nvm.next` (permanent once shipped) | `mygo.json`, `release.md` |
| App name, binary, slug | Nevermind | Nevermind Next, `nevermind-next`, `nevermind-next` | `mygo.json` |
| Local data | the Electron user-data folder, `~/.config/nevermind/extensions` for extensions | `~/.config/nvm-native` on every OS | `store.go`, `extension-paths.ts` |
| Keychain service | 1.x entries | `nvm-native` | `auth/vault.go` |
| URL scheme | `nvm` | `nvm` today (collision, section 2) | `deep-link.ts`, `mygo.json` |
| Update feed | GitHub releases `v...`, read through `release-pointers` and GitHub's latest stable | GitHub releases `native-v...` (never "latest") | `update-channels.md`, `release.md` |
| Backend identity | `X-Nevermind-Client: desktop`, version `YYYY.MMDD.N` | same name, version `2.0.0-next.N` | `nevermind-api.ts`, `identity.go` |

Things to settle before both apps run on one machine:

- **Palette hotkey.** 2.0 defaults to `Alt+Shift+Space` (`internal/shortcuts/system.go`). If 1.x uses the same
  default, the second app fails to register it and shows the "No shortcut set" banner. Check the 1.x default and,
  if they match, give 2.0 a different default for the overlap period.
- **Launch at login.** Each app has its own setting. Check the 2.0 default: two apps launching at login means two
  menu bar items.
- **macOS privacy grants** (Accessibility, Apple Events, microphone, camera) belong to the bundle identifier. A
  user grants them again for 2.0.
- **Uninstall.** Removing one app never removes the other's data. Say so in the install notes.
- **Updates never cross.** 1.x's updater cannot offer 2.0 (different bundle identifier), and 2.0's updater reads only
  `native-v` releases. Users who want 2.0 install it by hand (section 5).

## 2. Identity and URL scheme

**The collision.** Both apps claim `nvm://`. The backend sign-in approval page builds the link
(`src/backend/src/pages/auth/device.astro`, `src/backend/src/pages/profile.astro`). The device code is exchanged
with a poll, and each code is one-time (`deviceCodes.consumedAt`). If the OS hands the link to 1.x while a 2.0 user
is signing in, 1.x consumes the code, the 2.0 poll gets `consumed`, and the 2.0 sign-in fails. The fallback in the
approval page (copy the code and paste it into the app) works in both apps, so no user is locked out.

**Options.**

| Option | What | Cost | Verdict |
|---|---|---|---|
| A. Accept the collision | 2.0 keeps `nvm` | Sign-in can fail or land in 1.x | Only for dogfood on a machine without 1.x |
| B. Stop claiming `nvm` | 2.0 removes `nvm` from `urlSchemes` and signs in by pasting the code | Worse first-run sign-in for 2.0 | Interim, until B2 ships |
| B2. Scheme per client | 2.0 uses `nvm-next`; `POST /api/auth/device/initiate` takes an optional allow-listed `callbackScheme`, stored on the device code (new nullable column, default `nvm`); the approval pages build the link from it | Backend migration, two page changes, one contract fixture, one Go line in `mygo.json` | **Recommended** |
| C. No deep link at all | Paste-code only, for both | Removes a 1.x feature for nobody's benefit | Not recommended |

Why B2 is safe for 1.x: released 1.x clients never send `callbackScheme`, so they keep receiving `nvm://` links. The
change is additive under `docs/backend-api-compatibility.md`, and it needs no 1.x release.

Other rules that stay: 2.0 accepts a link only from its own scheme and host (`auth`); a link cannot move the user to
an untrusted origin (`ParseDeepLink`, `internal/backend/auth/deeplink.go`).

**Identity name.** "Nevermind Next" stays for the overlap. Renaming to "Nevermind" at cutover is possible without
changing the bundle identifier. The bundle identifier cannot change after the first public release.

## 3. Data

2.0 is a breaking release. There is no automatic migration (PLAN D5). 2.0 never reads or writes `state.json`,
`~/.config/nevermind/extensions`, or 1.x keychain entries (PLAN section 0.2). 1.x keeps every byte it has.

**Users keep** (in 1.x, untouched by 2.0):

- Everything in 1.x, including clipboard history, notes, chats and extensions. 1.x keeps working.
- Their account. Accounts and plans live on the backend, so a user who signs in to 2.0 sees the same account.
  2.0 holds its own token, in its own keychain entry.

**Users lose** when they move to 2.0 (unless the optional importer, section 9, is built and they run it):

- Clipboard history, quick notes, AI chats, palette query history and usage learning. 2.0 starts empty.
- Extensions. 1.x extensions are JavaScript or TypeScript modules. 2.0 runs Go source bundles through its own API,
  so no 1.x extension runs in 2.0. Users rebuild them with Automate (the AI builder) or wait for a built-in.
- Per-extension storage and secrets, settings, and shortcut customizations. Secrets and BYO keys must be entered
  again.
- Dictation settings and recordings. Window-management settings are reset to 2.0 defaults.
- Sign-in. The user signs in to 2.0 once.

**What to tell users** (draft; copy review is required, PLAN section 3.3):

> Nevermind Next is a new app. It installs beside Nevermind and starts empty: no clipboard history, notes or chats,
> and it does not run Nevermind extensions. Nothing in Nevermind changes. Sign in again in Nevermind Next. Keep
> Nevermind until Nevermind Next has what you use. Uninstalling one app never deletes the other's data.

**Rollback of data.** 2.0 stores are versioned and forward-compatible by one version (PLAN section 4). Going back to 1.x
loses nothing from 1.x, because 2.0 never wrote there. 2.0 data stays in `~/.config/nvm-native` until the owner
decides otherwise (section 10).

## 4. Backend compatibility while both clients run

Both apps send the same five compatibility headers (`X-Nevermind-Client`, `-Client-Version`, `-API-Version`,
`-Platform`, `-Arch`). Checked against `src/backend/src/lib/compatibility.ts`, `src/backend/src/lib/desktop-version.ts`
and `src/native/internal/backend/compat.go`:

1. **API major.** Both send `1`; the backend supports `[1]`. Compatible.
2. **One minimum for every client.** `unsupportedClientReason` compares `X-Nevermind-Client-Version` with
   `NEVERMIND_MIN_DESKTOP_VERSION` for every caller, whatever its product line. The backend gates on version and API
   major only, not on the client name.
3. **The version orders are different.** 1.x releases are date-stamped (`2026.1104.1-dev`, `2026.1102.2-nightly`
   appear in the releases list), so they sort above `2.0.0-next.0`. Consequences that break 2.0 silently:
   - If the owner ever sets the minimum to any 1.x release, every 2.0 build gets `426 unsupported_desktop_version`.
   - If `NEVERMIND_LATEST_DESKTOP_VERSION` holds a 1.x release, 2.0 reports "update recommended", and its
     `updateUrl` (default `.../releases/latest`, which is the 1.x line) points to the 1.x download.
   The repository's `.env.example` has minimum `0.0.0` and an empty latest version. The production values are
   not in the repository and the owner must read them from the deployment (decision D3).
4. **Required fix before 2.0 reaches anyone (engineering, backend, additive).** Give the 2.0 line its own client name
   (for example `nevermind-next`) and key the minimum, latest version and update URL by client name:
   `NEVERMIND_NEXT_MIN_DESKTOP_VERSION`, `NEVERMIND_NEXT_LATEST_DESKTOP_VERSION`, `NEVERMIND_NEXT_DESKTOP_UPDATE_URL`.
   1.x keeps `desktop` and its current variables, so released clients see no change. 2.0 has not shipped, so renaming
   its client costs nothing. The change needs route tests and manifest and 426 fixtures under
   `src/backend/src/fixtures/contracts/`. Until it ships, 2.0 must ignore the manifest's latest version and update URL,
   because they belong to 1.x. Setting the 1.x minimum above 2.0 is not an option: it would block the 2.0 line.
5. **`/api/me` is cookie-only.** `src/backend/src/pages/api/me.ts` reads the session cookie. 2.0 sends its bearer token
   there (`internal/backend/request.go`), so the account plan and credits fail with 401 in production. 1.x does not call
   `/api/me`. Fix one way: the backend accepts bearer tokens on `/api/me` (additive, the same lookup the token routes
   use), or 2.0 reads credits from `active-model`. This blocks the M2 account view.
6. **`GET /api/v1/models` has no route.** 2.0 calls it (`internal/backend/models.go`). The only route under that path
   is a provider catch-all for generate and stream operations. 1.x uses `GET /api/v1/active-model`. Fix in 2.0: use
   `active-model`, as 1.x does. No backend change.
7. **Routes both clients depend on.** `POST /api/auth/device/initiate`, `POST /api/auth/device/exchange`,
   `GET /api/compatibility`, `DELETE /api/tokens/current`, `GET /api/v1/active-model`, `POST /api/v1/decisions`,
   `POST` and `DELETE /api/v1/audio/transcription-preparations`, and `POST /api/v1/audio/transcriptions` (1.x dictation,
   confirmed in `src/app/electron/dictation/dictation-transcription.ts`). 2.0 also calls `POST /api/v1/chat/completions`
   and `GET /api/me`. Every backend change to these routes follows the checklist in `docs/backend-api-compatibility.md`.
8. **Idempotency and dictation headers.** Optional dictation headers stay optional. Neither app's billing identity
   changes.

Backend work list for the overlap (all additive, none needs a 1.x release): the client-name split (4), the scheme
option (section 2), and bearer support on `/api/me` (5). Each needs fixtures under `src/backend/src/fixtures/contracts/`
and a deploy that passes the pricing gate in `docs/backend-api-compatibility.md`.

## 5. Release channels and order

Today:

- **1.x** has hourly dev builds (`v<version>-dev`), nightly promotion (`-nightly`), and stable promotion (`v<version>`,
  GitHub's "latest"). Promotion workflows: `promote-nightly.yml`, `promote-stable.yml`, `nightly-release.yml`.
- **2.0** has `native.yml` (checks), `native-rig.yml` (L4 journeys) and `native-package.yml`, which is manual and
  publishes nothing. Release steps are in `release.md`. The app's updater has two channels, stable and nightly, that
  read `native-v` releases. **There is no 2.0 promotion workflow, no dev pointer, and no dry-run of promote or
  rollback.** These are missing (WP5.5 acceptance).

Order (each step must pass the checklist in `release-checklist.md`):

1. **Nightly (internal).** `native-v2.0.0-next.N`, a prerelease that is not "latest". The owner dogfoods it as the only
   launcher for five working days (PLAN section 3.3).
2. **Beta (volunteers).** Two weeks of `native-v2.0.0-beta.N` prereleases. The nightly channel offers prereleases, so
   testers stay on that channel. No new channel is needed for beta, but the owner must confirm that testers opt into
   nightly. Crash and budget telemetry stays within the privacy rules.
3. **Stable.** `native-v2.0.0`, published with `--latest=false`. Electron stays GitHub's "latest" until cutover, so
   1.x discovery is unaffected.
4. **Cutover (a separate decision).** Point the website, the backend's default `NEVERMIND_DESKTOP_UPDATE_URL` (only
   if the 1.x line is retired) and the download pages at 2.0. This is a backend or web change, not a client release.

Version rules: 2.0 uses SemVer prereleases in `mygo.json`; 1.x keeps date-stamped versions. The two lines are
compared only by the backend, and section 4 keeps that comparison from crossing lines.

## 6. Dogfood gate and parity review (PLAN section 3)

- **Dogfood gate.** The owner uses 2.0 as the only launcher for five working days before each milestone exit. Every
  crash, wrong result and "I had to go back to 1.x" moment gets a one-line cause. Exit needs zero data-loss bugs and
  zero open P1s. **Status: not started.** The repository holds no dogfood log.
- **Scripted sessions.** Each milestone exit runs the relevant journeys J01-J30 on each OS from a checklist in the PR.
  Every failure becomes an issue with its journey id. **Status: macOS and Windows have not run any journey.**
- **Parity review.** Side by side with 1.x, per feature: the same results order for 20 canned queries
  (`src/native/testdata/queries.txt`, **missing**), the same actions and shortcuts, and every difference listed and
  either accepted or fixed. The first-pass sheet is `src/native/testdata/parity/parity-review.md`. It lists every
  feature and its Go status.
- **Copy review.** Every user-facing string follows the product voice and says how to fix an error. Reviewed in the PR.

Per-feature parity checklist (repeat for each row of the sheet):

1. Same top results for the canned queries, with documented differences only.
2. Same actions, same shortcuts, same empty and error states (goldens for both).
3. The journey ids the feature touches pass on macOS, Windows and Linux.
4. No data loss on kill, relaunch and corrupt-store tests (J30).
5. Differences recorded in the sheet and accepted by the owner.

## 7. Rollback

Four levels, from the smallest to the largest:

- **User.** Keep 1.x installed. Uninstall 2.0 at any time. 1.x data is untouched. 2.0 data stays in its own folder and
  can be reopened by the same 2.0 build or a later one.
- **2.0 build.** The updater never installs an older version, and MyGo has no staged rollouts. The remedy is a fixed
  higher version (a prerelease, then a patch) and, for an installed bad build, a reinstall of the previous package by
  hand (`release.md`). Changing a release to draft stops the feed from offering it, but it does not roll back an app
  that already installed it. Say so in the release notes.
- **Program (before stable).** Nothing public has changed. Stop promotion; keep the nightly and beta feeds.
- **Program (after stable).** 1.x stays installable and keeps its updater. The owner can point the download page and,
  if needed, the backend's `NEVERMIND_DESKTOP_UPDATE_URL` (or the 2.0 `updateUrl` variable from section 4) back to
  1.x. No client release is needed.
- **Backend.** The additive changes in section 4 stay in place. They do not affect 1.x. `NEVERMIND_KILL_SWITCHES`
  (`ai_proxy`, `ai_streaming`, `audio_transcription`, `auth_device`) applies to every client, so it is an emergency
  tool for the whole backend, not a 2.0 rollback.

## 8. Support and communication

Support playbook (to be written as a short page before any outside user receives 2.0):

1. Ask for the version (Settings, Updates), the platform and architecture, and the error reference from the
   Diagnostics page (copy the reference, not the log). Never ask for clipboard, screen or file contents.
2. Sign-in trouble: the paste-code path always works. "consumed" means another app took the code; sign in again.
3. "Where is my data?" Use the section 3 copy. 2.0 never reads 1.x data.
4. "My extension stopped working." 2.0 does not run 1.x extensions. Point to Automate.
5. A client the backend marks unsupported (`unsupported_desktop_version`) gets an update prompt. 2.0 derives it in
   `internal/backend/compat.go`. The user updates that app; no support ticket is needed.
6. Shortcut banner: change the palette hotkey in 2.0 or in 1.x (section 1).

Communication steps:

1. Release notes for each nightly, beta and stable build, from `release.md` (the draft notes in section 3 are the
   starting copy). Copy review applies.
2. A 2.0 announcement on the same channels as the 1.x releases, and a note in the README. The owner writes it.
3. One final notice in 1.x, shipped in a 1.x release, if 1.x is to point users to 2.0. This needs a 1.x release and is
   the owner's decision.
4. A `native-2.0` issue label for support triage.

## 9. Optional importer (not built)

WP5.5 lists an optional, read-only importer for extension source and notes, with a dry-run report. It is not
built. If the owner wants it, it runs only after the dogfood gate, reads 1.x files without writing to them (the
acceptance test is that no write reaches a 1.x path), and reports what it would import before importing anything.

## 10. Gates before cutover

All of these must be true before stable is announced as the replacement:

1. Milestone exits M1-M5 met, or the owner accepts the gap in writing (PLAN section 7; readiness summary at the top of
   PLAN.md). **Today: M0 met; M1-M5 unmet.**
2. Backend changes from section 4 deployed, with fixtures, and the production minimum and latest values checked.
3. Signed and notarized macOS packages (arm64, and amd64 if the owner keeps it), a signed Windows package, and clean-machine
   installs (L7) for each. **Today: none run; Windows is unsigned.**
4. The updater proven on a published feed, from the previous version on each OS (L7). **Today: only a loopback run (J24).**
5. Five-day dogfood log and a two-week beta with zero data-loss bugs and zero open P1s.
6. Parity review complete, with every difference listed and accepted.
7. Budgets B1-B8 measured on each OS and ratified.
8. Support playbook and announcement copy approved.

## 11. Owner-only decisions

Each decision blocks a step above. Recommendations are in brackets.

1. **D1 Identity.** Keep "Nevermind Next" and `com.pablopunk.nvm.next` for the overlap? Rename to "Nevermind" at cutover?
   [Keep the bundle identifier, which cannot change; rename only at cutover.]
2. **D2 URL scheme.** Use `nvm-next` through the `callbackScheme` option (B2), with paste-code as the interim? [Yes.]
3. **D3 Backend identity.** Approve the client-name split (`nevermind-next`) and its three environment variables, and
   read the production values of `NEVERMIND_MIN_DESKTOP_VERSION`, `NEVERMIND_LATEST_DESKTOP_VERSION` and
   `NEVERMIND_DESKTOP_UPDATE_URL`. [Approve; read the values before the next backend deploy.]
4. **D4 Data directory.** Keep `~/.config/nvm-native` on every OS, or move to MyGo's per-OS user-data path
   (`mygo-practices.md` C10)? Permanent once users have data. [Decide before the first public build.]
5. **D5 Keychain service.** Keep `nvm-native`? [Keep; renaming it strands tokens.]
6. **D6 Importer.** Build the read-only importer for notes and extension source, or ship without it?
   [Ship without it; revisit after beta feedback.]
7. **D7 Cutover date and mechanism.** Who decides cutover, which download page changes, and whether 1.x receives a
   final release that points to 2.0? [Owner.]
8. **D8 Channel mapping.** Use the updater's nightly channel for beta, or add a beta channel (WP5.2 change)?
   [Nightly for beta.]
9. **D9 Platforms.** Keep macOS amd64 (Intel). 1.x releases are Apple silicon only (`docs/release-promotion.md`).
   [Drop it unless Intel users are a requirement.]
10. **D10 Windows signing.** Buy or create the code-signing certificate and add `WINDOWS_CERTIFICATE` and
    `WINDOWS_CERTIFICATE_PASSWORD`. [Required before stable.]
11. **D11 Update keys.** Generate the update key (`release.md`), store the private key, and add the variable and
    secret. Losing the key strands installed apps.
12. **D12 Palette hotkey default.** Change 2.0's default during the overlap if it matches 1.x (section 1). [Decide
    after the 1.x default is checked.]
13. **D13 Launch at login during the overlap.** Default off for 2.0? [Yes.]
14. **D14 Telemetry.** The 2.0 analytics endpoint, its consent copy and its event list (`docs/posthog-observability.md`).
15. **D15 Support owner and channel.** Who answers 2.0 support, and where.
16. **D16 Accepted contrast waivers (11) and lease takeover semantics** (WP0.4, WP0.6). Both still await review.
17. **D17 Dogfood and beta people.** Confirm the owner's five-day dogfood start date and the beta volunteers.
18. **D18 Go-only sign-in path.** Accept paste-code sign-in for 2.0 until the scheme option ships? [Yes, as the interim.]

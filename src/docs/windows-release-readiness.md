# Windows release readiness

Windows support is **UNVERIFIED**. The Windows CI job produces unsigned smoke artifacts and startup evidence; it does not authorize a public support claim, production publishing, close intent, or a signed release. Every required Windows 11 manual row must pass against the same candidate artifact before that boundary changes.

The machine-enforced companion inventory is [`windows-platform-inventory.json`](./windows-platform-inventory.json). Each source rule owns exactly one structural source selector, an explicit disposition, and one readiness-matrix surface. `scripts/check-windows-platform-inventory.cjs` fails new, duplicate, or stale platform reads, platform selectors, injected platform branches, OS-owned labels, and known-folder construction—even when another site in the same file is already inventoried.

## Capability and readiness matrix

| Surface | Current implementation state | Automated evidence | Required Windows 11 evidence | Owner |
|---|---|---|---|---|
| Palette hotkey | Implemented; formatting and reserved-shortcut decisions are platform-aware | Off-host shortcut and OS adapter tests | Register `Alt+Space`, exercise a conflict, focus, dismiss, and restore the prior binding | Desktop release |
| Window behavior | Unverified | Built portable writes a renderer `ready-to-show` marker and remains alive for five seconds | Multi-monitor placement, focus, taskbar visibility, dismissal, first show and relaunch | Desktop release |
| App discovery, launch, running status, quit | Implemented | Injected Windows Start Menu roots, recursive `.lnk` scan, `shell.openPath`, inaccessible roots, watcher cleanup, and exact `taskkill` argv | Discover nested shortcuts, launch names with spaces/Unicode, observe running state, force quit | Desktop release |
| File search and open | Implemented | Existing file suites plus platform inventory | Search/open local, spaced, Unicode and UNC paths | Desktop release |
| Clipboard text/image/file/paste | Implemented | Clipboard unit suites | Read/write text, HTML, image and files; paste into another application; confirm restore behavior | Desktop release |
| OCR and screen capture | Intentionally unsupported | Capability fallback omits or rejects unavailable actions | Confirm no misleading Windows OCR/screen-capture action is exposed | Desktop product |
| Camera | Implemented, unverified on Windows | Injected Windows/Darwin permission behavior; platform-neutral denial copy | Allow and deny camera access, switch devices, verify live preview | Desktop release |
| System actions and settings | Implemented | Capability inventory and off-host Windows settings contracts | Lock, sleep, restart, Windows Settings, and typing settings | Desktop release |
| Auth, device token, BYO key | Implemented | Shared private-file writer tests; CI creates representative files beneath `%APPDATA%` and rejects broad ACL grants | Sign in with the safe account, complete device auth, inspect redacted ACL identities for actual profile files | Security release |
| Generated extensions | Implemented | Persistence suites; ASAR verifier requires the extension API declarations and TypeScript runtime declarations | Generate, execute, restart, and execute again | Desktop release |
| Background jobs and watchers | Implemented | Recursive watcher simulation, cleanup proof, and existing job suites | Startup, interval, file-change, clipboard-change and relaunch persistence | Desktop release |
| Notifications | Missing; no support claim | Inventory records the absence | Not a release requirement until product approval adds this capability | Desktop product |
| Auto-updater | Enabled for installed NSIS builds; portable builds are excluded | Release publishing includes validated NSIS-only `latest.yml`; CI proves its URL, path, size and SHA-512 match the setup artifact | Unsigned installed candidate downloads and installs an update without channel collision | Release engineering |
| NSIS installer/uninstaller | Unverified | Exact x64 setup name, differential blockmap, unsigned Authenticode status, hash manifest | Per-user install path, Start menu shortcut, uninstall cleanup, preserved user data | Release engineering |
| Portable behavior | Unverified beyond startup | Staged wrapper identity, SHA-512, extracted child identity, packaged state/version, renderer readiness, PID stability and cleanup | Launch from writable/read-only locations and document where user data persists | Release engineering |
| Logging and crash recovery | Unverified | Absolute startup logs/evidence are uploaded even on failure | Redacted logs, forced crash, recovery and relaunch | Desktop release |
| Start at login | Implemented | Injected packaged/unpackaged login-item behavior | Enable, sign out/in, observe launch, disable | Desktop release |
| Icon and x64 identity | Implemented packaging contract | Builder source SHA-256/dimensions plus positive PE `RT_GROUP_ICON` and `RT_ICON` counts; this does not claim pixel equivalence | Compare taskbar, Start menu, installer and portable visuals with the source icon | Release engineering |

## Packaging contract

The smoke build runs `mise exec -- pnpm run dist:win:x64` with `CSC_IDENTITY_AUTO_DISCOVERY=false`, no certificate variables, and `--publish never`. It must produce exactly:

- `Nevermind-<version>-win-x64-setup.exe`
- `Nevermind-<version>-win-x64-setup.exe.blockmap`
- `Nevermind-<version>-win-x64-portable.exe`
- `latest.yml` with exactly one NSIS target
- `win-unpacked/Nevermind.exe`
- `win-unpacked/resources/app.asar`

The setup, portable launcher, and unpacked executable must all report Authenticode `NotSigned` with no signer certificate. The verifier extracts only the deterministic `win-unpacked` ASAR using the pinned `@electron/asar` binary installed from the frozen lockfile. It requires the main, preload, renderer, extension API, TypeScript library declarations, packaged-resource contract, and allowed runtime imports. CI first removes `app.asar` from an isolated copy and requires failure, then verifies the untouched build.

`windows-smoke-manifest.json` binds the version, x64 architecture, commit, unsigned state, target-specific filenames, sizes, SHA-512 hashes, blockmap ownership, ASAR hash, and narrowed icon evidence. The artifact upload also retains absolute portable startup logs and the readiness marker.

## Installer, updater, and data assumptions

- NSIS remains per-user (`perMachine: false`). The expected install location is the current user's application area; the exact path must be recorded by the manual run.
- Start menu creation, shortcut ownership/conflicts, uninstall cleanup, and preservation of Electron `userData` are manual gates. Packaging configuration alone is not evidence for them.
- Portable launcher extraction is proven only for CI startup. Portable user-data placement and behavior from constrained locations remain manual gates.
- Windows private-file `mode: 0o600` is creation-time POSIX/best-effort metadata, not an ACL guarantee. Production privacy relies on inheritance from Electron's per-user `userData`; CI resolves ACL identities to SIDs, denies broad-user grants, allows only the current user/SYSTEM/Administrators, and requires a current-user allow entry. The elevated CI runner may use Administrators as the default owner, so ownership alone is not treated as privacy evidence. Manual QA must inspect the actual profile files. Do not add ad-hoc `icacls` mutations without security review.
- Electron-builder emits `latest.yml` even with `--publish never`; release CI publishes that metadata with the unsigned NSIS setup executable. The unsigned smoke requires exactly one NSIS file entry whose URL, path, size and SHA-512 match the x64 setup executable and whose `.exe.blockmap` is non-empty. Portable is not an updater target. CI retains the validated metadata and binds its SHA-256 in the smoke manifest.
- A production candidate requires an installed old-to-new update test and must not collide with the macOS/Linux release channels. SmartScreen and UAC may warn because the release is unsigned.

## Windows CI evidence versus release evidence

The `windows-first-run` job remains a separate development regression: it proves a skipped Electron payload can be repaired by the normal development command. The `windows-package-smoke` job proves aggregate verification, unsigned NSIS/portable creation, fail-closed package inspection, inherited ACL privacy for representative files, and startup of the staged portable artifact.

CI startup does **not** prove installer UX, global shortcut behavior, palette focus, SmartScreen, icon pixel equivalence, updater installation, device auth, AI streaming, uninstall cleanup, or portable data semantics. A green package job therefore leaves support readiness unverified.

## Manual Windows 11 gate

Prerequisites:

- Windows 11 x64 desktop session with the exact build number recorded.
- Candidate downloaded from the named GitHub Actions run; verify its SHA-512 against `windows-smoke-manifest.json` before use.
- Dedicated non-production test account with no personal data and permission to complete device auth.
- Test network that can reach the selected non-production Nevermind environment and AI provider; never paste credentials or tokens into evidence.
- Screen recording/screenshot tooling and a redaction pass for usernames, home paths, tokens, prompts, clipboard contents, device codes and network identifiers.

Record one evidence header and keep every check tied to it:

```text
Windows edition/build:
Machine/VM architecture:
Artifact filename and SHA-512:
Git commit and GitHub Actions run URL:
Test date/time zone:
Owner:
Test backend/account class (no identity or secret):
Redacted log and screenshot attachment locations:
```

Required procedure:

1. Verify hashes, install per-user, record the real install path and Start menu entry, then launch.
2. Exercise first launch/relaunch, shortcut registration/conflict, palette focus/dismissal, multi-monitor behavior, and visible icon surfaces.
3. Exercise app/file search and open, including spaces, Unicode and an available UNC path; launch and force quit an application.
4. Exercise clipboard text/image/file flows and paste into another application.
5. Open Windows and typing settings; exercise only safe system actions on the disposable test machine.
6. Complete sign-in/device auth and one AI stream with the safe account/network. Inspect redacted ACL output for the actual auth and BYO files.
7. Generate and run an extension; exercise a background job; quit/relaunch and prove persistence.
8. Capture redacted normal logs, force a recoverable crash, and prove relaunch.
9. Exercise portable mode from representative locations and record user-data behavior.
10. Uninstall the NSIS build; record removed application/shortcut files and intentionally preserved or removed user data.
11. For a future signed candidate only, record Authenticode/timestamp/SmartScreen evidence and install an actual Windows update from validated NSIS metadata.

Every row is `PASS`, `FAIL`, or `UNAVAILABLE`, with evidence. Any `FAIL` or `UNAVAILABLE` required row keeps the matrix `UNVERIFIED`, blocks a Windows support claim and close intent, and requires a linked follow-up issue rather than a weakened gate.

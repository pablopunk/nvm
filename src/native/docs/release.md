# Cutting a Go app release

Status: no release has been cut. The workflow only builds. Publishing is manual,
and the first release waits for the owner actions below.

## Steps

1. Merge the change to `main` with `native.yml` and `native-package.yml` green.
2. Choose the version. Either set `version` in `src/native/mygo.json` in that change,
   or give the run a `version` input (for example `2.0.0-next.1`). The run stamps it
   into the packages and does not commit it.
3. Run **Native packages** (workflow_dispatch) with `release` set to true. Signing
   and notarization run only when their secrets exist. Notarization needs this
   dispatch run, so a pull request never submits to the notary service.
4. Read each job's summary. It says signed, notarized or UNSIGNED. Read the
   `--build-flags` output in the Linux and macOS logs: it must read
   `packaged=true release=true`.
5. Download the artifacts (`native-macos-*`, `native-linux-*`, `native-windows-amd64`)
   and check the `SHA256SUMS` files.
6. Test on a clean machine: install, launch, sign in against staging, use the
   hotkey. This is L7 in `PLAN.md` and is not automated yet.
7. Create a draft GitHub release for the tag, attach the packages and `SHA256SUMS`,
   then publish after review.

Rollback: keep the previous packages. Stores are versioned and forward-compatible
by one version (PLAN section 4), so a previous build opens current data.

## Owner actions before the first release

- Confirm the identity: the name **Nevermind Next**, the slug `nevermind-next`, and
  the bundle identifier `com.pablopunk.nvm.next`. The identifier is permanent once
  shipped. The Electron app uses `com.pablopunk.nvm`.
- Resolve the `nvm://` scheme. The Electron app claims `nvm` too. When both apps are
  installed, the system picks one, so the sign-in redirect may reach the wrong app.
  Choose a separate scheme for the next channel (the backend approval page must
  change with it) or accept the collision until cutover.
- Windows signing: create or buy a code-signing certificate and add
  `WINDOWS_CERTIFICATE` and `WINDOWS_CERTIFICATE_PASSWORD`. No Windows secret exists
  today. Until then the Windows packages are unsigned.
- Check runner access. `macos-15-intel` and `ubuntu-24.04-arm` may need a larger
  runner plan for this repository. If they are unavailable, build those targets on
  the other runners by cross compiling libopus or drop them.
- Approve the icon (taken from the Electron app's `build/Icon.icon`) and the libopus
  pin (1.5.2, BSD-3-Clause, notice in `THIRD_PARTY_NOTICES.md`).
- Decide when the updater (WP5.2) starts. Until `updates` is set in `mygo.json`,
  packages do not self-update.

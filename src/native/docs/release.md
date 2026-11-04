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
by one version (PLAN section 4), so a previous build opens current data. The
updater never installs an older version; a rollback is a reinstall of the previous
package by hand.

## Updates

A release run with `release` set to true and the two update settings below makes
the update feed: `update-<target>.json` manifests, the signed archives and, from the
second release, deltas. The run uploads them with the installers. Publishing is
manual: the feed is what installed apps read.

- Apps read the newest release whose tag starts with `native-v` (not a draft). They
  never read the Electron releases, which use `v` tags in the same repository.
- Stable: a final release such as `native-v2.0.0`. Nightly: a prerelease such as
  `native-v2.0.1-next.3`, which the Nightly channel also offers.
- Publish a final release with
  `gh release edit native-v2.0.0 --draft=false --latest=false`, so that it does not
  become the repository's latest release. Publish a prerelease with
  `gh release edit native-v2.0.1-next.3 --draft=false --prerelease`.
- An app downloads an update in the background after it finds one, verifies its
  signature and size, and waits for a restart (Settings, the menu bar or the root
  palette). Quitting without restarting also installs it: MyGo replaces the app on
  quit on macOS and at once on other systems, and the new version runs next launch.
- A build without the settings, or a release run without them, has no feed and says
  so in the Updates page.
- GitHub limits anonymous API calls to 60 an hour for each address. The build step
  passes `GH_TOKEN`, which MyGo uses to read the previous releases for deltas.

### Owner actions for updates

1. Run `go tool mygo keygen` once on a trusted machine. It writes `mygo-update.key`
   (secret) and `mygo-update.pub` (public). Store the private key in a password
   manager. Losing it strands installed apps; anyone who has it can ship code to them.
2. Add the public key as the repository variable `NATIVE_UPDATE_PUBLIC_KEY` (its
   contents, one line).
3. Add the private key as the repository secret `NATIVE_UPDATE_PRIVATE_KEY` (its
   contents). The workflow passes it to `mygo build` as `MYGO_UPDATER_PRIVATE_KEY`.
4. Dispatch **Native packages** with `release` set to true and `version` set. Read
   each job's summary: it must say the feed is on and signed, and the update key check
   must pass.
5. Publish the draft release (above) only after the L7 checks below pass on the
   candidate: install the N-1 package, launch, see the update found, downloaded and
   ready, restart, and confirm the new version runs.

Not done and not in the feed: staged rollouts (MyGo has none), an automatic rollback
(a reinstall by hand), the Windows portable updater (the installer's per-user install
is what MyGo updates), and signing Windows updates with Authenticode.

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
- Decide when the updater starts publishing. The feed needs the update owner actions
  above; until a release is published, installed apps report that no update is found.

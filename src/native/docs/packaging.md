# Packaging the Go app

`src/native` packages with MyGo (`go tool mygo build`, pinned in `go.mod`). The
workflow `.github/workflows/native-package.yml` builds every platform and uploads
the packages as run artifacts. It publishes nothing; see [release.md](release.md).

## Where each setting lives

- `src/native/mygo.json` (committed): name, slug, identifier, icon, the `nvm://`
  scheme, the macOS `Info.plist` keys (`LSUIElement`, usage strings), the
  entitlements file and the Linux dependencies.
- Per-run stamping (`scripts/packaging/stamp-mygo-config.sh`, on the CI checkout
  only): `version`, the notary profile for macOS, and the Windows certificate path.
  Secrets are never written into the repository.
- Release flags, through `MYGO_LDFLAGS` (`scripts/packaging/release-ldflags.sh`):
  - `internal/app.isPackagedBuild=true` on every packaged build. It makes the
    account refuse development backends.
  - `internal/app.releaseBuild=true` only for a run with `release=true`. It is the
    only build with an analytics endpoint.
  - A plain `go build` or `go run` leaves both at `false`.
- `nevermind-next --build-flags` prints `packaged=… release=…` and exits without
  starting the app. `scripts/packaging/verify-package.sh` compares that output with
  what the run expects, for each runnable artifact.

## Bundled natives and helpers

| Item | Built by | Pin | Ships as |
|---|---|---|---|
| libopus 1.5.2 (dictation) | `scripts/natives/build-libopus.sh` | SHA-256 of the upstream tarball, and four exported symbols checked after the build | `lib/libopus.dylib` (macOS) or `lib/libopus.so.0` (Linux) in the app resources |
| `macos-selected-text` (window and selected-text helper) | `scripts/natives/build-macos-helper.sh`, from the Electron app's Swift sources | the source files of this commit | `Contents/Resources/macos-selected-text` |

MyGo signs both with the app's identity (its resources rule): the hardened runtime
and a timestamp for Developer ID, ad hoc otherwise. The Go loader tries the bundled
libopus before the system one. Tesseract is not bundled: Linux users install it,
and the app reports text recognition only when it finds it. The macOS job fails when
the window helper is missing from the bundle or has the wrong architecture, because the
window commands look for it in `Contents/Resources` and fail without it.

Entitlements (`packaging/entitlements.mac.plist`): audio input, camera and Apple
events, which the paste and copy keystrokes need. No JIT entitlement: the app does
not run JIT code.

## Platforms

| Target | Runner | Output |
|---|---|---|
| darwin/arm64 | `macos-latest` | `Nevermind Next.app` and `nevermind-next-<version>-arm64.dmg` |
| darwin/amd64 | `macos-15-intel` | the same, for Intel |
| linux/amd64 | `ubuntu-latest` | archive with `install.sh`, `.deb`, `PKGBUILD` |
| linux/arm64 | `ubuntu-24.04-arm` | the same, for ARM64 |
| windows/amd64 | `ubuntu-latest` (NSIS and osslsigncode installed) | `Nevermind Next.exe` and `nevermind-next-setup-<version>-amd64.exe` |

Linux and Windows packages build on any Linux machine: run
`bash scripts/natives/build-libopus.sh`, then `go tool mygo build -platform linux/amd64`
(or `windows/amd64`, with `nsis` installed) from `src/native`. The macOS packages
need a Mac: a build on Linux stops before the DMG and the signature.

## Signing and notarization

Existing secrets, the same names as the Electron release workflows:
`MACOS_CERT_P12_BASE64`, `MACOS_CERT_PASSWORD`, `APPLE_ID`, `APPLE_TEAM_ID`,
`APPLE_APP_SPECIFIC_PASSWORD`.

- macOS: a run signs when the certificate secrets exist. It also notarizes when the
  Apple ID secrets exist, on pull requests and dispatch runs alike. Notarization
  waits for Apple's verdict (`notarytool --wait`), then MyGo staples the DMG and the
  app, and the job checks the DMG with `stapler validate` and `spctl`. Same-repository
  pull requests get the secrets; forks do not, so their builds are unsigned.
  The job timeout (90 minutes) covers the build and that wait.
- Windows: `WINDOWS_CERTIFICATE` (base64 `.pfx`) and `WINDOWS_CERTIFICATE_PASSWORD`.
  These secrets do not exist yet, so Windows packages are unsigned.
- Linux: no signing.

Without secrets each job writes an UNSIGNED note to its step summary. macOS packages
are ad hoc signed: Gatekeeper blocks them on other Macs. Windows packages get
SmartScreen warnings.

## Updates

Release runs (`release` set to true) with the repository variable
`NATIVE_UPDATE_PUBLIC_KEY` stamp an `updates` block into `mygo.json` for the build:
the public key and the repository, with the `native-v` release prefix. `mygo build`
signs the update files with `NATIVE_UPDATE_PRIVATE_KEY`, and each release job uploads
the manifests and archives beside the installers. `verify-updates.sh` checks that the
public key is linked into the executable. Publishing and owner steps are in
[release.md](release.md#updates).

## Checks in the workflow

`actionlint` covers the workflow (run locally with `actionlint`). Each runnable
artifact runs `--build-flags` and must match the run. The Linux `.deb` and archive
contents are printed in the log. Windows flags are not checked: the GUI executable
does not run on the Linux runner.

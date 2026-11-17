# Releasing the Go app

The Go app ships through the same release flow as 1.x: the same workflows, triggers, gates, version scheme, tags,
release titles, notes and retention. Only the build changed: the dev build packages the Go app with
`native-package.yml` instead of electron-builder.

## The flow

1. **Dev build** (`dev-build.yml`, hourly at :23 and on dispatch). When `main` has build-relevant changes since the
   published dev build, it allocates the next `YYYY.MMDD.N` version (UTC date, `N` the day's build), builds every
   target with `release=true` (signed, notarized, with the update key), validates the asset set, the manifests and
   their signatures, and publishes the prerelease `v<version>-dev`, titled `Nevermind <version> (dev)`. It then
   moves the dev pointer on the `release-pointers` branch.
2. **Nightly** (`promote-nightly.yml`, 02:00 and 03:00 UTC behind the Madrid 04:00 schedule gate, or dispatch).
   It promotes the newest verified dev build newer than the current nightly, as the prerelease
   `v<version>-nightly`, titled `Nevermind <version> (nightly)`, and moves the nightly pointer.
3. **Stable** (`promote-stable.yml`, dispatched by hand with a retained `v<version>-dev` tag). It writes the release
   notes with OpenCode and publishes `v<version>`, titled `v<version> - <name>`, as GitHub's latest release.
4. **Retention** (`dev-retention.yml`, daily). Deletes expired dev and nightly releases that no pointer keeps.

Promotion never rebuilds or re-signs: it copies the dev build's bytes. The one change is the update manifests
(`update-<target>.json`), whose archive and delta URLs name the release that holds them, so each promotion points them
at its own tag (`retarget-metadata`). The archives are signed, not the manifests, so the signatures still hold. 1.x did
the same when it copied its updater metadata to the stable file names.

Promotion requires a passing push run of both `ci.yml` and `native.yml` on the commit, and a healthy production
backend serving that commit or a verified descendant on `main`.

To check the build path without publishing, dispatch **Dev build** on a branch with `dry_run`: it builds and validates
that branch and stops before any release or pointer.

## How installed apps find updates

- **Stable** reads MyGo's feed: the manifest of GitHub's latest release, which only stable promotion sets.
- **Nightly** reads the nightly pointer and takes the newest of its releases and the latest stable release.
- **Dev** reads the dev pointer.

The app links the repository (`release-ldflags.sh`) to read the pointers. A download is installed only when its
archive or delta is signed with the update key built into the app.

## Update keys

The public key is the repository variable `NATIVE_UPDATE_PUBLIC_KEY`; the private key is the secret
`NATIVE_UPDATE_PRIVATE_KEY`, which `mygo build` reads as `MYGO_UPDATER_PRIVATE_KEY`. Losing the private key strands
installed apps, and anyone who has it can ship code to them, so keep a copy in a password manager.

## Rollback

There is no automatic rollback. Promote an earlier retained dev build, or reinstall a previous package by hand. The
updater never installs an older version.

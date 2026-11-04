# Native release checklist

Use this from the version bump to a published release. The details live in [packaging.md](packaging.md) (what the
packages contain and how they are signed) and [release.md](release.md) (the workflow, feeds and owner actions). Read
[cutover.md](cutover.md) first when the release is a channel step (nightly, beta, stable).

Maintainers run every step below. Agents prepare the change and read the results; they never dispatch a release
workflow or publish a release.

1. **Scope.** Confirm the release's channel: nightly, beta or stable (cutover.md section 5). Confirm the PLAN work
   packages it contains and the journeys each one must pass.
2. **Backend gate.** If the release depends on a backend change, confirm it is deployed and that its contract fixtures
   exist (`docs/backend-api-compatibility.md`, "Backend contract PR/release checklist"). A stable release also needs the
   2.0 client-name split and the production minimum and latest values checked (cutover.md section 4).
3. **Version.** Set `version` in `src/native/mygo.json` in a change to `main`: `2.0.0-next.N` for nightly, `2.0.0-beta.N`
   for beta, `2.0.0` for stable. Or pass the `version` input to the run (release.md step 2). Do not reuse a version
   that has a release tag.
4. **CI gate.** `Native (Go)` (`native.yml`), `Native (Go) rig` (`native-rig.yml`) and `Native packages`
   (`native-package.yml`, pull request) must be green on the commit. Run `actionlint` on any changed workflow. CI is the
   gate for checks and tests; run the rig locally only for a focused proof (`scripts/rig/README.md`).
5. **Release run.** The owner dispatches **Native packages** with `release` set to true and the `version` input
   (release.md steps 3 and 4). Read each job's summary: it must say signed or UNSIGNED, notarized or not, and the
   feed state. The `--build-flags` line must read `packaged=true release=true`.
6. **Artifacts.** Download `native-macos-*`, `native-linux-*` and `native-windows-amd64`. Check each `SHA256SUMS`.
   On the matching OS, run `src/native/scripts/packaging/verify-package.sh <executable> true` for each runnable
   artifact. For a build with the feed, run `src/native/scripts/packaging/verify-updates.sh <executable> <public key>`.
7. **Clean-machine checks (L7, not automated).** On a clean machine for each OS: install, launch, sign in against
   staging, use the palette hotkey. For a release that follows another one: install the previous build, launch it, see
   the update found, downloaded and ready, restart, and confirm the new version runs (release.md, Updates step 5).
   Record each result, and every check that was not run, in the release notes.
8. **Draft.** Create a draft GitHub release for `native-v<version>` with the packages and `SHA256SUMS` (release.md
   step 7). Keep the Electron releases and their tags untouched.
9. **Publish.** After review, publish with the command for the channel (release.md, Updates):
   - nightly or beta: `gh release edit native-v<version> --draft=false --prerelease`
   - stable: `gh release edit native-v<version> --draft=false --latest=false`
10. **After publish.** Open an installed copy of the previous build and check the Updates page finds the new release.
    Confirm `gh release view native-v<version>` lists the expected assets and that GitHub's latest release is still the
    Electron release (cutover.md section 5).
11. **Record.** Put the workflow run URL, the `SHA256SUMS` and the list of deferred checks in the release notes and in the
    PLAN readiness summary (`src/native/PLAN.md`). Say which checks ran and which are outstanding.
12. **If it goes wrong.** Keep the previous packages. The updater never installs an older version, so the remedy is a
    fixed higher version, and a reinstall of the previous package by hand (release.md, Rollback; cutover.md section 7).

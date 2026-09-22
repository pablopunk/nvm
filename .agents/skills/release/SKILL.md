---
name: release
description: Cut a Nevermind desktop release from a patch, minor, or major bump and publish user-facing GitHub release notes. Use when the user says "release patch/minor/major", asks to cut or publish a Nevermind release, or asks for release notes for a Nevermind version.
---

# Release

`scripts/release.sh x.y.z` requires a clean tree, updates `package.json` and `pnpm-lock.yaml`, commits the version as `release: vX.Y.Z`, tags it, and pushes the branch and tag. The tag starts the `CI` workflow, whose platform jobs build and upload Windows, signed and notarized macOS, and Linux packages plus updater metadata. The GitHub release is not complete until all required tag jobs succeed.

Releasing is a live external change. A direct instruction such as `release patch` authorizes that exact release. For an ambiguous request, or before changing the requested version or release type, ask for confirmation.

## Cut

1. Require a clean working tree. Do not stash, discard, or include unrelated changes.
2. Run `git fetch origin main --tags`, then establish the release range:
   - latest version tag: `git tag --sort=-v:refname | head -1`
   - current branch: `git branch --show-current`
   - local/remote state: `git status --short --branch`
3. Release only from an up-to-date `main`. Stop if `HEAD` differs from `origin/main` or the latest required `CI` run for `main` is not successful.
4. Bump the latest tag according to the user's word:
   - patch: `0.17.5` → `0.17.6`
   - minor: `0.17.5` → `0.18.0`
   - major: `0.17.5` → `1.0.0`
5. Review `git log --oneline vPREV..HEAD`. If the range changes a desktop/backend API contract, read the release checklist in `docs/backend-api-compatibility.md` and confirm its fixtures and compatibility work are present before tagging.
6. State the computed version and that this will push `main`, create the tag, and start public release CI. If the user's current instruction does not explicitly authorize that exact release, ask before continuing.
7. Run `./scripts/release.sh X.Y.Z` without bypassing hooks. Record the release URL printed by the script.
8. Find the tag-triggered run, not the branch-triggered run for the same commit. Use `gh run list --workflow CI --event push --limit 10 --json databaseId,headBranch,headSha,status,conclusion,url` and select the entry whose `headBranch` is `vX.Y.Z`.

Do not run local tests, typechecks, builds, or the app during this workflow; repository policy assigns runtime release verification to CI.

## Notes

1. While tag CI runs, inspect everything in `vPREV..vNEW`, excluding the mechanical `release:` commit. Read relevant diffs rather than treating commit subjects as release notes.
2. Check recent notes with `gh release view vPREV`. Follow an established Nevermind style when one exists; otherwise use the format below.
3. Ask Pablo to choose a short release name of two to four words, and offer several options grounded in the main user-facing changes.
4. Draft the body in a temporary file:

   ```markdown
   ## Nevermind X.Y.Z — <Name>

   <One sentence that truthfully summarizes the complete release.>

   - ✨ **Area** — User-facing change and why it matters.
   - …

   **Full changelog**: https://github.com/pablopunk/nvm/compare/vPREV...vNEW
   ```

5. Use one concise bullet per user-facing area. Mention compatibility, reliability, packaging, security, or updater changes when material. Do not invent checksums or claim that a platform package works before its CI job succeeds.
6. After the name and notes are final, wait for the full tag run and then publish them:

   ```sh
   gh run watch <run-id> --exit-status --interval 45 \
     && gh release edit vX.Y.Z --title "vX.Y.Z - <Name>" --notes-file <path>
   ```

7. Confirm the release and all expected assets with `gh release view vX.Y.Z` and `gh release view vX.Y.Z --json assets,url`. A created release with pending or failed platform jobs is not complete.

## Report

Report the tag, commit, release URL, tag-run URL and conclusion, published title, and expected platform assets. Use `complete` only after the full tag run succeeds and the final notes and artifacts are visible; otherwise state the exact outstanding validation or failed job.

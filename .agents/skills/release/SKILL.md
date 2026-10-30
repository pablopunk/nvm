---
name: release
description: Promote a retained Nevermind dev build to stable, inspect dev or nightly releases, or write and verify Nevermind release notes. Use immediately when asked to promote, cut, publish, or verify a Nevermind desktop release.
---

# Release

Stable publication promotes a retained signed dev build through the `Promote dev to stable` GitHub Actions workflow. It does not rebuild or re-sign packages. Dev builds run hourly when `main` has changed; nightly promotes a newer eligible dev build at 04:00 Europe/Madrid. See `docs/release-promotion.md` for contracts, recovery, and verification. A release that changes how host-owned persisted data is read or written must increment the state epoch in `src/app/resources/state-compatibility.json` first; otherwise channel switching can offer an older build that cannot read the new data.

Versions use `YYYY.MMDD.N`: the UTC date and a daily build sequence. Promotions retain the dev build's version; the GitHub tag suffix identifies its channel.

A release is a live external change. A direct instruction to promote a specific dev tag authorizes that promotion; do not publish another build without the user's approval. Do not run local tests, builds, typechecks, or the app: repository policy assigns runtime verification to CI.

1. Inspect retained dev prereleases and the latest stable release with `gh release list` and `gh release view`. Select the exact requested `v<version>-dev` tag; confirm it is published, its version exceeds stable, its source commit and platform assets are present, and its CI is successful. If the user has not identified a build, report the candidate and ask which to promote.
2. Review the commit range from the previous stable tag to the selected source. Check desktop/backend contract changes against `docs/backend-api-compatibility.md`; ensure the production identity gate can verify the selected commit or a healthy main descendant. Prepare concise user-facing notes grounded in the changes, including a full changelog link. Do not invent validation results.
3. Invoke `gh workflow run promote-stable.yml --ref main -f dev_tag=<tag>` only for the authorized build. The workflow validates checksums and production identity, publishes a verified draft, and copies updater metadata and exact binaries to the stable release. Capture its run URL.
4. Verify the workflow conclusion, resulting release and tag commit, platform packages, blockmaps, updater metadata, and release notes with `gh run view` and `gh release view`. If appropriate, update the release title and notes after the workflow completes. Confirm published assets match the selected build; do not infer packaged-app or user-journey success from CI alone.
5. Report the selected dev tag, source commit, stable release and workflow URLs, checks and review status, and deferred packaged-app, external, or user validation. Use `complete` only after all requested outcomes and validations are complete; otherwise use the repository's applicable handoff state.

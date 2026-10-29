# Release promotion

Nevermind builds platform artifacts for a selected `main` commit and records the version, source commit, workflow run, and SHA-256 hashes in a manifest. Dev prereleases use `v<version>-dev`; their binary version is the numeric version in the tag. Commit identity is provenance, not part of version ordering. Dev builds are retained for 30 days; the newest, referenced, and explicitly preserved builds are retained longer. Add a standalone `Nevermind-Preserve: true` line to a dev release's notes to preserve it.

Nightly and stable promotions copy the selected dev packages and blockmaps without rebuilding or re-signing. Their releases hold independent asset copies. Nightly uses `v<version>-nightly`; stable uses `v<version>` and publishes as GitHub's latest stable release. Stable updater metadata is copied to the `latest` channel filenames; the referenced package bytes remain unchanged. Drafts are verified before publication. Promotion requires successful CI and a healthy production backend serving the selected commit or a verified main descendant.

To promote stable, dispatch `promote-stable.yml` with the retained dev tag. Inspect the selected release, prior stable release, source history, CI, and backend compatibility before dispatch. Workflow failure is not publication success. Check the run and release, exact asset set and hashes, tag commit, notes, and updater metadata after publication. A packaged updater journey and external user validation are separate checks.

The dev build, promotion, and retention workflows serialize publication operations. A failed draft can be retried only if its source and promotion provenance match; a conflicting published release or tag must not be overwritten. Investigate orphan tags and partial drafts before intervening. Do not delete a release or tag merely to make a retry pass.

Channel changes currently do not permit downgrades. Do not enable them until a durable compatibility contract covers all host-owned persisted data and target builds advertise compatible state support. CI cannot prove packaged installs, OS permissions, or user state migration without the corresponding journeys.

## State compatibility boundary

The host persists more than `state.json`. A downgrade contract must account for portable configuration, AI chats and image files, learning records, clipboard images, extension windows and drafts, extension storage and secrets, extension source files, account tokens and BYO credentials, and persisted dictation recordings. Cache and diagnostic files have different recovery requirements, but their readers must also tolerate any format left by a newer build. The extension host can run user-provided extension code that mutates its own storage and source independently of application upgrades.

A target's version alone does not prove it can read these stores. Missing, corrupt, or unverifiable compatibility information must block a downgrade; selecting a channel must not cause an older binary to overwrite data from a newer incompatible writer. Any future compatibility marker must be durably advanced before affected writes, survive restarts, cover every host-owned store, and be paired with target capabilities in verified updater metadata. Released builds that predate such a contract cannot make that promise. The current updater therefore leaves `allowDowngrade` disabled.

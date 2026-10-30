# Update channels

Intent: stable is the default and only installs published stable releases.
Nightly also installs the newest nightly prerelease, and a stable release still
wins whenever it is newer. The choice is a persisted setting (Update Channel)
and applies to startup, periodic, and manual checks.

## Rules

- Nevermind never downgrades automatically. When the selected channel's newest
  release is older than the installed build, the Updates view offers it as an
  older build only after the host has verified that the build can read the
  current data; otherwise it explains that a verified state backup is needed.
  An older build is downloaded and installed only on explicit request.
- Switching channels discards any available or downloaded update from the
  previous channel and cancels its in-flight download. Results stamped for
  another channel are ignored.
- Dev builds use `YYYY.MMDD.N` versions, ordered by UTC date and daily build
  sequence. Nightly and stable promotions retain the exact dev binary version;
  their GitHub tags identify the channel. See `docs/release-promotion.md`.
- Stable releases publish the default `latest*.yml` update metadata. Nightly
  releases publish `nightly*.yml` metadata (Windows `nightly.yml`, macOS
  `nightly-mac.yml`, Linux `nightly-linux[-arm64].yml`).
- Windows is portable-only. Its `latest.yml`/`nightly.yml` name the portable
  executable, which updates in place; see `docs/windows-release-readiness.md`.

## Switching to an older build

Run Back Up and Verify State in the Updates view, then check for updates. The
offer requires the target's advertised state epoch to match the epoch recorded
for the user's data. See `docs/release-promotion.md` for the contract.

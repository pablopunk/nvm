# Update channels

Intent: stable is the default and only installs published stable releases.
Nightly also installs the newest nightly prerelease, and a stable release still
wins whenever it is newer. The choice is a persisted setting (Update Channel)
and applies to startup, periodic, and manual checks.

## Rules

- Nevermind never downgrades automatically. When the selected channel's newest
  release is older than the installed build, updates pause and the Updates view
  says the build is ahead of the latest release on that channel.
- Switching channels discards any available or downloaded update from the
  previous channel and cancels its in-flight download. Results stamped for
  another channel are ignored.
- Nightly versions are the next patch after the newest stable release with a
  `-nightly.<date>.<run>` prerelease suffix. Semver precedence orders them:
  a stable release outranks its own nightlies, and nightlies order by date and
  then run number.
- Stable releases publish the default `latest*.yml` update metadata. Nightly
  releases publish `nightly*.yml` metadata (Windows `nightly.yml`, macOS
  `nightly-mac.yml`, Linux `nightly-linux[-arm64].yml`).

## Not yet supported

Immediately reinstalling the stable build after leaving nightly. Stable is
offered again once a stable release newer than the installed build ships. An
immediate downgrade needs a data-compatibility plan for forward-only state
migrations before it can be offered safely.

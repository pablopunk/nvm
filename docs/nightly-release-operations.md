# Nightly release operations

The nightly release workflow runs at 04:00 Europe/Madrid, including daylight
saving time. It publishes a GitHub prerelease only when the app has changed since
the last nightly (or the latest stable release when there is no nightly), CI has
passed for the selected commit, and production is healthy and serving that commit
or a verified newer commit on main. The production deployment workflow must also
have completed its production deployment successfully. A failed or unverified
backend deployment prevents publication.

The workflow summary records why a run built or skipped. When a nightly is
missing, inspect the **Nightly release** run and its plan summary first, then the
corresponding **CI** and **Database migration** runs. Check the public
`https://api.nvm.fyi/api/health` endpoint and its reported full commit identity
before investigating release artifacts. Do not treat a healthy older deployment
as evidence that the selected commit deployed successfully.

The workflow can be dispatched manually with `dry_run` enabled to build and
validate packages without publishing. A non-dry manual run uses the same CI,
production identity, and publication checks as the scheduled run. Pull requests
also build unsigned nightly packages and verify their updater metadata; they do
not prove signing, notarization, production deployment, or an installed-app update.

Published nightlies older than 30 days are removed after a successful nightly
publication. The newest published nightly is retained. Stable releases are not
part of nightly retention.

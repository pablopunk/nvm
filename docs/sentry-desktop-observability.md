# Desktop diagnostics

## Purpose and boundaries

Use Sentry for safe error locations and all instrumented operation timing while technical reporting is enabled, Axiom for backend request evidence, and a local metadata index for account-free support lookup.

Desktop tracing currently samples 100% of instrumented operations, including successful journeys, at the owner's request. Session-bound journey and action identifiers connect operations without account identity or private content. This is not a recording of every click or keystroke, and transport failures, bounds, or server quotas can still prevent delivery.
Do not collect full activity histories, screen content, replay, profiling, native dumps, arbitrary log uploads, account identity, or persistent installation identity.

The repository changes are not production validation: collection approval, CI, packaged-app checks, remote event lookup, and alert configuration remain release gates.

## Collection policy

- Packaged main and renderer JavaScript errors use the main SDK transport; development host errors stay local. The renderer SDK loads after the first frame; a bounded early-error queue requests host references before SDK loading completes.
- SDK integrations own unhandled errors. Catch-and-convert owners and React boundaries own explicit captures.
- Operations have separate asynchronous context even when sampling is off. Failures cannot turn into success when an action returns an error view. Cancellation is not an exception capture.
- The host resolves extension ownership from registered handlers. Renderer failure handles bind to the sender window, not renderer-supplied ownership.
- Support references are random host identifiers, not Sentry event IDs. Boot, journey, action, and extension target IDs are session-bound.
- Only approved categories, stages, outcomes, bounded generated stack locations, and validated identifiers leave the computer. Exception text, arbitrary context, request bodies, credentials, prompts, URLs, clipboard data, function names, and user extension paths do not.
- Renderer scope synchronization and automatic breadcrumbs are disabled. Small semantic breadcrumbs are window-local and metadata-only.
- The final desktop transport sanitizes again and allows only error and transaction envelopes. It has a 20-item SDK memory buffer, a 64 KiB sanitized envelope limit, and a 60-error-per-minute capture limit. There is no persistent offline uploader.
- Known legacy Sentry queue and scope files are checked for presence only. They are not replayed, read as payloads, exported, or deleted. Native crash files remain outside support exports.
- `Send Technical Diagnostics` in Settings controls desktop remote collection. No desktop remote collection starts before stored settings are loaded. Turning it off preserves local support records; reports already in flight can still arrive. Backend request diagnostics remain controlled by the backend configuration.

## Support lookup

Use **Show Recent Errors** in Command-K. Each row can copy an error reference or safe diagnostics. **Report a Problem** creates a separate informational support event without free text or normal error paging.

The index is `nevermind-diagnostics.json` in `app.getPath('logs')`. It retains at most 100 records for seven days and rejects files above one MiB. Writes are asynchronous, coalesced, and atomic. Unexpected exit or disk failure can lose records.

Update attempts use `nevermind-update-diagnostics.json` with ten records and the same age and size limits. `targetRunning: true` means that a later launch is running the requested version; it does not prove updater causation or successful installation by that mechanism.

Reporting state is one of:

- `local_only`: no remote capture was requested.
- `capture_requested`: remote availability is unknown.
- `dropped`: local capture policy rejected remote collection.

An event ID, SDK callback, or `flush()` is not delivery proof. Only a successful authenticated event lookup in the mapped project proves remote availability. Offline exit, SDK filtering, transport limits, and server rejection can lose events.

For “an error just now,” read this index first, select the time window and boot/build, and look up `support_ref` in the validated desktop project. Use generated stack lines and columns with that build's source maps. If `targetId` is present, local `diagnostic.target` logs map it to the extension source; those logs may contain private data and must not be uploaded or copied as safe diagnostics.

## Journey evidence

| Boundary | Evidence | What it does not prove |
| --- | --- | --- |
| Commands, root items, view actions, shortcuts | Host execution outcome and registered component target | Successful rendering or an external side effect |
| View loaders and renderer boundaries | Load stage or JavaScript render failure | Full UI navigation, focus, or visual correctness |
| AI streams and builder stages | Stream error/cancellation, generation validation, installation failures | HTTP success alone does not prove stream completion |
| Sign-in | Persisted usable credentials or timeout/cancellation | Browser launch alone does not prove sign-in |
| OS dispatch | Host acceptance/failure of the existing capability | OS acceptance does not prove external completion |
| Host and extension jobs | Once-only run/timeout outcome | Late completion cannot overwrite a terminal result |
| Updates | Check/download failures and pending target-version observation | Quit/install request does not prove next-launch success |

Unexpected transport, unexplained stream, and valid-response client-processing failures are client-owned. Classified backend failures create local desktop outcomes with request IDs, not duplicate desktop exception events.

Desktop metadata requests propagate action/journey IDs and SDK `sentry-trace` only to canonical HTTPS Nevermind API origins. Redirects on these traced requests fail closed. BYO and custom origins receive no diagnostic headers. Provider streams use the provider's response callback to read `x-request-id`; they do not receive injected tracing headers because their SDK redirect policy is not established. Full provider-stream trace-parent propagation remains outstanding coverage.

Backend correlation headers are optional, bounded metadata, never authentication. Responses retain bodies, status, stream content, and existing server-generated billing/idempotency request IDs. Axiom `diagnostic_response` links final request IDs to available desktop IDs; `diagnostic_stream_terminal` records completion, failure, or cancellation. Backend error payloads are sanitized before Sentry delivery.

## Release source maps

Main, preload, renderer, and renderer chunks produce hidden maps. Electron packaging excludes `.map` files.
The release identity is `nevermind@<version>+<commit>.<build-platform>`. The same build output is uploaded before packaging, without rebuilding. Release jobs require `SENTRY_AUTH_TOKEN`, `SENTRY_ORG`, and `SENTRY_PROJECT_DESKTOP`; upload credentials are never build definitions or packaged resources.

The desktop DSN comes from runtime `SENTRY_DSN_DESKTOP`, legacy `NEVERMIND_SENTRY_DSN`, or the existing fallback project ID `4511502032502784`. A build-machine environment variable does not automatically become packaged runtime configuration. Validate that the upload project matches this DSN before a release.

## Operational setup requiring approval

Use `.agents/skills/production-debugger/sentry.md` and `axiom.md` to check project scope, read access, upload access, and alert-management access separately; one successful probe does not prove all permissions.

After approval:

1. Validate desktop/backend project mapping, upload credentials, retention policy, and privacy notice.
2. Confirm a controlled packaged main, preload, renderer, and lazy-chunk failure resolves to original source in Sentry for its exact release.
3. Confirm offline exit and remote reporting disabled leave truthful local records.
4. Confirm real auth and AI journeys can pivot from support reference to the correct backend request and terminal stream evidence.
5. Create dashboard panels for failures by operation/stage/component/release, operation latency percentiles, and Axiom stream outcomes by request ID.
6. Alert on new production error groups and sustained backend upstream/stream failures. Exclude `operation:support.report`, cancellations, and permission/credit blocks from error paging.

Do not label sampled error counts as complete failure rates: a separate measurement denominator is required. Do not create user counts or account cohorts from these session-only identifiers.

## Validation state

Local formatter/linter checks are allowed. Tests, typechecks, builds, and app launches remain CI-only unless the user requests local validation. Require root typecheck/tests, backend checks/tests, workflow tests, and relevant packaged smoke jobs before release readiness.

Manual or separately approved automated checks must cover Recent Errors selection/copy, empty and populated lists, Command-K dismissal/focus, independent windows, same-message repeated failures, streaming cancellation, disabled reporting, real sign-in, update next launch, privacy canaries, and remote symbolication. None is replaced by a healthy endpoint or an upload command that exits successfully.

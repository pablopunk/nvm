# Desktop and backend observability

PostHog is the single place for errors, product usage, and performance. Axiom keeps backend request logs.

## What is collected

- **Errors:** main-process exceptions, renderer exceptions (posthog-js), and backend exceptions arrive as `$exception` events with a `support_ref` that matches the local diagnostic record.
- **Performance and reliability:** every instrumented operation (`command.execute`, `view.load`, `ai.stream`, `auth.signin`, `update.*`, ...) emits one `operation_completed` event with `operation`, `outcome`, `stage`, `duration_ms`, `journey_id`, and `action_id`. Successful `job.run` background ticks are skipped; failures are kept. Backend requests emit `backend_request` with `route`, `method`, `status`, and `duration_ms`.
- **Usage:** `feature.used` is the one canonical "someone used the app" event. It is emitted only for operations a person starts (`command.run`, `view.action`, `ai.chat`, `extension.generate`, `extension.install`, `auth.signin`, `update.install`), with a `variant` from a closed set: action kinds and built-in command ids from this repository, or the diagnostic component. User extensions collapse to the single bucket `user-extension`, so their names, ids and text never leave the machine. `app.launched` marks each start.
- **Alerts:** failed cron checks and exhausted provider chains emit `backend_alert` with an `alert` property.
- **Identity:** every host event is sent under one stable random install id (`distinct_id`) and never builds a person profile, so a device's history does not split at sign-in. Whose install it is travels separately as `install.owner_observed` with a SHA-256 hash of the account id, so ownership rules can change after the fact without touching event history. No email or raw account id reaches PostHog. Every event carries `channel` (`stable` or `prerelease`, from the build version), `app_version`, `os_type`, `os_release` and `arch`.
- **Not collected on purpose:** prompts, model output, clipboard content, and screen content are never attached to events. Exception messages and stack traces are sent as the app produced them, so errors that embed user content (for example a provider error echoing a prompt) can carry it; fix such errors at the source.

The `Send Technical Diagnostics` setting is the single switch. It gates the host client and the renderer SDK, and the host broadcasts changes to every window immediately. Development builds never send.

The host client drops queued batches at send time once the setting turns off. The renderer SDK stops capturing at once, but a request already queued by posthog-js (a few seconds of events at most) can still be delivered. The renderer SDK is not even loaded until the host setting reads true, so browser-stored consent from an earlier launch can never act on its own. Errors raised before that moment are not reported. Autocapture, pageviews, replay, surveys and flags are disabled: only anonymous exceptions leave the renderer; the host sends a `renderer_failure` event with the install id and the same `support_ref`, which is the join key.

## First-party ingest route

The desktop host process does not talk to PostHog directly. It posts PostHog-format batches to `https://api.nvm.fyi/api/ingest/batch/` (`src/backend/src/pages/api/ingest/batch.ts`), which:

- keeps the project key server-side (the app ships only a placeholder key),
- accepts a fixed set of event names from a real install id and drops everything else, rejects client-supplied person properties, location and ip, and forces every event anonymous,
- rate-limits per IP, caps batch and event size, and accepts gzip,
- forwards accepted events to PostHog and answers 503 on failure (or when the server key is missing) so the app retries a few times; after that it drops the batch, so a long PostHog outage loses events instead of queueing them,
- trims oversized exceptions (message to 2,000 characters, stack to the 50 frames nearest the throw) instead of dropping them,
- logs a per-batch summary (counts by event name) to Axiom,
- stops everything with `TELEMETRY_INGEST_ENABLED=false` in the Vercel environment (it answers success so installed apps stop retrying).

Renderer exceptions still go from `posthog-js` straight to PostHog because they need the build's source-map chunk ids. Backend events are sent from the server and do not pass through this route.

## Website analytics

The public pages (`Layout.astro`: landing, pricing, legal) load `posthog-js` through `src/components/Posthog.astro` for page views, page leaves, web vitals, exceptions, and autocapture. It runs in cookieless mode (`cookieless_mode: 'always'`, project setting `cookieless_server_hash_mode: 2`), so it sets no cookies and stores nothing in the browser, and requests go through the same-origin `/ingest` rewrites in `vercel.json`. The signed-in console (`ConsoleLayout.astro`, profile and admin) and `/invite` are not instrumented. With no `POSTHOG_KEY` (previews, local development) nothing loads.

## Interpreting events

- Count active installs with `feature.used`, never with `operation_completed`: the latter includes extension loads, update polling and other housekeeping.
- Event counts are not call counts: an operation can emit both `operation_completed` and `feature.used`. `feature.used` is emitted once per feature per journey, so a wrapped operation (the sign-in IPC around the sign-in itself) counts once.
- Compare `channel` before comparing releases; dev and nightly builds are `prerelease` and include the maintainers' own installs.
- Unknown values stay absent. Do not backfill a missing property from another event.
- Renderer exceptions carry no install id. Join them to a host `renderer_failure` event on `support_ref`.

## Where it lives

- `src/app/electron/diagnostics/posthog-client.ts` owns the host client, identity, and consent.
- `src/app/electron/diagnostics/diagnostic-reporting.ts` plugs into the operation seam in `observability.ts` and keeps the local support index.
- `src/app/palette/posthog.ts` owns the renderer SDK, identification, and consent sync.
- `src/backend/src/lib/error-tracking.ts` owns backend capture and per-request context; the middleware flushes after each response.

## Local support lookup

Use **Show Recent Errors** in Command-K. Each row can copy an error reference or safe diagnostics. **Report a Problem** emits a `support_report` event. The index is `nevermind-diagnostics.json` in `app.getPath('logs')`: at most 100 records for seven days, written asynchronously.

An event ID or `flush()` is not delivery proof. Only a successful PostHog event lookup for the `support_ref` proves remote availability.

## Release source maps

Main, preload, renderer, and renderer chunks produce hidden maps. Electron packaging excludes `.map` files. The release identity is `nevermind@<version>+<commit>.<build-platform>`.

Release builds run `posthog-cli sourcemap process` (pinned in `scripts/upload-desktop-source-maps.cjs`) over `dist/` before packaging, without rebuilding.

Required configuration:

- Renderer build: `VITE_POSTHOG_KEY`, `VITE_POSTHOG_HOST` (project token and ingestion host; both are public). The host process needs neither.
- Release upload only: `POSTHOG_CLI_API_KEY` (personal API key with error tracking write) and `POSTHOG_CLI_PROJECT_ID`. These come from CI secrets and variables and never from build definitions.
- Backend: `POSTHOG_KEY` and `POSTHOG_HOST` in the Vercel environment.

## Verification

The packaged diagnostics smoke test points the host client at a loopback collector with `POSTHOG_HOST_DESKTOP` (diagnostic test mode only) and asserts `support_report`, `operation_completed`, and a controlled main-process `$exception`. Renderer delivery depends on the build-time project key and is validated against a real project.

# Desktop and backend observability

PostHog is the single place for errors, product usage, and performance. Axiom keeps backend request logs.

## What is collected

- **Errors:** app exceptions and backend exceptions arrive as `$exception` events with a `support_ref` that matches the local diagnostic record.
- **Performance and reliability:** every instrumented operation (`command.execute`, `view.load`, `ai.stream`, `auth.signin`, `update.*`, ...) emits one `operation_completed` event with `operation`, `label`, `outcome`, `stage`, `duration_ms`, `journey_id`, and `action_id`. Successful `job.run` background ticks are skipped; failures are kept. Backend requests emit `backend_request` with `route`, `method`, `status`, and `duration_ms`.
- **Usage:** `feature.used` is the one canonical "someone used the app" event. It is emitted only for operations a person starts (`command.run`, `view.action`, `ai.chat`, `extension.generate`, `extension.install`, `auth.signin`, `update.install`), with a `variant` from a closed set: action kinds and built-in command ids from this repository, or the diagnostic component. User extensions collapse to the single bucket `user-extension`, so their names, ids and text never leave the machine. `app.launched` marks each start.
- **Views and sessions:** `$pageview` records one settled, visible navigation: opening the palette, an extension view or AI chat, going back, a shortcut's resolved view, or reopening the window. Loading placeholders, refreshes (a view replaced by itself) and hiding never count, and neither does a view retained from before a shortcut revealed the window. The renderer names only a host-defined `view` (`palette`, `extension`, `ai-chat`); the host assigns identity and session, and the ingest route rebuilds the event with a synthetic URL (`https://app.nevermind.invalid/<view>`) and drops anything else the client sent. The main process owns one `$session_id` (UUIDv7): pageviews and person-initiated operations keep it alive, it restarts after 30 idle minutes or 24 hours, and consent withdrawal discards it for good: an operation that began earlier does not bring the old id back. Anything running under a background job (periodic AI reviews, extension jobs) is never activity: it neither keeps a session alive nor emits `feature.used`. `feature.used` and the `operation_completed` of those operations carry the session that was current when the operation started, so a long AI stream stays in the session it began in. Shortcut actions that never show the palette still count as activity.
- **Alerts:** failed cron checks and exhausted provider chains emit `backend_alert` with an `alert` property.
- **Identity:** every host event is sent under one stable random install id (`distinct_id`) and never builds a person profile, so a device's history does not split at sign-in. Whose install it is travels separately as `install.owner_observed` with a SHA-256 hash of the account id, so ownership rules can change after the fact without touching event history. No email or raw account id reaches PostHog. Every event reaching PostHog through the ingest route carries `source: desktop`, stamped by the server, so desktop data can be separated from website data. Every event carries `channel` (`stable` or `prerelease`, from the build version), `app_version`, `os_type`, `os_release` and `arch`.
- **Not collected on purpose:** prompts, model output, clipboard content, and screen content are never attached to events. Exception messages and stack traces are sent as the app produced them, so errors that embed user content (for example a provider error echoing a prompt) can carry it; fix such errors at the source.

The `Send Technical Diagnostics` setting is the single switch. It gates the analytics client. Development builds never send.

The client drops queued batches at send time once the setting turns off.

## First-party ingest route

The desktop host process does not talk to PostHog directly. It posts PostHog-format batches to `https://api.nvm.fyi/api/ingest/batch/` (`src/backend/src/pages/api/ingest/batch.ts`), which:

- keeps the project key server-side (the app ships only a placeholder key),
- accepts a fixed set of event names from a real install id and drops everything else, rejects client-supplied person properties, location and ip, and forces every event anonymous,
- accepts `$pageview` only with a fixed `view` and a UUIDv7 `$session_id`, rebuilds it from those parts plus the host's common context, and stamps `source: desktop` on every event (additive: deploy it before the desktop release that sends pageviews, because the previous allowlist drops them),
- rate-limits per IP, caps batch and event size, and accepts gzip,
- forwards accepted events to PostHog and answers 503 on failure (or when the server key is missing) so the app retries a few times; after that it drops the batch, so a long PostHog outage loses events instead of queueing them,
- trims oversized exceptions (message to 2,000 characters, stack to the 50 frames nearest the throw) instead of dropping them,
- logs a per-batch summary (counts by event name) to Axiom,
- stops everything with `TELEMETRY_INGEST_ENABLED=false` in the Vercel environment (it answers success so installed apps stop retrying).

Backend events are sent from the server and do not pass through this route.

## Website analytics

The public pages (`Layout.astro`: landing, pricing, legal) load `posthog-js` through `src/components/Posthog.astro` for page views, page leaves, web vitals, exceptions, and autocapture. It runs in cookieless mode (`cookieless_mode: 'always'`, project setting `cookieless_server_hash_mode: 2`), so it sets no cookies and stores nothing in the browser, and requests go through the same-origin `/ingest` rewrites in `vercel.json`. The signed-in console (`ConsoleLayout.astro`, profile and admin) and `/invite` are not instrumented. With no `POSTHOG_KEY` (previews, local development) nothing loads.

## Interpreting events

- Count active installs with `feature.used`, never with `operation_completed`: the latter includes extension loads, update polling and other housekeeping.
- Event counts are not call counts: an operation can emit both `operation_completed` and `feature.used`. `feature.used` is emitted once per feature per journey, so a wrapped operation (the sign-in IPC around the sign-in itself) counts once.
- `$pageview` measures exposure and `feature.used` measures action. Analyze paths with pageviews and activity with `feature.used`; both share `$session_id`. PostHog's default web analytics overview is unfiltered and also includes website traffic, so filter dashboards on `source` (`desktop` or absent for the website).
- Compare `channel` before comparing releases; dev and nightly builds are `prerelease` and include the maintainers' own installs.
- Unknown values stay absent. Do not backfill a missing property from another event.

## Where it lives

- `src/native/internal/diag/analytics` owns the client, identity, consent, the closed event schema and the sender.
- `src/native/internal/diag` plugs into the operation seam and keeps the local support index.
- `src/native/internal/diag/timing` measures operations, command use and view actions.
- `src/backend/src/lib/error-tracking.ts` owns backend capture and per-request context; the middleware flushes after each response.

## Local support lookup

Use **Show Recent Errors** in Command-K. Each row can copy an error reference or safe diagnostics. **Report a Problem** emits a `support_report` event. The index is `nevermind-diagnostics.json` in the app's logs directory: at most 100 records for seven days, written asynchronously.

An event ID or `flush()` is not delivery proof. Only a successful PostHog event lookup for the `support_ref` proves remote availability.

## Configuration

- Backend: `POSTHOG_KEY` and `POSTHOG_HOST` in the Vercel environment.

## Verification

The analytics tests send to a fake collector and assert `support_report`, `operation_completed` and `$exception` events and their allow-listed properties. Delivery to a real project is validated against that project.

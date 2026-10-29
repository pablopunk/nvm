# PostHog Connector

Use this connector for error issues, `$exception` events, `operation_completed` timings, `backend_alert` cron alerts, release/source-map questions, and desktop/backend pivots.

Prefer the PostHog MCP (`exec`) when connected: it covers error tracking issues, event queries (HogQL), dashboards, and insights. Run `info <tool>` before calling a tool; never guess schemas.

## Establish scope first

- **Project:** call `project-get` and confirm the project and region host match `VITE_POSTHOG_HOST` (desktop build) and `POSTHOG_HOST` (Vercel).
- **Backend:** `POSTHOG_KEY` must exist in the linked Vercel production environment, or backend capture is silently off.
- **Desktop:** reporting only starts in packaged builds, after stored settings load, and only while `Send Technical Diagnostics` is on. Development builds never send.

## Pivot keys

- `support_ref`: random host identifier shown by **Show Recent Errors**; present on `$exception` and `support_report` events and in `nevermind-diagnostics.json`.
- `request_id`: matches the backend `x-request-id` and Axiom rows.
- `journey_id` / `action_id`: tie `operation_completed` events from one user action together; the backend echoes the desktop ids on `backend_request`.
- `release`: `nevermind@<version>+<commit>.<platform>` (desktop) or the backend commit SHA.

## Useful queries

Recent failures by operation:

```sql
SELECT properties.operation, properties.stage, count() AS failures
FROM events
WHERE event = '$exception' AND timestamp > now() - INTERVAL 1 DAY
GROUP BY 1, 2 ORDER BY failures DESC
```

Operation latency by release:

```sql
SELECT properties.release, properties.operation,
  quantile(0.5)(toFloat(properties.duration_ms)) AS p50_ms,
  quantile(0.95)(toFloat(properties.duration_ms)) AS p95_ms
FROM events
WHERE event = 'operation_completed' AND timestamp > now() - INTERVAL 7 DAY
GROUP BY 1, 2
```

## Source maps

Release builds upload with `posthog-cli sourcemap process` using `POSTHOG_CLI_API_KEY` and `POSTHOG_CLI_PROJECT_ID`. Check event-read, symbol-set upload, and alert-management access separately; one successful probe does not prove the others. If stack frames are minified, confirm the release's symbol sets exist for the exact `release` value.

Extract environment, release, timestamp, exception, route/status, `request_id`, deployment id, and the failing release before moving to Axiom or Vercel.

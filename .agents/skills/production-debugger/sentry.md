# Sentry Connector

Verified skill search result: `getsentry/sentry-for-ai@sentry-workflow` (`https://skills.sh/getsentry/sentry-for-ai/sentry-workflow`). Related setup/SDK skills found: `sentry-sdk-setup`, `sentry-node-sdk`.

Use this connector for Sentry alerts, issue URLs, event URLs, crash spikes, cron alert messages, and backend/desktop exceptions.

## Desktop versus backend

Do not treat the backend and the packaged Electron app as one integration.

- **Backend:** verify `SENTRY_DSN` in the linked Vercel production environment.
- **Desktop:** inspect `src/electron/sentry.ts` and the packaged release. The
  desktop DSN is selected from `SENTRY_DSN_DESKTOP`, then
  `NEVERMIND_SENTRY_DSN`, then its built-in fallback. It does not inherit the
  backend Vercel runtime environment.

When a desktop Sentry project has no events, establish these facts before
calling it a DSN misconfiguration:

1. Which Sentry project the selected desktop DSN targets (use dashboard or
   trusted configuration evidence; do not guess from a project name).
2. Whether `@sentry/electron/main` loaded in the packaged build. Loading is
   best-effort, so a load failure can leave the app running with only a local
   warning.
3. Whether the symptom happened in the Electron main process or renderer. A
   main-only SDK does not report renderer/UI errors.
4. Whether the error was caught and handled locally. `console.error` and a
   caught exception do not create a Sentry event unless code calls
   `captureException`.
5. The installed app version/release, environment, and the Sentry project’s
   time window.

For an end-to-end delivery check, first obtain explicit approval to trigger a
controlled desktop test error. Verify the resulting event in the intended
project with its release and environment tags; do not count SDK initialization
or a present DSN as proof of delivery.

## Inputs accepted

- Sentry issue URL, event URL, short id, issue id, event id.
- Alert title and approximate time.
- Project/environment/release if no URL is available.

## Fetch evidence

Use the installed Sentry workflow skill if available. Otherwise use authenticated Sentry tooling/API; do not guess project ids.

If Sentry env vars are missing locally, pull production env from the linked Vercel project into a non-committed file and only report presence/absence, never values:

```bash
umask 077
cd backend
vercel env pull .vercel/.env.production.local --environment=production --yes
```

For this app, Vercel production env currently includes `SENTRY_AUTH_TOKEN`, `SENTRY_ORG`, and `SENTRY_PROJECT`. It does not imply the token has every Sentry API scope.

```bash
sentry-cli info
sentry-cli projects list
sentry-cli issues list --org "$SENTRY_ORG" --project "$SENTRY_PROJECT" --query 'is:unresolved environment:production' --stats-period 24h
sentry-cli issues info "$ISSUE_ID"
```

The project event endpoint works with a Sentry event id and project slug, even when issue/group endpoints may 403 with a project-scoped token:

```bash
curl -sS -H "Authorization: Bearer $SENTRY_AUTH_TOKEN" \
  "https://sentry.io/api/0/projects/$SENTRY_ORG/$SENTRY_PROJECT/events/$EVENT_ID/" \
  | jq '{eventID,groupID,title,message,platform,release,tags,contexts,entries,dateCreated}'
```

Use project event search for blast-radius checks when issue APIs are forbidden:

```bash
curl -sS -H "Authorization: Bearer $SENTRY_AUTH_TOKEN" \
  "https://sentry.io/api/0/projects/$SENTRY_ORG/$SENTRY_PROJECT/events/?query=$QUERY&per_page=10" \
  | jq '[.[] | {eventID,id,groupID,title,dateCreated}]'
```

Use issue endpoints only when the token permits them:

```bash
curl -sS -H "Authorization: Bearer $SENTRY_AUTH_TOKEN" \
  "https://sentry.io/api/0/issues/$ISSUE_ID/" \
  | jq '{id,shortId,title,level,status,count,userCount,firstSeen,lastSeen,permalink,project,metadata}'
```

## Extract and pivot

Extract environment, release, timestamp, exception, route/transaction/status, tags/contexts, `request_id`, deployment id/SHA, first/last seen, event count, and affected users. Also inspect `contexts.response.status_code`, `contexts.cloud_resource`, and recent breadcrumbs: for async uncaught exceptions, Sentry `dateCreated` can be minutes after the request breadcrumbs/DB usage row. Then pivot to `axiom.md` with the Sentry timestamp and breadcrumb/request timestamp ±15m plus request id, route, host, release, or error text.

Never paste secrets, cookies, auth headers, raw request bodies, prompts, or full payloads.

# Sentry Connector

Verified skill search result: `getsentry/sentry-for-ai@sentry-workflow` (`https://skills.sh/getsentry/sentry-for-ai/sentry-workflow`). Related setup/SDK skills found: `sentry-sdk-setup`, `sentry-node-sdk`.

Use this connector for Sentry alerts, issue URLs, event URLs, crash spikes, cron alert messages, and backend/desktop exceptions.

## Inputs accepted

- Sentry issue URL, event URL, short id, issue id, event id.
- Alert title and approximate time.
- Project/environment/release if no URL is available.

## Fetch evidence

Use the installed Sentry workflow skill if available. Otherwise use authenticated Sentry tooling/API; do not guess project ids.

```bash
sentry-cli info
sentry-cli projects list
sentry-cli issues list --org "$SENTRY_ORG" --project "$SENTRY_PROJECT" --query 'is:unresolved environment:production' --stats-period 24h
sentry-cli issues info "$ISSUE_ID"
```

```bash
curl -sS -H "Authorization: Bearer $SENTRY_AUTH_TOKEN" \
  "https://sentry.io/api/0/issues/$ISSUE_ID/" \
  | jq '{id,shortId,title,level,status,count,userCount,firstSeen,lastSeen,permalink,project,metadata}'

curl -sS -H "Authorization: Bearer $SENTRY_AUTH_TOKEN" \
  "https://sentry.io/api/0/issues/$ISSUE_ID/events/latest/" \
  | jq '{eventID,title,message,platform,environment,release,tags,contexts,entries,dateCreated}'
```

## Extract and pivot

Extract environment, release, timestamp, exception, route/transaction/status, tags/contexts, `request_id`, deployment id/SHA, first/last seen, event count, and affected users. Then pivot to `axiom.md` with the timestamp ±15m plus request id, route, host, release, or error text.

Never paste secrets, cookies, auth headers, raw request bodies, prompts, or full payloads.

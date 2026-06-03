# Neon Connector

Verified skill search result: `neondatabase/agent-skills@neon-postgres` (`https://skills.sh/neondatabase/agent-skills/neon-postgres`). Related: `neondatabase/ai-rules@neon-drizzle` for Drizzle-specific guidance.

Use this connector for database connectivity, branch/project state, slow queries, migration failures, health-check DB failures, balance/usage data checks, and serverless Postgres incidents.

## Source of truth

The Neon skill says to verify claims against current Neon docs. Neon docs can be fetched as markdown by appending `.md` to docs URLs or requesting `Accept: text/markdown`.

## Operational flow

1. Start from vendor evidence: Sentry DB exception, Axiom `health_db_failed`, Vercel build/migration failure, or a user/request id needing DB correlation.
2. Confirm the issue type: connection/auth, branch/project availability, migration, data shape, query latency, or app logic.
3. Use Neon-specific tooling/docs before generic Postgres advice.
4. For SQL, use read-only queries first and redact data.

## Nevermind-specific correlation

- AI proxy issues correlate through `request_id` in usage records.
- Credit/balance issues correlate through ledger rows.
- Migration failures should show in Vercel build events because migrations run during backend build.
- Sentry error `terminating connection due to administrator command` from `@neondatabase/serverless` means Postgres/Neon closed the connection server-side. Treat it as a Neon connection/admin event until Axiom/Sentry evidence shows an app regression; verify current `/api/health` and search for repeated events before changing code.

Confirm before prod DB writes, migrations, admin grants, credit edits, branch changes, or destructive SQL.

# Safety Rules and Durable Gotchas

## Safety rules

- Do not run destructive Vercel, Sentry, Neon, WorkOS, Upstash, GitHub, or release operations without confirming first.
- Do not commit `.env`, `.vercel/`, tokens, DSNs with secret keys, cookies, authorization headers, raw user prompts/responses, or production data dumps.
- Do not paste raw secret values into chat output; say which var/service was checked and whether it matched expected shape.
- Before declaring external work inaccessible, read its vendor playbook and use safe read-only checks to establish installed-tool support, authentication, and the exact project/team/environment. Missing worktree-local config alone is not evidence of missing access.
- Before configuration or deployment changes, name the target host/environment and terminal acceptance state; prefer preview/test, confirm destructive, security-sensitive, billing, release, or live changes, then verify the intended public aliases before exercising the real user flow.
- After any fix, verify the exact failing endpoint/flow again and correlate the new result with logs/Sentry/request id.
- After two symptom patches in the same code path, stop and audit the architecture.

## Durable gotchas

- OAuth callback URLs must target the canonical non-redirecting host. If apex redirects to `www`, register the redirected-to host with WorkOS; OAuth servers do not follow 308s with state cookies.
- WorkOS redirect URI management is dashboard-only.
- `WORKOS_REDIRECT_URI`, `PUBLIC_DASHBOARD_URL`, and packaged desktop `PRODUCTION_BASE_URL` must agree with the actual host split.
- `SENTRY_DSN` missing means no backend Sentry integration; absence of Sentry events is evidence only after checking env/integration.
- Vercel production env can be pulled with `cd backend && vercel env pull .vercel/.env.production.local --environment=production --yes`; keep the file under `.vercel/`, set restrictive permissions, and never print values.
- A Sentry project event endpoint can work with an event id while issue/group endpoints return 403 for project-scoped tokens; prefer `/api/0/projects/$SENTRY_ORG/$SENTRY_PROJECT/events/$EVENT_ID/` for alert emails containing an event id.
- Vercel env edits require redeploys.
- Vercel project root is `backend/`; running deployment commands from repo root can target the wrong tree.
- Build migrations fail the Vercel build fast. If backend `pnpm build` succeeded on Vercel, `tsx scripts/migrate.ts` completed.
- Transparent reverse-proxy behavior is intentional for AI services.

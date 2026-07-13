# Nevermind backend

Astro (SSR) + Vercel + Neon Postgres (Drizzle) + WorkOS AuthKit.

Serves the dashboard at `nvm.fyi` and the API the desktop app talks to.

## Setup

1. `cp .env.example .env` and fill in:
   - Neon `DATABASE_URL`
   - Production WorkOS `API_KEY`, `CLIENT_ID`, `REDIRECT_URI`
   - `WORKOS_COOKIE_PASSWORD` — `openssl rand -base64 32`
   - Stripe `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, subscription/top-up price IDs and credit grants
2. `pnpm install`
3. `pnpm db:migrate` to apply migrations on Neon (idempotent; auto-bootstraps an existing DB)
4. `pnpm dev` → http://localhost:4321

## Axiom logs

The backend always writes structured JSON to stdout. To also ingest production
logs directly into Axiom (without a Vercel Log Drain), set `AXIOM_TOKEN` and
`AXIOM_DATASET` in the Vercel project. Use an Axiom API token with ingest
permission; set `AXIOM_EDGE` only when the dataset uses a non-default Axiom edge.
The middleware flushes the SDK batch through Vercel's `waitUntil()` lifecycle
hook, so ingestion does not delay responses.

## Migrations

Versioned via drizzle-kit. SQL lives in `backend/drizzle/`.

- Edit `src/db/schema.ts`
- `pnpm db:generate` → produces a new `drizzle/NNNN_*.sql`
- `pnpm db:migrate` → applies pending migrations (also runs automatically on `pnpm build` / Vercel deploy)

## Deploy

Push to GitHub, import in Vercel, set env vars, and point `nvm.fyi` + `api.nvm.fyi` at it. `WORKOS_REDIRECT_URI` remains the production callback (`https://nvm.fyi/api/auth/callback`). Release v2 uses disjoint, one-use `v:2` production and Preview-gateway states; legacy state, grants, sealed sessions, and `nvm_session` material are rejected immediately.

Preview sign-in is fail-closed until the production gateway capability is configured. Trusted Preview deployments require `VERCEL_ENV=preview`, an exact `VERCEL_URL` matching `nvm-<deployment-token>-pablo-varelas-projects-4f86af8b.vercel.app`, `PREVIEW_GATEWAY_ORIGIN=https://nvm.fyi`, `PREVIEW_START_KEY`, `GATEWAY_STATE_KEY`, `GATEWAY_STATE_REDIS_URL/TOKEN`, production `UPSTASH_REDIS_REST_URL/TOKEN`, and `PREVIEW_SESSION_KEY`. Preview deliberately uses the production WorkOS client, `DATABASE_URL`, Redis, settings, and email; there are no separate Preview DB/Redis/WorkOS/email variables. The callback writes no local production data for Preview flows, while the exchange completes against the same production-backed runtime and mints only the host-scoped `nvm_preview_session`.

Required Redis identities and ACL boundaries:

- `gateway_state`: `GATEWAY_STATE_REDIS_URL/TOKEN`, key pattern `~nvm:gateway:state:v2:*`; only state `SET NX EX` and atomic consume (`GETDEL`).
- `prod_runtime`: the current production-faithful Preview gateway uses the broad production `UPSTASH_REDIS_REST_URL/TOKEN` credential for grant writes/atomic consumption and the Preview runtime. This credential does not enforce grant-only key-level isolation; do not claim `NOPERM` or a separate grant-writer ACL in this release. The opaque grant namespace and v2 protocol still provide one-use separation, while protected trusted-branch secret scoping is the deployment control.

Do not provision or copy these secrets, ACLs, databases, WorkOS settings, or Vercel configuration as part of a code change. Verify the gateway capability at authenticated `/api/health/deployment` before enabling Preview sign-in. Vercel access logs, analytics, tracing, and observability pipelines must redact `grant`, `state`, `intent`, and `code` query values; the exchange route emits no full URL telemetry. Scope production-equivalent secrets only to protected trusted branches and keep fork protection enabled. Rollback disables Preview gateway sign-in and requires a fresh login; it never restores the legacy parser or shared sealed-session handoff.

Only enable preview auth for trusted internal branches. In Vercel Project Settings → Environment Variables, scope WorkOS, cookie, database, and Redis secrets to the approved Preview branches rather than all previews. Keep Security → Git Fork Protection enabled so fork PR deployments cannot receive those variables. If fork or untrusted preview builds must run, use a separate non-production WorkOS client, cookie/state secret, Redis namespace, and isolated dependencies instead of production-equivalent credentials. Environment changes require a new deployment.

When working from a git worktree, ignored local env files are not inherited. Run `pnpm worktree:env:copy` from the worktree to copy env files from the canonical checkout without printing secrets; use `--dry-run` to preview and `--force` only when you intend to overwrite local env files. If `.vercel/project.json` is still missing, run `vercel link` before Vercel deploy commands; never commit `.env*` or `.vercel/` files.

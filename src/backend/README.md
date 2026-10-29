# Nevermind backend

Astro (SSR) + Vercel + Neon Postgres (Drizzle) + WorkOS AuthKit.

Serves the dashboard at `www.nvm.fyi` and the API the desktop app talks to.

The machine API base is the origin-only `https://api.nvm.fyi`; callers append
exactly one `/api/...`. Browser routes use relative `/api/...` paths on
`https://www.nvm.fyi`, which is not an API base. `PUBLIC_API_ORIGIN` and
`PRODUCTION_ORIGIN` are strict, validated production settings. Preview URLs
must be the exact protected deployment origin; missing or mismatched origins
fail closed.

## Setup

1. `cp .env.example .env` and fill in:
   - A development-only Neon `DATABASE_URL`
   - `NVM_ENV=development` and `NVM_DATABASE_ENV=development`
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

Versioned via drizzle-kit. SQL lives in `src/backend/drizzle/`.

- Edit `src/db/schema.ts`
- `pnpm db:generate` → produces a new `drizzle/NNNN_*.sql`
- `pnpm db:migrate` → applies pending migrations after validating the target identity

`pnpm build` never connects to the database or applies migrations. GitHub's
`Database migration` workflow is the only migration and deployment owner. It uses
the `database-development`, `database-preview`, or `database-production` GitHub
environment, where each environment must provide its own `DATABASE_URL` secret.
Production runs automatically for the exact commit from each successful `main`
push CI run. Manual production runs still require the workflow's explicit
approval input. Workflow concurrency and a PostgreSQL advisory lock prevent two
migrations from changing one target at the same time. Development runs
migrations only. Preview and production deploy the selected workflow commit only
after its migration succeeds; automatic Vercel Git deployments are disabled in
`vercel.json` so this workflow remains the only deployment owner.

Each deploy passes the checked-out full commit SHA to the Vercel CLI as
`--build-env NVM_BUILD_COMMIT_SHA=<sha>`. The build bakes it into the server bundle, and
`/api/health` and `/api/health/deployment` report it as `commit` next to the short
`version`. The nightly release gate requires a valid deployed identity and checks the public alias before publication.
`/api/health` probes the providers that live routes can reach, using the same resolution and refusals as requests: the free and pro smart and fast slots (their primary and the enabled failover links the proxy would try, after the proxy's key, format, and pricing skips), the audio route, and the decision route. A route the request path always refuses (a missing key or pricing, an unsupported provider) fails closed. A kill switch on the path takes its route out of the probe set, and a tripped route breaker is skipped as the proxy skips it. An unreadable breaker or failover lookup keeps the route serviceable but unverified, so readiness cannot pass on it. Each route's resolution errors stay with that route.

Liveness and readiness are separate. `upstream` is chat liveness with the released meaning: each tiered chat route has a candidate that answers without a 5xx, and none is unresolved. Audio and decision routes are optional features beside chat, so a missing, disabled, or refused one never flips `upstream` and never 503s a deployment whose chat works. `upstreamReady` is strict: every route, optional ones included, must be configured and serviceable, with no lookup left unverified, and every candidate must return a 2xx model catalog with a `data` array (Google's is `models`) and an accepted key. A missing or disabled audio or decision route therefore reports `upstreamReady: false`. OpenRouter's catalog answers anonymously, so its key is verified on the authenticated `/key` endpoint. OpenCode also serves its catalog anonymously for legacy keyless access, but it refuses an invalid bearer with 401, so a 2xx with the configured key does show the key is accepted. Readiness does not prove credit balance, rate limits, or that an authenticated inference call succeeds.

Each instance runs at most one provider evaluation at a time and caches a completed result for 30 seconds when ready and 5 seconds otherwise. A request waits at most 8 seconds for it. If that wait ends first, `upstreamReady` is false, and `upstream` carries the last completed liveness for up to 60 seconds, so a slow evaluation of a deployment that was live does not 503 it. On a cold instance with no completed evaluation, a first evaluation that outlasts that wait reports `upstream: false` until it completes. The database check stays live. The nightly gate requires `upstreamReady`.

Use expand-and-contract migrations: apply a backward-compatible schema change,
deploy compatible code only after migration succeeds, migrate data if needed,
and remove old schema in a later release. If a deployment fails after migration, roll code
back without rolling schema back; fix forward unless a separately reviewed
backward migration is safe for all released desktop clients.

## Pricing snapshots

Admission prices come only from durable snapshots that the scheduled refresh validated. The request path never fetches models.dev, OpenRouter endpoints, or the OpenRouter audio and decision catalogs. It restores the last validated copy from `pricing_snapshots` and answers from that.

- `/api/cron/pricing-refresh` runs daily at 05:30 UTC (`vercel.json`). It refreshes the models.dev sections, the endpoint price of every live OpenRouter route model (each model isolated from the others), and the OpenRouter audio and decision catalogs. It requires `CRON_SECRET` in production and fails closed without it. It returns 503 with a per-catalog report when anything failed, and reports the failure to PostHog as a `backend_alert` event.
- A validation inside 36 hours is current. A validation between 36 hours and 7 days is stale: every tier admits it, and each reservation holds twice its estimate, so a rate that moved while the snapshot aged cannot settle above its hold. Free spend is held the same way inside the monthly free-spend cap. Settlement always uses the actual cost. Past 7 days a price is refused for every tier with `pricing_unavailable`, `reason: stale`.
- A route whose price has never been validated refuses with `pricing_unavailable`, `reason: unverified`. Audio and decision routes with no validated catalog refuse as unavailable models.
- Saving an admin route refreshes that route's price first, so a new model is priced when it is chosen.

### Deploy gate

Bootstrap is automatic. The `Database migration` workflow runs `pnpm -C src/backend db:seed-pricing` after migrations and before any Vercel deploy. The script refreshes the snapshots, then requires every configured route to admit a current price from them: the primary and each enabled failover link of the free and pro smart and fast slots, the audio route, and the decision route. Kill switches remove a route the same way requests lose it. A failed gate stops the deploy, and the previous deployment keeps serving with its own pricing.

The gate proves prices and route configuration. It does not prove provider keys or upstream answers; `upstreamReady` on `/api/health` does that after the deploy. A failed refresh is reported but does not fail the gate while the snapshots it kept are still current, so an upstream outage cannot block a deploy whose prices are already verified.

To seed a fresh database by hand, run `pnpm db:migrate` and then the same command with that environment's `DATABASE_URL`, `NVM_ENV`, and `NVM_DATABASE_ENV`. The output lists each route's refusal, and the command exits non-zero until every route is priced.

### Bundled prices

The bundled entries in `src/backend/src/lib/pricing-bundled.ts` share one verification date, `BUNDLED_CATALOG_VERIFIED_AT`. A bundled entry ages on the same windows as a snapshot validated that day: current, then stale, then refused past the hard bound for every model. Nothing refreshes the bundle. Re-reconcile its entries against models.dev and move the date in a reviewed change when a bundled model must keep admitting; never add per-model expiry rules.

### Settlement overruns

A settlement whose cost exceeds its hold trips that route's breaker (`settlement_overrun`). Breakers stay tripped until an admin re-enables them. A debit the balance cannot cover trips no breaker: that is a balance problem, not a price problem.

Overrun also has an aggregate bound across routes. Each settlement adds the list-cost credits it does not collect to one durable counter per UTC day. That is the cost above its hold, plus any part the debit could not take because the user's balance or held pool ran short when it settled (logged as `credit_settlement_debit_limited`). The difference is added once, so an overrun that is also short of balance is not counted twice. A release adds nothing, because it settles no cost. Once that counter reaches `PRICING_OVERRUN_DAILY_CAP_CREDITS` (default 500 credits, about $5 of list charge or $1 of provider cost at the default markup), stale-priced admissions are refused on chat, audio, and decision routes until the next UTC day. Credits, not dollars: changing `CREDIT_USD` or `CREDIT_MARKUP` changes what the same cap is worth. Current prices are never refused by the cap.

The cap is read at admission and written at settlement, so it is not a hard ceiling. A hold admitted before the counter reached the cap still settles, and its uncollected cost can carry the counter past the cap; the stale margin bounds each such hold. A counter that cannot be read refuses stale admissions.

The cap does not hide an outage. A reached cap leaves each stale route refused, so `upstreamReady` reports it unserviceable and the deploy gate fails. The gate refreshes prices first, so the cap only holds it back when that refresh fails and the kept snapshots are stale.

### Outstanding production prerequisites

These cannot be verified from code, and they must hold before relying on this deploy path:

- `CRON_SECRET` must be set in the Vercel production environment. Vercel cron sends it as the bearer token. Without it the daily refresh returns 401, the snapshots age, and the deploy gate would not notice until the prices refuse.
- One missed daily run leaves prices stale on the second day, when they are held at twice the estimate. They are refused after a week of missed runs. Confirm the first refresh after deploy succeeds and that its PostHog alert channel is watched.
- Confirm the first production deploy of this change logged `"ready": true` from the seed step before any traffic reached it.

## Deploy

Push to GitHub, import in Vercel, set env vars, and point `www.nvm.fyi` + `api.nvm.fyi` at it. `WORKOS_REDIRECT_URI` is the non-redirecting production callback (`https://www.nvm.fyi/api/auth/callback`). Release v2 uses disjoint, one-use `v:2` production and Preview-gateway states; legacy state, grants, sealed sessions, and `nvm_session` material are rejected immediately.

Preview sign-in is fail-closed until the production gateway capability is configured. Trusted Preview deployments require `VERCEL_ENV=preview`, `NVM_ENV=preview`, `NVM_DATABASE_ENV=preview`, an isolated Preview `DATABASE_URL`, an exact `VERCEL_URL` matching `nvm-<deployment-token>-pablo-varelas-projects-4f86af8b.vercel.app`, `PREVIEW_GATEWAY_ORIGIN=https://www.nvm.fyi`, `PREVIEW_START_KEY`, `GATEWAY_STATE_KEY`, `GATEWAY_STATE_REDIS_URL/TOKEN`, production `UPSTASH_REDIS_REST_URL/TOKEN`, and `PREVIEW_SESSION_KEY`. Preview uses the production WorkOS gateway and Redis runtime but must not use the production database credential. The callback writes no local production data for Preview flows, while the exchange completes against the production gateway and mints only the host-scoped `nvm_preview_session`.

Required Redis identities and ACL boundaries:

- `gateway_state`: `GATEWAY_STATE_REDIS_URL/TOKEN`, key pattern `~nvm:gateway:state:v2:*`; only state `SET NX EX` and atomic consume (`GETDEL`).
- `prod_runtime`: the current production-faithful Preview gateway uses the broad production `UPSTASH_REDIS_REST_URL/TOKEN` credential for grant writes/atomic consumption and the Preview runtime. This credential does not enforce grant-only key-level isolation; do not claim `NOPERM` or a separate grant-writer ACL in this release. The opaque grant namespace and v2 protocol still provide one-use separation, while protected trusted-branch secret scoping is the deployment control.

Do not provision or copy these secrets, ACLs, databases, WorkOS settings, or Vercel configuration as part of a code change. Verify the gateway capability at authenticated `/api/health/deployment` before enabling Preview sign-in. Vercel access logs, analytics, tracing, and observability pipelines must redact `grant`, `state`, `intent`, and `code` query values; the exchange route emits no full URL telemetry. Scope production-equivalent secrets only to protected trusted branches and keep fork protection enabled. Rollback disables Preview gateway sign-in and requires a fresh login; it never restores the legacy parser or shared sealed-session handoff.

Only enable preview auth for trusted internal branches. In Vercel Project Settings → Environment Variables, scope WorkOS, cookie, database, and Redis secrets to the approved Preview branches rather than all previews. Keep Security → Git Fork Protection enabled so fork PR deployments cannot receive those variables. If fork or untrusted preview builds must run, use a separate non-production WorkOS client, cookie/state secret, Redis namespace, and isolated dependencies instead of production-equivalent credentials. Environment changes require a new deployment.

When working from a git worktree, ignored local env files are not inherited. Run `pnpm worktree:env:copy` from the worktree to copy env files from the canonical checkout without printing secrets; use `--dry-run` to preview and `--force` only when you intend to overwrite local env files. If `.vercel/project.json` is still missing, run `vercel link` before Vercel deploy commands; never commit `.env*` or `.vercel/` files.

After an approved production deployment, follow the
[production auth and proxy billing smoke](../../docs/production-auth-proxy-smoke.md)
to verify the real WorkOS callback, packaged device handoff, structured tool
calling, billing debit, and non-zero Google usage telemetry.

Billing is intentionally simple: `STRIPE_SUBSCRIPTION_TIERS` and
`STRIPE_TOP_UP_PACKS` map Stripe price IDs to credits, and Stripe owns the
amounts. If a webhook names a price that is not in those maps, it fails with a
500 so Stripe retries; fix the env and redeploy, and the retry grants credits.
The live Stripe webhook must send `checkout.session.completed`, `invoice.paid`,
`customer.subscription.updated`, and `customer.subscription.deleted`, and the
Billing Portal needs a saved live-mode configuration.

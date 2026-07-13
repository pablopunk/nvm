# Nevermind backend

Astro (SSR) + Vercel + Neon Postgres (Drizzle) + WorkOS AuthKit.

Serves the dashboard at `nvm.fyi` and the API the desktop app talks to.

## Setup

1. `cp .env.example .env` and fill in:
   - Neon `DATABASE_URL`
   - WorkOS `API_KEY`, `CLIENT_ID`, `REDIRECT_URI`
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

Push to GitHub, import in Vercel, set env vars, point `nvm.fyi` + `api.nvm.fyi` at it. Keep `WORKOS_REDIRECT_URI` set to the production callback (`https://nvm.fyi/api/auth/callback`): Vercel preview deployments use a short-lived encrypted handoff from that callback to establish their own host-only session cookie.

Preview authentication is intentionally limited to this project's deployment hostname pattern (`nvm-git-*-pablo-varelas-projects-4f86af8b.vercel.app`). The sign-in state is signed, expires after 60 seconds, and is consumed once from the shared Upstash Redis store; preview auth fails closed when that store is unavailable. The production callback does not set an `nvm.fyi` session cookie for preview logins.

Only enable preview auth for trusted internal branches. In Vercel Project Settings → Environment Variables, scope WorkOS, cookie, database, and Redis secrets to the approved Preview branches rather than all previews. Keep Security → Git Fork Protection enabled so fork PR deployments cannot receive those variables. If fork or untrusted preview builds must run, use a separate non-production WorkOS client, cookie/state secret, Redis namespace, and isolated dependencies instead of production-equivalent credentials. Environment changes require a new deployment.

When working from a git worktree, ignored local env files are not inherited. Run `pnpm worktree:env:copy` from the worktree to copy env files from the canonical checkout without printing secrets; use `--dry-run` to preview and `--force` only when you intend to overwrite local env files. If `.vercel/project.json` is still missing, run `vercel link` before Vercel deploy commands; never commit `.env*` or `.vercel/` files.

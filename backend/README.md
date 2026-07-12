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

## Migrations

Versioned via drizzle-kit. SQL lives in `backend/drizzle/`.

- Edit `src/db/schema.ts`
- `pnpm db:generate` → produces a new `drizzle/NNNN_*.sql`
- `pnpm db:migrate` → applies pending migrations (also runs automatically on `pnpm build` / Vercel deploy)

## Deploy

Push to GitHub, import in Vercel, set env vars, point `nvm.fyi` + `api.nvm.fyi` at it. Keep `WORKOS_REDIRECT_URI` set to the production callback (`https://nvm.fyi/api/auth/callback`): Vercel preview deployments use a short-lived encrypted handoff from that callback to establish their own host-only session cookie.

When working from a git worktree, ignored local env files are not inherited. Run `pnpm worktree:env:copy` from the worktree to copy env files from the canonical checkout without printing secrets; use `--dry-run` to preview and `--force` only when you intend to overwrite local env files. If `.vercel/project.json` is still missing, run `vercel link` before Vercel deploy commands; never commit `.env*` or `.vercel/` files.

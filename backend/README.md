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

## Credits and billing

Credits are user-facing retail units, not raw provider dollars. By default `CREDIT_USD=0.01` and `CREDIT_MARKUP=5`, so 1 credit represents about one cent of retail usage while covering roughly $0.002 of raw model cost. New and returning users receive a monthly free allowance up to `MONTHLY_FREE_CREDITS` free credits; paid Stripe prices map to monthly subscription or one-time top-up credit grants through `STRIPE_SUBSCRIPTION_TIERS` and `STRIPE_TOP_UP_PACKS`.

Suggested launch packages:

- Free: 500 credits/month, free-model pool only.
- Pro: $10/month for 1,000 paid credits/month.
- Top-up: $10 one-time for 1,000 paid credits when a user is low or out.

## Routes

- `GET  /` — landing, sign-in link
- `GET  /dashboard` — auth-gated balance view
- `GET  /api/auth/signin` — redirects to WorkOS AuthKit
- `GET  /api/auth/callback` — exchanges code → session cookie, upserts user + free grant
- `POST /api/auth/signout`
- `GET  /api/me` — `{ email, plan, balance, recentUsage }`
- `POST /api/billing/checkout` — create a Stripe Checkout session for subscription or top-up
- `POST /api/billing/portal` — create a Stripe Billing Portal session
- `POST /api/billing/webhook` — Stripe webhook receiver with idempotent credit grants

## Deploy

Push to GitHub, import in Vercel, set env vars, point `nvm.fyi` + `api.nvm.fyi` at it.

When working from a git worktree, Vercel CLI project links are not inherited because `.vercel/` is untracked. Copy or symlink `.vercel/` from the canonical checkout, or run `vercel link`, before Vercel env/deploy commands; never commit `.vercel/`.

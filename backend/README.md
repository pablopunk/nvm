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

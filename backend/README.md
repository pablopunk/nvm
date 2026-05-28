# Nevermind backend

Astro (SSR) + Vercel + Neon Postgres (Drizzle) + WorkOS AuthKit.

Serves the dashboard at `nvm.fyi` and the API the desktop app talks to.

## Setup

1. `cp .env.example .env` and fill in:
   - Neon `DATABASE_URL`
   - WorkOS `API_KEY`, `CLIENT_ID`, `REDIRECT_URI`
   - `WORKOS_COOKIE_PASSWORD` — `openssl rand -base64 32`
2. `pnpm install`
3. `pnpm db:push` to create tables on Neon
4. `pnpm dev` → http://localhost:4321

## Routes

- `GET  /` — landing, sign-in link
- `GET  /dashboard` — auth-gated balance view
- `GET  /api/auth/signin` — redirects to WorkOS AuthKit
- `GET  /api/auth/callback` — exchanges code → session cookie, upserts user + free grant
- `POST /api/auth/signout`
- `GET  /api/me` — `{ email, plan, balance, recentUsage }`

## Deploy

Push to GitHub, import in Vercel, set env vars, point `nvm.fyi` + `api.nvm.fyi` at it.

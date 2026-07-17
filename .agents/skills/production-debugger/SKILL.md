---
name: production-debugger
description: Use for Nevermind production incidents, external configuration, or deployed-flow verification — Sentry alerts, Vercel/Axiom logs, backend 5xx, auth failures, Neon/Upstash incidents, upstream AI provider failures, GitHub release failures, desktop prod logs, or any "check prod" request. This is an index skill for vendor-specific operational connectors.
---

# Production Debugger

This is the production operations router. It keeps app-specific stack knowledge here, then sends the agent to vendor-specific playbooks that act like MCP connectors: Sentry, Axiom, Vercel, Neon, WorkOS, Upstash, Stripe, GitHub, OpenRouter, and OpenCode.

Do not start by reading code. Start from the alert/log source, collect production evidence, and only inspect code after a vendor signal points to a code path.

## Capability and scope preflight

For configuration or verification work, read the relevant vendor playbook before
classifying the task as dashboard-only or owner-only. Use its safe read-only
commands to establish installed-tool support, authentication, and the intended
project, team, environment, host, or dataset. A missing local config file or
unlinked worktree does not establish that the runtime lacks access. Only then
separate safe CLI work from actions that need explicit confirmation and steps
that are genuinely dashboard or human only.

## Nevermind production stack

- App backend: Astro SSR on Vercel.
- Public hosts: `nvm.fyi`, `www.nvm.fyi`, `api.nvm.fyi`, plus Vercel deployment URLs.
- Runtime logs: Vercel drains to Axiom; Axiom is the primary log backend.
- Error tracking: Sentry for backend and desktop.
- Database: Neon serverless Postgres with Drizzle migrations.
- Auth: WorkOS AuthKit plus desktop device auth.
- Billing: Stripe Checkout, Billing Portal, subscriptions, top-ups, and webhooks.
- Rate limits/cache: Upstash Redis REST / Vercel KV-compatible env.
- AI upstreams: OpenCode Zen by default, OpenRouter as alternate provider.
- Releases: GitHub Actions + GitHub Releases for desktop artifacts.
- Desktop support: packaged app logs to `nevermind.log`; use this only for client-specific incidents.

## Vendor playbooks

- `./sentry.md` — fetch Sentry issue/event details, tags, release, request context, and pivot keys.
- `./axiom.md` — query Axiom datasets for Vercel runtime logs by time, route, request id, deployment, error text.
- `./vercel.md` — deployment/build/domain/env/redeploy checks and Vercel API fallback.
- `./neon.md` — Neon project/branch/connection/operation and safe database incident checks.
- `./workos.md` — WorkOS AuthKit, redirect URI, auth events, users/sessions, device auth evidence.
- `./upstash.md` — Upstash Redis/rate-limit health and key/prefix checks.
- `./stripe.md` — Stripe billing setup/incidents, price IDs, webhooks, live/test mode, and local forwarding.
- `./github.md` — GitHub Actions/release artifacts/signing/update failures.
- `./openrouter.md` — OpenRouter upstream model/API failures.
- `./opencode.md` — OpenCode Zen upstream model/API failures.

## Incident routing

- Sentry alert? Read `sentry.md`, then usually `axiom.md`.
- 5xx or route failure without Sentry? Read `axiom.md`, then `vercel.md` if deployment/env/domain may be involved.
- Auth/OAuth/device login failure? Read `workos.md`, then `axiom.md`.
- DB/health/migration/balance issue? Read `neon.md`, then `axiom.md`.
- Rate-limit/cache issue? Read `upstash.md`, then `axiom.md`.
- Billing/checkout/subscription/top-up/webhook issue? Read `stripe.md`, then `axiom.md` and `vercel.md` as needed.
- AI proxy/upstream issue? Read `axiom.md`, then `opencode.md` or `openrouter.md` based on active provider.
- Release/update/install issue? Read `github.md`; use Sentry/desktop logs only if the installed app is crashing.

## Minimum incident context

Capture this inline; do not keep a separate generic checklist file:

- Alert URL/id, source vendor, and time window/timezone.
- Environment and host.
- Route/transaction/status/error message.
- Deployment id, release, or git SHA if present.
- Correlation id such as `request_id`, Sentry event id, Vercel deployment id, or Axiom row timestamp.
- Affected scope: one user, many users, one route, one provider, or all prod.

## Output format

When asked to debug prod, respond with:

1. What the vendor evidence says happened.
2. Blast radius and whether it is still happening.
3. Correlated ids/log rows/events.
4. Most likely cause category.
5. Next mitigation or verification query.

Do not print secrets, cookies, auth headers, raw prompts, full payloads, or production data dumps.

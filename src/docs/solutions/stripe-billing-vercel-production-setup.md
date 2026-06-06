# Stripe billing production setup on Vercel

## Problem or symptoms

When enabling Stripe billing for the Nevermind backend, the app needs a coordinated set of Stripe Dashboard objects, Vercel production environment variables, webhook routing, and a production redeploy. Missing any one of these can make checkout, top-ups, subscription renewals, or credit grants fail silently or only after Stripe redirects back to the app.

## Context

Relevant backend paths:

- `backend/src/pages/api/billing/checkout.ts`
- `backend/src/pages/api/billing/portal.ts`
- `backend/src/pages/api/billing/webhook.ts`
- `backend/src/lib/billing.ts`
- `backend/src/lib/cost.ts`
- `backend/README.md`

The launch credit model used for this setup was:

- Free allowance: monthly free credits.
- Pro subscription: monthly paid credit grant.
- Top-up: one-time paid credit grant.
- Credits are retail units; upstream model costs are USD-denominated and converted through `CREDIT_USD` and `CREDIT_MARKUP`.

## Root cause of setup friction

Several operational details are easy to miss:

- Stripe Checkout uses **price IDs**, not product IDs.
- Stripe live-mode prices must be paired with live-mode API keys and webhooks; Stripe test cards require separate test-mode prices, keys, and webhook secret.
- Vercel env changes do not affect an existing deployment until production is redeployed.
- `vercel env ls production` only proves a variable name exists; encrypted values cannot confirm whitespace or the exact pasted secret.
- The public apex host may redirect. In this case, `nvm.fyi` redirected to `www.nvm.fyi`, while `api.nvm.fyi` directly served the backend API.
- Git worktrees do not inherit ignored local files such as `backend/.env` or `.vercel/` from the canonical checkout.

## Fix / working procedure

1. In Stripe, create products and prices for the subscription and top-up. Copy the `price_...` IDs from each price row.
2. Create a Stripe webhook/event destination for the backend API host, preferring the direct API domain:

   ```text
   https://api.nvm.fyi/api/billing/webhook
   ```

3. Subscribe the endpoint to these events:

   - `checkout.session.completed`
   - `invoice.paid`
   - `payment_intent.succeeded`
   - `customer.subscription.updated`
   - `customer.subscription.deleted`

4. Set Vercel production env vars:

   - `STRIPE_SECRET_KEY`
   - `STRIPE_WEBHOOK_SECRET`
   - `STRIPE_SUBSCRIPTION_TIERS` with price-id-to-credit grant mapping
   - `STRIPE_TOP_UP_PACKS` with price-id-to-credit grant mapping
   - `CREDIT_USD`
   - `CREDIT_MARKUP`
   - `MONTHLY_FREE_CREDITS`

5. Redeploy production after env changes. If the global Vercel CLI is too old, use:

   ```sh
   mise exec -- pnpm dlx vercel@latest deploy --prod --yes
   ```

6. Verify deployment health and webhook reachability without printing secrets:

   ```sh
   curl -sS -i https://api.nvm.fyi/api/health | head -40
   curl -sS -i -X POST https://api.nvm.fyi/api/billing/webhook \
     -H 'content-type: application/json' --data '{}' | head -30
   ```

   The unsigned webhook probe should reject with an invalid webhook signature/payload response; that confirms the route is live and signature validation is active.

## Worktree env handling

When doing backend/Vercel work from a git worktree, remember that ignored local configuration does not come along with the worktree:

- Use the canonical checkout as the source of truth for local `backend/.env` and `.vercel/` linkage when available.
- Copy or symlink `.vercel/`, or run `vercel link`, before Vercel env/deploy commands.
- Copy `backend/.env` only into ignored local files, never into tracked docs or committed files.
- Prefer restrictive permissions when pulling production env for debugging, for example `umask 077` before `vercel env pull`.
- Never print, commit, or persist raw secrets in documentation, logs, or solution notes.

## Verification from this setup

- Vercel production env names were present for Stripe and credit configuration.
- Production deploy completed successfully and aliased to `api.nvm.fyi`.
- `https://api.nvm.fyi/api/health` returned healthy DB and upstream status.
- An unsigned `POST` to `/api/billing/webhook` returned the expected invalid-signature error.

## Notes for future searches

Keywords: Stripe, billing, Checkout, Billing Portal, webhook, webhooks, event destination, `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_SUBSCRIPTION_TIERS`, `STRIPE_TOP_UP_PACKS`, `CREDIT_USD`, `CREDIT_MARKUP`, `MONTHLY_FREE_CREDITS`, `price_`, product ID vs price ID, `checkout.session.completed`, `invoice.paid`, `payment_intent.succeeded`, `customer.subscription.updated`, `customer.subscription.deleted`, Vercel env, Vercel deploy, outdated Vercel CLI, worktree env, `.vercel`, `backend/.env`, `api.nvm.fyi`, `nvm.fyi` redirect.

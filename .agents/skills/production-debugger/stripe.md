# Stripe Connector

Use this playbook for Nevermind billing setup and incidents: Checkout, Billing Portal, webhooks, subscriptions, top-ups, credit grants, price IDs, live/test mode, and local webhook forwarding.

## Safety rules

- Never print, commit, or document raw Stripe secrets (`sk_...`, `rk_...`, `whsec_...`).
- If a live secret is pasted into chat, terminal output, process args, or logs, finish the immediate task only if needed, then rotate it in Stripe and update Vercel/local envs.
- Do not pass Stripe API keys as command-line args; they appear in process listings. Prefer environment variables such as `STRIPE_API_KEY` or local `.env` files.
- Keep live and test resources separate. Never mix live price IDs with test keys, or test price IDs with live keys.

## Nevermind billing envs

Backend billing depends on these envs:

- `STRIPE_SECRET_KEY` — live or test secret key for the current environment.
- `STRIPE_WEBHOOK_SECRET` — signing secret for the exact webhook endpoint/listener.
- `STRIPE_SUBSCRIPTION_TIERS` — JSON mapping Stripe subscription price IDs to plan tier and paid credit grants.
- `STRIPE_TOP_UP_PACKS` — JSON mapping one-time top-up price IDs to paid credit grants.
- `CREDIT_USD`, `CREDIT_MARKUP`, `MONTHLY_FREE_CREDITS` — credit accounting configuration.

Relevant backend routes:

- `POST /api/billing/checkout`
- `POST /api/billing/portal`
- `POST /api/billing/webhook`

## Price IDs vs product IDs

The backend expects Stripe **price IDs** (`price_...`), not product IDs (`prod_...`). In Stripe Dashboard, copy the ID from the price row for each product.

## Required webhook events

Subscribe Stripe webhooks/event destinations to:

- `checkout.session.completed`
- `invoice.paid`
- `payment_intent.succeeded`
- `customer.subscription.updated`
- `customer.subscription.deleted`

For hosted environments, prefer the direct API host:

```text
https://api.nvm.fyi/api/billing/webhook
```

Avoid redirecting hosts for webhooks unless Stripe delivery has been verified end-to-end.

## Localhost test-mode workflow

Use Stripe test mode for localhost. The Stripe Dashboard webhook URL cannot be plain localhost; use Stripe CLI forwarding instead. If the CLI is missing, install it with Homebrew (`brew install stripe/stripe-cli/stripe`) or the official Stripe install flow, then verify with `stripe --version`.

1. Put test-mode values in ignored local env files (`backend/.env` in the worktree and, when useful, the canonical checkout):

   - `STRIPE_SECRET_KEY=sk_test_...`
   - `STRIPE_SUBSCRIPTION_TIERS=[{"priceId":"price_test_subscription","tier":"pro","credits":1000}]`
   - `STRIPE_TOP_UP_PACKS=[{"priceId":"price_test_topup","credits":1000}]`

2. Start Stripe forwarding without putting the key in process args. Prefer `STRIPE_API_KEY` from the sourced env over `stripe listen --api-key ...`, because command-line args can appear in process listings:

   ```sh
   set -a
   source backend/.env
   set +a
   STRIPE_API_KEY="$STRIPE_SECRET_KEY" stripe listen --forward-to localhost:4321/api/billing/webhook
   ```

3. Copy the generated `whsec_...` from the listener into `STRIPE_WEBHOOK_SECRET` in the same local env file(s), then start/restart the backend so Astro reads the updated secret. Do not paste the `whsec_...` into docs, logs, or committed files.

4. Run the backend locally and test Checkout with Stripe test card `4242 4242 4242 4242`.

## Worktree env handling

Git worktrees do not inherit ignored local files from the canonical checkout.

- Treat the canonical checkout as the source of truth for local `backend/.env` and `.vercel/` linkage when available.
- Copy or symlink `.vercel/`, or run `vercel link`, before Vercel env/deploy commands.
- Mirror local Stripe test env values into both the active worktree and canonical checkout only when the user asks.
- Never commit `.env`, `.vercel/`, pulled production env files, or raw secret values.

## Production / preview workflow

- Production Vercel should use live Stripe keys, live price IDs, live webhook secret, and a production redeploy after env edits.
- Preview/staging should use test Stripe keys, test price IDs, and a webhook pointing at a public Vercel Preview URL.
- Env changes require redeploys; `vercel env ls` only proves variable names exist, not values.
- If global Vercel CLI is outdated, use the Vercel playbook fallback: `mise exec -- pnpm dlx vercel@latest ...`.

## Verification

Useful checks that do not expose secrets:

```sh
curl -sS -i https://api.nvm.fyi/api/health | head -40
curl -sS -i -X POST https://api.nvm.fyi/api/billing/webhook \
  -H 'content-type: application/json' --data '{}' | head -30
```

An unsigned webhook probe should return an invalid-signature/payload error, confirming the route is live and signature validation is enabled.

For actual webhook validation, use the Stripe Dashboard event delivery log or Stripe CLI listener output, then correlate with Axiom/Vercel logs and backend audit/credit ledger rows.

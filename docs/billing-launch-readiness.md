# Billing launch readiness

Code readiness is not production launch approval. Do not deploy, run a
production migration, change Stripe, or make a payment as part of code review.

## Before enabling commercial Checkout

- The owner approves an immutable price-term snapshot for every configured
  Checkout Price and every Price selectable through the Billing Portal. Approval
  must use the admin read-only Price refresh and exact-term approval flow; it
  must not create or modify a Stripe Price. Configure
  `BILLING_PRICE_APPROVER_WORKOS_USER_ID` to the owner's exact WorkOS user ID;
  approval is unavailable when it is unset and other admins cannot approve.
- The owner verifies the existing Stripe webhook event subscriptions using
  read-only inspection. Do not change the endpoint or its subscriptions as part
  of this checklist.
- Preview migration and deployment validation pass for every pending migration.
  Production migration and deployment run automatically after successful CI on
  a `main` push; the owner must explicitly approve that workflow consequence
  before merge.
- Recheck Vercel's then-current plan terms before the first commercial payment.
  Keep Vercel Hobby while the beta remains free-only; upgrade or move hosting if
  commercial use requires it.

## Risks and evidence still required

- The aggregate free-provider-cost cap is intentionally unset. Until the owner
  explicitly configures a cap, aggregate free-provider subsidy remains
  unbounded; in-flight provider requests may still overrun any admission cap.
  Do not describe global subsidy as bounded or publicly budget-protected.
- Verify the deployed WorkOS callback is exactly
  `https://www.nvm.fyi/api/auth/callback` and complete a real signup journey
  before calling open signup production-ready. A sign-in redirect alone is not
  proof of successful signup.
- Inspect Stripe history, open or unpaid Checkout Sessions, subscriptions,
  configured and portal-selectable Prices, and webhook-event coverage
  read-only. Until that evidence is reviewed, do not infer that production
  Checkout is disabled, that historical coverage is complete, or that all
  purchases have verified terms.
- Inventory every existing order pinned to terms without owner approval before
  deployment. A new owner-approved Price version does not retroactively approve
  those orders; unresolved sessions may not resume or retry, and paid events
  may require explicit reconciliation instead of automatic credit fulfillment.
  If a legacy retry's PaymentIntent is confirmed canceled, reconciliation closes
  that retry without creating a replacement Checkout; the user must start a new
  purchase after current Price terms are approved.
- Do not use Stripe writes, payments, refunds, historical production backfills,
  production migrations, or deployments as validation steps.

## Handoff boundary

Record repository/CI evidence separately from Preview, production, and
owner-only evidence. A green code check does not satisfy the Price approval,
webhook verification, deployed signup, financial-history, or production
workflow gates above.

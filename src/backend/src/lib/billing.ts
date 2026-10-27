import { and, eq, isNull } from 'drizzle-orm';
import Stripe from 'stripe';
import { parsePublicOrigin } from '../../../app/shared/public-origin';
import { db } from '../db/client';
import { creditLedger, stripeEvents, subscriptions, users } from '../db/schema';
import { productionWebOrigin } from './auth-config';
import { env } from './env';
import { log } from './log';

/**
 * Billing model: env maps Stripe price IDs to credits; Stripe owns amounts.
 * A Pro subscription grants credits on checkout and on every renewal, and
 * top-ups (subscribers only) grant credits once. Webhooks are idempotent by
 * Stripe event ID and by ledger (user, reason, ref) keys.
 */
export type BillingKind = 'subscription' | 'top_up';

type SubscriptionTier = {
  kind: 'subscription';
  priceId: string;
  tier: string;
  credits: number;
};

type TopUpPack = {
  kind: 'top_up';
  priceId: string;
  credits: number;
};

export type BillingCatalogItem = SubscriptionTier | TopUpPack;

type BillingCatalog = {
  subscriptions: SubscriptionTier[];
  topUps: TopUpPack[];
};

export type StripeLike = Pick<
  Stripe,
  | 'checkout'
  | 'billingPortal'
  | 'customers'
  | 'subscriptions'
  | 'prices'
  | 'webhooks'
>;

type User = typeof users.$inferSelect;
type BillingDb = Parameters<Parameters<typeof db.transaction>[0]>[0];

const STRIPE_REQUEST_TIMEOUT_MS = 20_000;
const CHECKOUT_IDEMPOTENCY_WINDOW_MS = 10 * 60 * 1000;
const LEDGER_REASON = {
  subscription: 'stripe_checkout_subscription',
  renewal: 'stripe_subscription_renewal',
  topUp: 'stripe_top_up',
} as const;

let stripeOverride: StripeLike | null = null;

export class BillingConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'BillingConfigError';
  }
}

export class BillingEligibilityError extends Error {
  constructor(
    public type: 'top_up_requires_subscription' | 'already_subscribed',
    message: string,
  ) {
    super(message);
    this.name = 'BillingEligibilityError';
  }
}

export class BillingRequestError extends Error {
  constructor(
    public type: 'unknown_price',
    message: string,
  ) {
    super(message);
    this.name = 'BillingRequestError';
  }
}

export function setStripeForTests(next: StripeLike | null) {
  stripeOverride = next;
}

export function stripeClient(): StripeLike {
  if (stripeOverride) return stripeOverride;
  const secretKey = env('STRIPE_SECRET_KEY');
  if (!secretKey)
    throw new BillingConfigError('STRIPE_SECRET_KEY is not configured');
  return new Stripe(secretKey);
}

export function formatBillingPrice(price: {
  amount: number;
  currency: string;
}): string | null {
  try {
    const formatter = new Intl.NumberFormat('en', {
      style: 'currency',
      currency: price.currency.toUpperCase(),
    });
    const fractionDigits =
      formatter.resolvedOptions().maximumFractionDigits ?? 2;
    return formatter.format(price.amount / 10 ** fractionDigits);
  } catch {
    return null;
  }
}

function parsePositiveInt(value: unknown): number {
  const parsed = Number.parseInt(String(value ?? ''), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}

/** Accepts arrays or objects keyed by price ID; extra fields are ignored. */
function parseCatalogEnv(name: string): Array<Record<string, unknown>> {
  const value = env(name);
  if (!value) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed)
      ? parsed
      : Object.entries(parsed).map(([priceId, row]) => ({
          ...(row as object),
          priceId,
        }));
  } catch {
    throw new BillingConfigError(`${name} must be valid JSON`);
  }
}

export function billingCatalog(): BillingCatalog {
  const subscriptionTiers: SubscriptionTier[] = [];
  for (const raw of parseCatalogEnv('STRIPE_SUBSCRIPTION_TIERS')) {
    const priceId = String(raw.priceId ?? raw.price_id ?? '').trim();
    const credits = parsePositiveInt(raw.credits);
    const tier = String(raw.tier ?? raw.plan ?? 'pro').trim() || 'pro';
    if (priceId && credits)
      subscriptionTiers.push({ kind: 'subscription', priceId, tier, credits });
  }
  const topUps: TopUpPack[] = [];
  for (const raw of parseCatalogEnv('STRIPE_TOP_UP_PACKS')) {
    const priceId = String(raw.priceId ?? raw.price_id ?? '').trim();
    const credits = parsePositiveInt(raw.credits);
    if (priceId && credits) topUps.push({ kind: 'top_up', priceId, credits });
  }
  return { subscriptions: subscriptionTiers, topUps };
}

export function findCatalogItem(input: {
  kind: BillingKind;
  priceId?: string;
  tier?: string;
}): BillingCatalogItem | null {
  const catalog = billingCatalog();
  const items: BillingCatalogItem[] =
    input.kind === 'subscription' ? catalog.subscriptions : catalog.topUps;
  if (input.priceId)
    return items.find((item) => item.priceId === input.priceId) ?? null;
  if (input.tier && input.kind === 'subscription')
    return catalog.subscriptions.find((item) => item.tier === input.tier) ?? null;
  return items[0] ?? null;
}

function publicUrl(path: string): string {
  try {
    const configured =
      env('VERCEL_ENV') === 'preview'
        ? env('VERCEL_URL')
          ? `https://${env('VERCEL_URL')}`
          : ''
        : env('VERCEL_ENV') === 'production' || env('NODE_ENV') === 'production'
          ? productionWebOrigin()
          : env('PUBLIC_DASHBOARD_URL')?.match(
                /^https?:\/\/localhost(?::\d+)?\/?$/,
              )
            ? env('PUBLIC_DASHBOARD_URL')!
            : productionWebOrigin();
    const policy =
      env('VERCEL_ENV') === 'preview'
        ? 'preview'
        : configured.startsWith('http://localhost')
          ? 'local'
          : 'production_web';
    const base =
      policy === 'preview'
        ? parsePublicOrigin(configured, 'preview', configured)
        : parsePublicOrigin(configured, policy);
    return new URL(path, `${base}/`).toString();
  } catch {
    throw new BillingConfigError('Public web origin configuration is invalid');
  }
}

export function rejectCrossOriginBillingPost(
  request: Request,
): Response | null {
  const origin = request.headers.get('origin');
  if (!origin || origin === new URL(request.url).origin) return null;
  return Response.json(
    {
      error: {
        type: 'forbidden',
        message: 'Cross-origin billing request rejected',
      },
    },
    { status: 403 },
  );
}

export function isActiveBillingSubscriptionStatus(status: string): boolean {
  return status === 'active' || status === 'trialing' || status === 'past_due';
}

async function userHasActiveSubscription(userId: string): Promise<boolean> {
  const [row] = await db
    .select({ status: subscriptions.status })
    .from(subscriptions)
    .where(eq(subscriptions.userId, userId))
    .limit(1);
  return row ? isActiveBillingSubscriptionStatus(row.status) : false;
}

/** The idempotency key makes concurrent and retried creates return one customer. */
export async function getOrCreateStripeCustomer(user: User): Promise<string> {
  if (user.stripeCustomerId) return user.stripeCustomerId;
  const customer = await stripeClient().customers.create(
    { email: user.email, metadata: { user_id: user.id } },
    {
      idempotencyKey: `billing-customer-${user.id}`,
      timeout: STRIPE_REQUEST_TIMEOUT_MS,
    },
  );
  await db
    .update(users)
    .set({ stripeCustomerId: customer.id })
    .where(and(eq(users.id, user.id), isNull(users.stripeCustomerId)));
  const [current] = await db
    .select({ stripeCustomerId: users.stripeCustomerId })
    .from(users)
    .where(eq(users.id, user.id))
    .limit(1);
  return current?.stripeCustomerId ?? customer.id;
}

export async function createBillingCheckout(input: {
  user: User;
  kind: BillingKind;
  priceId?: string;
  tier?: string;
}): Promise<{ url: string }> {
  const item = findCatalogItem(input);
  if (!item) {
    if (input.priceId)
      throw new BillingRequestError('unknown_price', 'Unknown Stripe billing price.');
    throw new BillingConfigError(`No Stripe ${input.kind} price is configured`);
  }
  const subscribed = await userHasActiveSubscription(input.user.id);
  if (item.kind === 'subscription' && subscribed)
    throw new BillingEligibilityError(
      'already_subscribed',
      'You already have an active Pro subscription. Manage it from billing settings.',
    );
  if (item.kind === 'top_up' && !subscribed)
    throw new BillingEligibilityError(
      'top_up_requires_subscription',
      'Top-ups are available after subscribing to Pro.',
    );

  const customer = await getOrCreateStripeCustomer(input.user);
  const metadata: Stripe.MetadataParam = {
    billing_kind: item.kind,
    user_id: input.user.id,
    price_id: item.priceId,
    credits: String(item.credits),
    ...(item.kind === 'subscription' ? { tier: item.tier } : {}),
  };
  const window = Math.floor(Date.now() / CHECKOUT_IDEMPOTENCY_WINDOW_MS);
  const session = await stripeClient().checkout.sessions.create(
    {
      mode: item.kind === 'subscription' ? 'subscription' : 'payment',
      customer,
      client_reference_id: input.user.id,
      line_items: [{ price: item.priceId, quantity: 1 }],
      success_url: publicUrl('/profile?billing=success'),
      cancel_url: publicUrl('/profile?billing=canceled'),
      metadata,
      ...(item.kind === 'subscription'
        ? { subscription_data: { metadata } }
        : { payment_intent_data: { metadata } }),
    },
    {
      idempotencyKey: `checkout:${input.user.id}:${item.priceId}:${window}`,
      timeout: STRIPE_REQUEST_TIMEOUT_MS,
    },
  );
  if (!session.url) throw new Error('Stripe Checkout returned no URL');
  return { url: session.url };
}

export async function createBillingPortal(input: { user: User }) {
  if (!input.user.stripeCustomerId)
    throw new BillingConfigError('No Stripe customer exists for this user');
  return stripeClient().billingPortal.sessions.create({
    customer: input.user.stripeCustomerId,
    return_url: publicUrl('/profile'),
  });
}

export function constructStripeWebhookEvent(
  payload: string,
  signature: string | null,
): Stripe.Event {
  const webhookSecret = env('STRIPE_WEBHOOK_SECRET');
  if (!webhookSecret)
    throw new BillingConfigError('STRIPE_WEBHOOK_SECRET is not configured');
  if (!signature) throw new Error('Missing stripe-signature header');
  return stripeClient().webhooks.constructEvent(payload, signature, webhookSecret);
}

function objectId(value: unknown): string | null {
  if (typeof value === 'string') return value;
  if (value && typeof value === 'object' && typeof (value as { id?: unknown }).id === 'string')
    return (value as { id: string }).id;
  return null;
}

/** Works with both the pre-Basil and current invoice shapes. */
function invoiceSubscriptionId(invoice: Stripe.Invoice): string | null {
  return (
    objectId((invoice as any).subscription) ??
    objectId((invoice as any).parent?.subscription_details?.subscription)
  );
}

function subscriptionPriceId(subscription: Stripe.Subscription): string | null {
  return subscription.items.data[0]?.price?.id ?? null;
}

function subscriptionPeriodEnd(subscription: Stripe.Subscription): Date {
  const seconds =
    (subscription.items.data[0] as any)?.current_period_end ??
    (subscription as any).current_period_end;
  return new Date(Number(seconds) * 1000);
}

/** Unknown prices fail the webhook so Stripe retries after the env is fixed. */
function subscriptionTierFor(subscription: Stripe.Subscription): SubscriptionTier {
  const priceId = subscriptionPriceId(subscription);
  const tier = billingCatalog().subscriptions.find((item) => item.priceId === priceId);
  if (!tier)
    throw new BillingConfigError(`Subscription price ${priceId} is not in STRIPE_SUBSCRIPTION_TIERS`);
  return tier;
}

async function grantPaidCredits(
  tx: BillingDb,
  input: { userId: string; credits: number; reason: string; refId: string },
) {
  await tx
    .insert(creditLedger)
    .values({
      userId: input.userId,
      delta: input.credits,
      kind: 'paid',
      reason: input.reason,
      refId: input.refId,
    })
    .onConflictDoNothing({
      target: [creditLedger.userId, creditLedger.reason, creditLedger.refId],
    });
}

async function userIdForStripeSubscription(
  tx: BillingDb,
  subscription: Stripe.Subscription,
): Promise<string | null> {
  const [bySubscription] = await tx
    .select({ userId: subscriptions.userId })
    .from(subscriptions)
    .where(eq(subscriptions.stripeSubId, subscription.id))
    .limit(1);
  if (bySubscription) return bySubscription.userId;
  if (subscription.metadata?.user_id) return subscription.metadata.user_id;
  const customerId = objectId(subscription.customer);
  if (!customerId) return null;
  const [byCustomer] = await tx
    .select({ id: users.id })
    .from(users)
    .where(eq(users.stripeCustomerId, customerId))
    .limit(1);
  return byCustomer?.id ?? null;
}

/**
 * Mirrors Stripe's current subscription state. An inactive subscription never
 * replaces a different active one, so a late delete of an old subscription
 * cannot downgrade a user who already resubscribed.
 */
async function syncSubscription(
  tx: BillingDb,
  userId: string,
  subscription: Stripe.Subscription,
) {
  const active = isActiveBillingSubscriptionStatus(subscription.status);
  const [existing] = await tx
    .select()
    .from(subscriptions)
    .where(eq(subscriptions.userId, userId))
    .limit(1);
  if (
    !active &&
    existing &&
    existing.stripeSubId !== subscription.id &&
    isActiveBillingSubscriptionStatus(existing.status)
  )
    return;
  const tier = active ? subscriptionTierFor(subscription).tier : (existing?.tier ?? 'pro');
  const values = {
    stripeSubId: subscription.id,
    tier,
    status: subscription.status,
    currentPeriodEnd: subscriptionPeriodEnd(subscription),
  };
  await tx
    .insert(subscriptions)
    .values({ userId, ...values })
    .onConflictDoUpdate({ target: subscriptions.userId, set: values });
  await tx
    .update(users)
    .set({ plan: active ? tier : 'free' })
    .where(eq(users.id, userId));
}

async function handleCheckoutCompleted(
  tx: BillingDb,
  session: Stripe.Checkout.Session,
  subscription: Stripe.Subscription | null,
) {
  if (session.payment_status !== 'paid' && session.payment_status !== 'no_payment_required')
    return;
  const userId = session.metadata?.user_id ?? session.client_reference_id;
  if (!userId) throw new Error(`Checkout ${session.id} has no user`);
  const customerId = objectId(session.customer);
  if (customerId)
    await tx
      .update(users)
      .set({ stripeCustomerId: customerId })
      .where(and(eq(users.id, userId), isNull(users.stripeCustomerId)));

  if (session.mode === 'subscription') {
    if (!subscription) throw new Error(`Checkout ${session.id} has no subscription`);
    const tier = subscriptionTierFor(subscription);
    await grantPaidCredits(tx, {
      userId,
      credits: tier.credits,
      reason: LEDGER_REASON.subscription,
      refId: session.id,
    });
    await syncSubscription(tx, userId, subscription);
    return;
  }

  const priceId = session.metadata?.price_id;
  const pack = billingCatalog().topUps.find((item) => item.priceId === priceId);
  if (!pack)
    throw new BillingConfigError(`Top-up price ${priceId} is not in STRIPE_TOP_UP_PACKS`);
  await grantPaidCredits(tx, {
    userId,
    credits: pack.credits,
    reason: LEDGER_REASON.topUp,
    refId: objectId(session.payment_intent) ?? session.id,
  });
}

/** The first invoice is granted by checkout; only renewals grant here. */
async function handleInvoicePaid(
  tx: BillingDb,
  invoice: Stripe.Invoice,
  subscription: Stripe.Subscription | null,
) {
  if (invoice.billing_reason !== 'subscription_cycle' || !subscription) return;
  const userId = await userIdForStripeSubscription(tx, subscription);
  if (!userId) throw new Error(`Invoice ${invoice.id} has no matching user`);
  await grantPaidCredits(tx, {
    userId,
    credits: subscriptionTierFor(subscription).credits,
    reason: LEDGER_REASON.renewal,
    refId: invoice.id!,
  });
  await syncSubscription(tx, userId, subscription);
}

async function handleSubscriptionChanged(
  tx: BillingDb,
  subscription: Stripe.Subscription,
) {
  const userId = await userIdForStripeSubscription(tx, subscription);
  if (!userId) {
    log.warn('stripe_subscription_unmapped', { stripe_sub_id: subscription.id });
    return;
  }
  await syncSubscription(tx, userId, subscription);
}

/** Stripe reads happen before the transaction so no DB lock waits on Stripe. */
async function retrieveEventSubscription(
  event: Stripe.Event,
): Promise<Stripe.Subscription | null> {
  const object = event.data.object as unknown as Record<string, unknown>;
  const subscriptionId =
    event.type === 'checkout.session.completed' ||
    event.type === 'checkout.session.async_payment_succeeded'
      ? objectId(object.subscription)
      : event.type === 'invoice.paid'
        ? invoiceSubscriptionId(object as unknown as Stripe.Invoice)
        : event.type === 'customer.subscription.updated' ||
            event.type === 'customer.subscription.deleted'
          ? objectId(object.id)
          : null;
  return subscriptionId
    ? stripeClient().subscriptions.retrieve(subscriptionId)
    : null;
}

/**
 * Processes each Stripe event once. Any failure rolls back the event row and
 * returns an error, so Stripe redelivers it.
 */
export async function processStripeEvent(
  event: Stripe.Event,
): Promise<{ processed: boolean }> {
  const [seen] = await db
    .select({ eventId: stripeEvents.eventId })
    .from(stripeEvents)
    .where(eq(stripeEvents.eventId, event.id))
    .limit(1);
  if (seen) return { processed: false };
  const subscription = await retrieveEventSubscription(event);
  return db.transaction(async (tx) => {
    const inserted = await tx
      .insert(stripeEvents)
      .values({
        eventId: event.id,
        type: event.type,
        apiVersion: event.api_version ?? null,
        payload: event as any,
        state: 'processed',
        processedAt: new Date(),
      })
      .onConflictDoNothing({ target: stripeEvents.eventId })
      .returning({ eventId: stripeEvents.eventId });
    if (inserted.length === 0) return { processed: false };
    switch (event.type) {
      case 'checkout.session.completed':
      case 'checkout.session.async_payment_succeeded':
        await handleCheckoutCompleted(tx, event.data.object, subscription);
        break;
      case 'invoice.paid':
        await handleInvoicePaid(tx, event.data.object, subscription);
        break;
      case 'customer.subscription.updated':
      case 'customer.subscription.deleted':
        if (subscription) await handleSubscriptionChanged(tx, subscription);
        break;
    }
    log.info('stripe_event_processed', {
      stripe_event_id: event.id,
      stripe_event_type: event.type,
    });
    return { processed: true };
  });
}

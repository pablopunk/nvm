import Stripe from 'stripe';
import { and, eq, isNull, or, sql } from 'drizzle-orm';
import { createHash, randomUUID } from 'node:crypto';
import { db } from '../db/client';
import {
  billingDuplicateChargeConflicts,
  billingOrders,
  billingPriceTerms,
  billingRetryOperations,
  billingSubscriptionPriceVersions,
  creditLedger,
  stripeCustomerAttempts,
  stripeEvents,
  subscriptions,
  users,
} from '../db/schema';
import { env } from './env';
import { log } from './log';
import { parsePublicOrigin } from '../../../app/shared/public-origin';
import { productionWebOrigin } from './auth-config';

export type BillingKind = 'subscription' | 'top_up';
export type BillingInterval = 'day' | 'week' | 'month' | 'year';

export type BillingPriceDisplay = {
  amount: number;
  currency: string;
  interval?: BillingInterval;
};

type SubscriptionTier = {
  kind: 'subscription';
  priceId: string;
  tier: string;
  credits: number;
  displayPrice?: BillingPriceDisplay & { interval: BillingInterval };
};

type TopUpPack = {
  kind: 'top_up';
  priceId: string;
  credits: number;
  displayPrice?: BillingPriceDisplay;
};

export type BillingCatalogItem = SubscriptionTier | TopUpPack;

type BillingCatalog = {
  subscriptions: SubscriptionTier[];
  topUps: TopUpPack[];
};

type StripeLike = Pick<
  Stripe,
  'checkout' | 'billingPortal' | 'customers' | 'subscriptions' | 'prices' | 'paymentIntents' | 'invoices' | 'webhooks'
>;

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
    public type: 'unknown_price' | 'idempotency_conflict' | 'checkout_in_flight',
    message: string,
  ) {
    super(message);
    this.name = 'BillingRequestError';
  }
}

export class BillingPendingError extends Error {
  constructor(
    public status: 'processing' | 'retry_required' | 'completed' | 'expired' | 'failed' | 'reconciliation_required',
    public attemptKey?: string,
  ) {
    super(status);
    this.name = 'BillingPendingError';
  }
}

export function setStripeForTests(next: StripeLike | null) {
  stripeOverride = next;
}

function stripeClient(): StripeLike {
  if (stripeOverride) return stripeOverride;
  const secretKey = env('STRIPE_SECRET_KEY');
  if (!secretKey) throw new BillingConfigError('STRIPE_SECRET_KEY is not configured');
  return new Stripe(secretKey);
}

function parsePositiveInt(value: string | undefined, fallback = 0): number {
  const parsed = Number.parseInt(value ?? '', 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function parseDisplayPrice(raw: Record<string, unknown>, requiresInterval: boolean): BillingPriceDisplay | null {
  const amount = Number(raw.amount ?? raw.unitAmount ?? raw.unit_amount);
  const currency = String(raw.currency ?? '')
    .trim()
    .toUpperCase();
  if (!Number.isSafeInteger(amount) || amount <= 0 || !/^[A-Z]{3}$/.test(currency)) return null;

  const interval = String(raw.interval ?? raw.recurringInterval ?? raw.recurring_interval ?? '')
    .trim()
    .toLowerCase();
  if (!requiresInterval) return { amount, currency };
  if (interval !== 'day' && interval !== 'week' && interval !== 'month' && interval !== 'year') return null;
  return { amount, currency, interval };
}

export function formatBillingPrice(price: BillingPriceDisplay): string | null {
  try {
    const formatter = new Intl.NumberFormat('en', {
      style: 'currency',
      currency: price.currency,
    });
    const fractionDigits = formatter.resolvedOptions().maximumFractionDigits ?? 2;
    return formatter.format(price.amount / 10 ** fractionDigits);
  } catch {
    return null;
  }
}

function parseCatalogArray<T>(name: string, normalize: (raw: any) => T | null): T[] {
  const value = env(name);
  if (!value) return [];
  try {
    const parsed = JSON.parse(value);
    const rows = Array.isArray(parsed)
      ? parsed
      : Object.entries(parsed).map(([key, val]) => ({
          ...(val as object),
          priceId: key,
        }));
    return rows.map(normalize).filter((row): row is T => Boolean(row));
  } catch {
    throw new BillingConfigError(`${name} must be valid JSON`);
  }
}

export function billingCatalog(): BillingCatalog {
  const subscriptionsFromJson = parseCatalogArray<SubscriptionTier>('STRIPE_SUBSCRIPTION_TIERS', (raw) => {
    const priceId = String(raw.priceId ?? raw.price_id ?? '').trim();
    const tier = String(raw.tier ?? raw.plan ?? '').trim();
    const credits = parsePositiveInt(String(raw.credits ?? ''), 0);
    const displayPrice = parseDisplayPrice(raw, true);
    return priceId && tier && credits
      ? {
          kind: 'subscription',
          priceId,
          tier,
          credits,
          ...(displayPrice?.interval
            ? {
                displayPrice: {
                  ...displayPrice,
                  interval: displayPrice.interval,
                },
              }
            : {}),
        }
      : null;
  });
  const topUpsFromJson = parseCatalogArray<TopUpPack>('STRIPE_TOP_UP_PACKS', (raw) => {
    const priceId = String(raw.priceId ?? raw.price_id ?? '').trim();
    const credits = parsePositiveInt(String(raw.credits ?? ''), 0);
    const displayPrice = parseDisplayPrice(raw, false);
    return priceId && credits
      ? {
          kind: 'top_up',
          priceId,
          credits,
          ...(displayPrice ? { displayPrice } : {}),
        }
      : null;
  });

  const fallbackSubscriptionPriceId = env('STRIPE_SUBSCRIPTION_PRICE_ID');
  const fallbackTopUpPriceId = env('STRIPE_TOP_UP_PRICE_ID');
  const fallbackSubscriptionCredits = parsePositiveInt(env('STRIPE_SUBSCRIPTION_CREDITS'), 0);
  const fallbackTopUpCredits = parsePositiveInt(env('STRIPE_TOP_UP_CREDITS'), 0);
  const fallbackSubscriptions: SubscriptionTier[] =
    fallbackSubscriptionPriceId && fallbackSubscriptionCredits > 0
      ? [
          {
            kind: 'subscription',
            priceId: fallbackSubscriptionPriceId,
            tier: env('STRIPE_SUBSCRIPTION_TIER') ?? 'pro',
            credits: fallbackSubscriptionCredits,
          },
        ]
      : [];
  const fallbackTopUps: TopUpPack[] =
    fallbackTopUpPriceId && fallbackTopUpCredits > 0
      ? [
          {
            kind: 'top_up',
            priceId: fallbackTopUpPriceId,
            credits: fallbackTopUpCredits,
          },
        ]
      : [];
  return {
    subscriptions: subscriptionsFromJson.length ? subscriptionsFromJson : fallbackSubscriptions,
    topUps: topUpsFromJson.length ? topUpsFromJson : fallbackTopUps,
  };
}

export function findCatalogItem(input: { kind: BillingKind; priceId?: string; tier?: string }): BillingCatalogItem | null {
  const catalog = billingCatalog();
  if (input.kind === 'subscription') {
    if (input.priceId) return catalog.subscriptions.find((item) => item.priceId === input.priceId) ?? null;
    if (input.tier) return catalog.subscriptions.find((item) => item.tier === input.tier) ?? null;
    return catalog.subscriptions[0] ?? null;
  }
  if (input.priceId) return catalog.topUps.find((item) => item.priceId === input.priceId) ?? null;
  return catalog.topUps[0] ?? null;
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
          : env('PUBLIC_DASHBOARD_URL')?.match(/^https?:\/\/localhost(?::\d+)?\/?$/)
            ? env('PUBLIC_DASHBOARD_URL')!
            : productionWebOrigin();
    const policy = env('VERCEL_ENV') === 'preview' ? 'preview' : configured.startsWith('http://localhost') ? 'local' : 'production_web';
    const base = policy === 'preview' ? parsePublicOrigin(configured, 'preview', configured) : parsePublicOrigin(configured, policy);
    return new URL(path, `${base}/`).toString();
  } catch {
    throw new BillingConfigError('Public web origin configuration is invalid');
  }
}

export function rejectCrossOriginBillingPost(request: Request): Response | null {
  const origin = request.headers.get('origin');
  if (!origin) return null;
  if (origin === new URL(request.url).origin) return null;
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

const STRIPE_REQUEST_TIMEOUT_MS = 20_000;
const BILLING_LEASE_MS = 60_000;

export async function getOrCreateStripeCustomer(user: typeof users.$inferSelect): Promise<string> {
  if (user.stripeCustomerId) return user.stripeCustomerId;
  const stripe = stripeClient();
  const customerPayload = { email: user.email, metadata: { user_id: user.id } };
  const idempotencyKey = `billing-customer-${user.id}`;
  const claimed = await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${user.id} FOR UPDATE`);
    const [currentUser] = await tx.select({ stripeCustomerId: users.stripeCustomerId }).from(users).where(eq(users.id, user.id)).limit(1);
    if (currentUser?.stripeCustomerId) return { kind: 'linked' as const, id: currentUser.stripeCustomerId };
    const [attempt] = await tx.select().from(stripeCustomerAttempts).where(eq(stripeCustomerAttempts.userId, user.id)).limit(1);
    if (!attempt) {
      await tx.insert(stripeCustomerAttempts).values({
        userId: user.id,
        idempotencyKey,
        payload: customerPayload,
        status: 'creating',
        leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
      });
      return {
        kind: 'owner' as const,
        key: idempotencyKey,
        payload: customerPayload,
      };
    }
    if (attempt.status === 'linked' && attempt.customerId) return { kind: 'linked' as const, id: attempt.customerId };
    if (attempt.status === 'creating' && attempt.leaseExpiresAt && attempt.leaseExpiresAt > new Date()) return { kind: 'pending' as const };
    if (Date.now() - attempt.createdAt.getTime() >= 23 * 60 * 60 * 1000 || attempt.status === 'reconciliation_required') {
      await tx
        .update(stripeCustomerAttempts)
        .set({
          status: 'reconciliation_required',
          leaseExpiresAt: null,
          updatedAt: new Date(),
        })
        .where(eq(stripeCustomerAttempts.userId, user.id));
      return {
        kind: 'lookup' as const,
        payload: attempt.payload as typeof customerPayload,
      };
    }
    const storedPayload = attempt.payload as typeof customerPayload;
    await tx
      .update(stripeCustomerAttempts)
      .set({
        status: 'creating',
        leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
        updatedAt: new Date(),
      })
      .where(eq(stripeCustomerAttempts.userId, user.id));
    return {
      kind: 'owner' as const,
      key: attempt.idempotencyKey,
      payload: storedPayload,
    };
  });
  if (claimed.kind === 'linked') return claimed.id;
  if (claimed.kind === 'pending') throw new BillingPendingError('processing');
  if (claimed.kind === 'lookup') {
    const customers = await stripe.customers.list({
      email: claimed.payload.email,
      limit: 100,
    });
    const matches = customers.data.filter((candidate) => candidate.metadata.user_id === user.id);
    if (customers.has_more || matches.length !== 1) throw new BillingPendingError('reconciliation_required');
    const customer = matches[0];
    await db.transaction(async (tx) => {
      await tx.execute(sql`SELECT id FROM users WHERE id = ${user.id} FOR UPDATE`);
      await tx
        .update(users)
        .set({ stripeCustomerId: customer.id })
        .where(and(eq(users.id, user.id), isNull(users.stripeCustomerId)));
      await tx
        .update(stripeCustomerAttempts)
        .set({
          customerId: customer.id,
          status: 'linked',
          updatedAt: new Date(),
        })
        .where(eq(stripeCustomerAttempts.userId, user.id));
    });
    return customer.id;
  }
  try {
    const customer = await stripe.customers.create(claimed.payload, {
      idempotencyKey: claimed.key,
      timeout: STRIPE_REQUEST_TIMEOUT_MS,
    });
    await db.transaction(async (tx) => {
      await tx.execute(sql`SELECT id FROM users WHERE id = ${user.id} FOR UPDATE`);
      await tx
        .update(users)
        .set({ stripeCustomerId: customer.id })
        .where(and(eq(users.id, user.id), isNull(users.stripeCustomerId)));
      await tx
        .update(stripeCustomerAttempts)
        .set({
          customerId: customer.id,
          status: 'linked',
          leaseExpiresAt: null,
          updatedAt: new Date(),
        })
        .where(eq(stripeCustomerAttempts.userId, user.id));
    });
    return customer.id;
  } catch (error) {
    const definitiveFailure =
      error instanceof Stripe.errors.StripeInvalidRequestError || error instanceof Stripe.errors.StripeAuthenticationError;
    await db
      .update(stripeCustomerAttempts)
      .set({
        status: definitiveFailure ? 'failed' : 'ambiguous',
        leaseExpiresAt: null,
        lastError: String(error).slice(0, 500),
        updatedAt: new Date(),
      })
      .where(eq(stripeCustomerAttempts.userId, user.id));
    if (definitiveFailure) {
      throw new BillingPendingError('failed');
    }
    throw new BillingPendingError('processing');
  }
}

async function userHasActiveSubscription(database: BillingDb, userId: string): Promise<boolean> {
  const [subscription] = await database
    .select({ status: subscriptions.status })
    .from(subscriptions)
    .where(eq(subscriptions.userId, userId))
    .limit(1);
  return subscription ? isActiveBillingSubscriptionStatus(subscription.status) : false;
}

function checkoutIntentHash(input: { kind: BillingKind; priceId?: string; tier?: string }, resolvedPriceId: string) {
  return createHash('sha256')
    .update(
      JSON.stringify({
        kind: input.kind,
        priceId: resolvedPriceId,
        selectedPriceId: input.priceId ?? null,
        selectedTier: input.tier ?? null,
      }),
    )
    .digest('hex');
}

async function resumeStoredBillingCheckout(input: {
  userId: string;
  attemptKey: string;
  intentHash: (priceId: string) => string;
}): Promise<{ status: 'url'; url: string } | { status: 'processing' } | null> {
  const [found] = await db
    .select()
    .from(billingOrders)
    .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.attemptKey, input.attemptKey)))
    .limit(1);
  if (!found) return null;
  if (found.intentHash !== input.intentHash(found.priceId)) {
    throw new BillingRequestError('idempotency_conflict', 'This idempotency key was already used for a different purchase.');
  }
  const claim = await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${input.userId} FOR UPDATE`);
    const [order] = await tx
      .select()
      .from(billingOrders)
      .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.id, found.id)))
      .limit(1);
    if (!order) return { kind: 'processing' as const };
    if (order.intentHash !== input.intentHash(order.priceId)) {
      throw new BillingRequestError('idempotency_conflict', 'This idempotency key was already used for a different purchase.');
    }
    if (order.state === 'open' && order.sessionUrl) return { kind: 'url' as const, url: order.sessionUrl };
    if (['completed', 'expired', 'failed', 'retry_required', 'needs_reconciliation'].includes(order.state)) {
      return {
        kind: 'status' as const,
        status: (order.state === 'needs_reconciliation' ? 'reconciliation_required' : order.state) as BillingPendingError['status'],
      };
    }
    if (order.state !== 'creating' || (order.leaseExpiresAt && order.leaseExpiresAt > new Date())) {
      return { kind: 'processing' as const };
    }
    if (Date.now() - order.firstAttemptAt.getTime() > 23 * 60 * 60 * 1000) {
      await tx
        .update(billingOrders)
        .set({ leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS), updatedAt: new Date() })
        .where(and(eq(billingOrders.id, order.id), eq(billingOrders.version, order.version), eq(billingOrders.state, 'creating')));
      return { kind: 'lookup' as const, order };
    }
    const generation = order.leaseGeneration + 1;
    await tx
      .update(billingOrders)
      .set({ leaseGeneration: generation, leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS), updatedAt: new Date() })
      .where(and(eq(billingOrders.id, order.id), eq(billingOrders.version, order.version), eq(billingOrders.state, 'creating')));
    return { kind: 'create' as const, order, generation };
  });
  if (claim.kind === 'url') return { status: 'url', url: claim.url };
  if (claim.kind === 'status') throw new BillingPendingError(claim.status);
  if (claim.kind === 'processing') return { status: 'processing' };
  if (claim.kind === 'lookup') return recoverAmbiguousCheckoutCreate(claim.order);

  const stripe = stripeClient();
  try {
    const session = await stripe.checkout.sessions.create(claim.order.requestSnapshot as Stripe.Checkout.SessionCreateParams, {
      idempotencyKey: `billing-checkout-${claim.order.id}`,
      timeout: STRIPE_REQUEST_TIMEOUT_MS,
    });
    const persisted = await db.transaction(async (tx) => {
      await tx.execute(sql`SELECT id FROM users WHERE id = ${input.userId} FOR UPDATE`);
      const [current] = await tx.select().from(billingOrders).where(eq(billingOrders.id, claim.order.id)).limit(1);
      if (!current || current.state !== 'creating' || current.leaseGeneration !== claim.generation) return false;
      await tx
        .update(billingOrders)
        .set({
          sessionId: session.id,
          sessionUrl: session.url,
          paymentMethodTypes: session.payment_method_types ?? [],
          paymentIntentId: objectId(session.payment_intent),
          state: session.status === 'open' && session.url ? 'open' : 'processing',
          leaseExpiresAt: null,
          version: current.version + 1,
          updatedAt: new Date(),
        })
        .where(
          and(
            eq(billingOrders.id, current.id),
            eq(billingOrders.version, current.version),
            eq(billingOrders.leaseGeneration, claim.generation),
            eq(billingOrders.state, 'creating'),
          ),
        );
      return true;
    });
    if (session.status === 'open' && session.url && persisted) return { status: 'url', url: session.url };
    return { status: 'processing' };
  } catch (error) {
    log.warn('stripe_checkout_recovery_ambiguous', { order_id: claim.order.id, error });
    return { status: 'processing' };
  }
}

export async function createBillingCheckout(input: {
  user: typeof users.$inferSelect;
  kind: BillingKind;
  priceId?: string;
  tier?: string;
  attemptKey?: string;
  legacyCaller?: boolean;
}) {
  const attemptKey = input.attemptKey ?? randomUUID();
  if (input.attemptKey) {
    const recovered = await resumeStoredBillingCheckout({
      userId: input.user.id,
      attemptKey,
      intentHash: (priceId) => checkoutIntentHash(input, priceId),
    });
    if (recovered) return recovered;
  }
  const item = findCatalogItem(input);
  if (!item) {
    if (input.priceId || input.tier) throw new BillingRequestError('unknown_price', 'Unknown Stripe billing price.');
    throw new BillingConfigError(`No Stripe ${input.kind} price is configured`);
  }
  // Fail closed until an owner-approved, versioned price snapshot exists.
  // Retrieve Stripe facts before creating a Customer or entering any DB tx.
  const termVersions = await db.select().from(billingPriceTerms).where(eq(billingPriceTerms.priceId, item.priceId));
  const terms = termVersions
    .filter(
      (candidate) =>
        candidate.kind === item.kind &&
        candidate.credits === item.credits &&
        candidate.tier === (item.kind === 'subscription' ? item.tier : null) &&
        candidate.quantity === 1 &&
        Boolean(item.displayPrice) &&
        candidate.unitAmount === item.displayPrice?.amount &&
        candidate.currency === item.displayPrice?.currency.toLowerCase() &&
        candidate.interval === (item.kind === 'subscription' ? item.displayPrice?.interval : null) &&
        candidate.intervalCount === (item.kind === 'subscription' ? 1 : null),
    )
    .sort((left, right) => right.version - left.version)[0];
  if (
    !terms ||
    terms.kind !== item.kind ||
    terms.credits !== item.credits ||
    terms.tier !== (item.kind === 'subscription' ? item.tier : null) ||
    terms.quantity !== 1 ||
    !item.displayPrice ||
    terms.unitAmount !== item.displayPrice.amount ||
    terms.currency !== item.displayPrice.currency.toLowerCase() ||
    terms.interval !== (item.kind === 'subscription' ? item.displayPrice.interval : null) ||
    terms.intervalCount !== (item.kind === 'subscription' ? 1 : null)
  ) {
    throw new BillingRequestError(
      'unknown_price',
      'This Stripe price has no matching approved purchase terms. Checkout is unavailable until the price is reviewed.',
    );
  }
  const verifiedPrice = await stripeClient().prices.retrieve(item.priceId);
  const recurringInterval = verifiedPrice.recurring?.interval ?? null;
  if (
    !verifiedPrice.active ||
    verifiedPrice.billing_scheme !== 'per_unit' ||
    verifiedPrice.unit_amount === null ||
    verifiedPrice.tiers_mode !== null ||
    verifiedPrice.transform_quantity !== null ||
    verifiedPrice.recurring?.usage_type === 'metered' ||
    verifiedPrice.unit_amount !== terms.unitAmount ||
    verifiedPrice.currency.toLowerCase() !== terms.currency ||
    (verifiedPrice.type === 'recurring') !== (terms.kind === 'subscription') ||
    verifiedPrice.recurring?.interval !== terms.interval ||
    (verifiedPrice.recurring?.interval_count ?? null) !== terms.intervalCount ||
    recurringInterval !== terms.interval
  ) {
    throw new BillingRequestError('unknown_price', 'The Stripe price no longer matches its approved purchase terms.');
  }
  const hasActiveSubscription = await userHasActiveSubscription(db, input.user.id);
  if (item.kind === 'subscription' && hasActiveSubscription) {
    throw new BillingEligibilityError(
      'already_subscribed',
      'You already have an active Pro subscription. Manage it from billing settings.',
    );
  }
  if (item.kind === 'top_up' && !hasActiveSubscription) {
    throw new BillingEligibilityError('top_up_requires_subscription', 'Top-ups are available after subscribing to Pro.');
  }
  const customer = await getOrCreateStripeCustomer(input.user);
  const stripe = stripeClient();
  const intentHash = checkoutIntentHash(input, item.priceId);
  const claimed = await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${input.user.id} FOR UPDATE`);
    const [existing] = await tx
      .select()
      .from(billingOrders)
      .where(and(eq(billingOrders.userId, input.user.id), eq(billingOrders.attemptKey, attemptKey)))
      .limit(1);
    if (existing) {
      if (existing.intentHash !== intentHash)
        throw new BillingRequestError('idempotency_conflict', 'This idempotency key was already used for a different purchase.');
      if (existing.state === 'open' && existing.sessionUrl) return { kind: 'url' as const, url: existing.sessionUrl };
      if (['completed', 'expired', 'failed', 'retry_required', 'needs_reconciliation'].includes(existing.state)) {
        const status = existing.state === 'needs_reconciliation' ? 'reconciliation_required' : existing.state;
        return {
          kind: 'status' as const,
          status: status as BillingPendingError['status'],
        };
      }
      if (existing.state !== 'creating')
        return {
          kind: 'status' as const,
          status: 'processing' as const,
          attemptKey: existing.attemptKey,
        };
      if (existing.leaseExpiresAt && existing.leaseExpiresAt > new Date())
        return { kind: 'status' as const, status: 'processing' as const };
      if (Date.now() - existing.firstAttemptAt.getTime() > 23 * 60 * 60 * 1000) {
        await tx
          .update(billingOrders)
          .set({
            leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
            updatedAt: new Date(),
          })
          .where(and(eq(billingOrders.id, existing.id), eq(billingOrders.version, existing.version)));
        return { kind: 'lookup_order' as const, order: existing };
      }
      const generation = existing.leaseGeneration + 1;
      await tx
        .update(billingOrders)
        .set({
          leaseGeneration: generation,
          leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
          updatedAt: new Date(),
        })
        .where(eq(billingOrders.id, existing.id));
      return {
        kind: 'owner' as const,
        orderId: existing.id,
        generation,
        snapshot: existing.requestSnapshot as Stripe.Checkout.SessionCreateParams,
      };
    }
    const [active] = await tx
      .select()
      .from(billingOrders)
      .where(
        and(
          eq(billingOrders.userId, input.user.id),
          sql`${billingOrders.state} in ('creating', 'open', 'processing', 'retry_required', 'retry_cancel_pending', 'needs_reconciliation')`,
        ),
      )
      .limit(1);
    if (active && input.legacyCaller) return { kind: 'legacy_in_flight' as const };
    if (active)
      return {
        kind: 'status' as const,
        status: 'processing' as const,
        attemptKey: active.attemptKey,
      };
    const [latestUser] = await tx
      .select({ stripeCustomerId: users.stripeCustomerId })
      .from(users)
      .where(eq(users.id, input.user.id))
      .limit(1);
    if (!latestUser?.stripeCustomerId || latestUser.stripeCustomerId !== customer) throw new BillingPendingError('processing');
    const [subscription] = await tx
      .select({ status: subscriptions.status })
      .from(subscriptions)
      .where(eq(subscriptions.userId, input.user.id))
      .limit(1);
    const eligible = subscription ? isActiveBillingSubscriptionStatus(subscription.status) : false;
    if (item.kind === 'top_up' && !eligible)
      throw new BillingEligibilityError('top_up_requires_subscription', 'Top-ups are available after subscribing to Pro.');
    if (item.kind === 'subscription' && eligible)
      throw new BillingEligibilityError(
        'already_subscribed',
        'You already have an active Pro subscription. Manage it from billing settings.',
      );
    const orderId = randomUUID();
    const metadata: Stripe.MetadataParam = {
      billing_kind: item.kind,
      user_id: input.user.id,
      price_id: item.priceId,
      credits: String(item.credits),
      order_id: orderId,
      ...(item.kind === 'subscription' ? { tier: item.tier } : {}),
    };
    const snapshot: Stripe.Checkout.SessionCreateParams = {
      mode: item.kind === 'subscription' ? 'subscription' : 'payment',
      customer,
      client_reference_id: input.user.id,
      line_items: [{ price: item.priceId, quantity: 1 }],
      success_url: publicUrl('/profile?billing=success&session_id={CHECKOUT_SESSION_ID}'),
      cancel_url: publicUrl('/profile?billing=canceled'),
      metadata,
      ...(item.kind === 'subscription' ? { subscription_data: { metadata } } : { payment_intent_data: { metadata } }),
    };
    await tx.insert(billingOrders).values({
      id: orderId,
      userId: input.user.id,
      attemptKey,
      intentHash,
      kind: item.kind,
      priceTermsId: terms.id,
      priceId: terms.priceId,
      tier: terms.tier,
      credits: terms.credits,
      expectedUnitAmount: terms.unitAmount,
      expectedTotalAmount: terms.unitAmount * terms.quantity,
      currency: terms.currency,
      quantity: terms.quantity,
      interval: terms.interval,
      intervalCount: terms.intervalCount,
      stripeCustomerId: customer,
      requestSnapshot: snapshot,
      state: 'creating',
      leaseGeneration: 1,
      leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
    });
    return { kind: 'owner' as const, orderId, generation: 1, snapshot };
  });
  if (claimed.kind === 'url') return { status: 'url' as const, url: claimed.url };
  if (claimed.kind === 'legacy_in_flight')
    throw new BillingRequestError('checkout_in_flight', 'A checkout session is already in progress. Please wait and try again.');
  if (claimed.kind === 'status') throw new BillingPendingError(claimed.status, 'attemptKey' in claimed ? claimed.attemptKey : undefined);
  if (claimed.kind === 'lookup_order') return recoverAmbiguousCheckoutCreate(claimed.order);
  try {
    const session = await stripe.checkout.sessions.create(claimed.snapshot, {
      idempotencyKey: `billing-checkout-${claimed.orderId}`,
      timeout: STRIPE_REQUEST_TIMEOUT_MS,
    });
    const persisted = await db.transaction(async (tx) => {
      await tx.execute(sql`SELECT id FROM users WHERE id = ${input.user.id} FOR UPDATE`);
      const [current] = await tx.select().from(billingOrders).where(eq(billingOrders.id, claimed.orderId)).limit(1);
      if (!current || current.state !== 'creating' || current.leaseGeneration !== claimed.generation) return [];
      return tx
        .update(billingOrders)
        .set({
          sessionId: session.id,
          sessionUrl: session.url,
          paymentMethodTypes: session.payment_method_types ?? [],
          paymentIntentId: objectId(session.payment_intent),
          state: session.status === 'open' && session.url ? 'open' : 'processing',
          leaseExpiresAt: null,
          version: current.version + 1,
          updatedAt: new Date(),
        })
        .where(
          and(
            eq(billingOrders.id, claimed.orderId),
            eq(billingOrders.leaseGeneration, claimed.generation),
            eq(billingOrders.state, 'creating'),
          ),
        )
        .returning({ state: billingOrders.state });
    });
    if (persisted.length === 0) throw new BillingPendingError('processing');
    if (session.status === 'open' && session.url) return { status: 'url' as const, url: session.url };
    return { status: 'processing' as const };
  } catch (error) {
    if (error instanceof BillingPendingError) throw error;
    if (error instanceof Stripe.errors.StripeIdempotencyError) throw new BillingPendingError('processing');
    if (error instanceof Stripe.errors.StripeInvalidRequestError || error instanceof Stripe.errors.StripeAuthenticationError) {
      await updateBillingOrderWithUserLock({
        userId: input.user.id,
        orderId: claimed.orderId,
        expectedGeneration: claimed.generation,
        changes: {
          state: 'failed',
          leaseExpiresAt: null,
          version: sql`${billingOrders.version} + 1`,
          updatedAt: new Date(),
        },
      });
      throw new BillingPendingError('failed');
    }
    log.warn('stripe_checkout_create_ambiguous', {
      order_id: claimed.orderId,
      error,
    });
    throw new BillingPendingError('processing');
  }
}

async function recoverAmbiguousCheckoutCreate(order: typeof billingOrders.$inferSelect) {
  const stripe = stripeClient();
  const matches: Stripe.Checkout.Session[] = [];
  let startingAfter: string | undefined;
  let terminal = false;
  for (let pageIndex = 0; pageIndex < 3; pageIndex += 1) {
    const page = await stripe.checkout.sessions.list({
      customer: order.stripeCustomerId,
      created: { gte: Math.floor(order.firstAttemptAt.getTime() / 1000) },
      limit: 100,
      ...(startingAfter ? { starting_after: startingAfter } : {}),
    });
    matches.push(...page.data.filter((session) => session.metadata?.order_id === order.id));
    if (!page.has_more) {
      terminal = true;
      break;
    }
    startingAfter = page.data.at(-1)?.id;
    if (!startingAfter) break;
  }
  if (!terminal || matches.length !== 1) {
    await updateBillingOrderWithUserLock({
      userId: order.userId,
      orderId: order.id,
      expectedVersion: order.version,
      expectedGeneration: order.leaseGeneration,
      changes: {
        state: 'needs_reconciliation',
        leaseExpiresAt: null,
        version: order.version + 1,
        updatedAt: new Date(),
      },
    });
    throw new BillingPendingError('reconciliation_required');
  }
  const session = await stripe.checkout.sessions.retrieve(matches[0].id);
  if (objectId(session.customer) !== order.stripeCustomerId || session.metadata?.order_id !== order.id) {
    await updateBillingOrderWithUserLock({
      userId: order.userId,
      orderId: order.id,
      expectedVersion: order.version,
      expectedGeneration: order.leaseGeneration,
      changes: {
        state: 'needs_reconciliation',
        leaseExpiresAt: null,
        version: order.version + 1,
        updatedAt: new Date(),
      },
    });
    throw new BillingPendingError('reconciliation_required');
  }
  const intentId = objectId(session.payment_intent);
  const intent = intentId ? await stripe.paymentIntents.retrieve(intentId) : null;
  const state = session.status
    ? classifyCheckoutEvidence({
        sessionStatus: session.status,
        paymentStatus: session.payment_status,
        paymentIntentStatus: intent?.status,
      })
    : 'needs_reconciliation';
  const updated = await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${order.userId} FOR UPDATE`);
    const [current] = await tx.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
    if (!current || current.state === 'completed' || current.version !== order.version || current.leaseGeneration !== order.leaseGeneration)
      return [];
    return tx
      .update(billingOrders)
      .set({
        sessionId: session.id,
        sessionUrl: session.url,
        paymentMethodTypes: session.payment_method_types ?? [],
        paymentIntentId: intentId,
        state,
        leaseExpiresAt: null,
        version: current.version + 1,
        updatedAt: new Date(),
      })
      .where(and(eq(billingOrders.id, order.id), eq(billingOrders.version, current.version)))
      .returning({ id: billingOrders.id });
  });
  if (!updated.length) throw new BillingPendingError('processing');
  if (state === 'open' && session.url) return { status: 'url' as const, url: session.url };
  throw new BillingPendingError(state === 'needs_reconciliation' ? 'reconciliation_required' : (state as BillingPendingError['status']));
}

export async function createBillingPortal(input: { user: typeof users.$inferSelect }) {
  if (!input.user.stripeCustomerId) throw new BillingConfigError('No Stripe customer exists for this user');
  const stripe = stripeClient();
  return stripe.billingPortal.sessions.create({
    customer: input.user.stripeCustomerId,
    return_url: publicUrl('/profile'),
  });
}

export async function cancelPaymentIntentForRetry(paymentIntentId: string, idempotencyKey: string): Promise<Stripe.PaymentIntent> {
  const stripe = stripeClient();
  await stripe.paymentIntents.cancel(
    paymentIntentId,
    {},
    {
      idempotencyKey,
      timeout: STRIPE_REQUEST_TIMEOUT_MS,
    },
  );
  return stripe.paymentIntents.retrieve(paymentIntentId);
}

export function canCreateRetryReplacement(paymentIntentStatus: string): boolean {
  return paymentIntentStatus === 'canceled';
}

export type PublicCheckoutStatus =
  | 'none'
  | 'open'
  | 'processing'
  | 'retry_required'
  | 'completed'
  | 'expired'
  | 'failed'
  | 'reconciliation_required';

export function classifyCheckoutEvidence(input: {
  sessionStatus: 'open' | 'complete' | 'expired';
  paymentStatus: 'paid' | 'unpaid' | 'no_payment_required';
  paymentIntentStatus?: Stripe.PaymentIntent.Status;
}): 'open' | 'processing' | 'retry_required' | 'expired' | 'failed' {
  if (input.paymentStatus === 'paid') return 'processing';
  if (input.sessionStatus === 'open') return 'open';
  if (input.sessionStatus === 'expired') {
    if (input.paymentStatus !== 'unpaid') return 'processing';
    if (!input.paymentIntentStatus || input.paymentIntentStatus === 'canceled') return 'expired';
    if (input.paymentIntentStatus === 'requires_payment_method') return 'retry_required';
    return 'processing';
  }
  if (input.paymentStatus !== 'unpaid') return 'processing';
  if (input.paymentIntentStatus === 'requires_payment_method') return 'retry_required';
  if (input.paymentIntentStatus === 'canceled') return 'failed';
  return 'processing';
}

function publicOrderStatus(state: string): PublicCheckoutStatus {
  if (state === 'needs_reconciliation') return 'reconciliation_required';
  if (state === 'retry_cancel_pending' || state === 'creating') return 'processing';
  if (
    state === 'open' ||
    state === 'processing' ||
    state === 'retry_required' ||
    state === 'completed' ||
    state === 'expired' ||
    state === 'failed'
  )
    return state;
  return 'processing';
}

async function resolveOwnedBillingOrder(input: { userId: string; attemptKey?: string; sessionId?: string; retryOperationId?: string }) {
  const found: Array<typeof billingOrders.$inferSelect> = [];
  if (input.attemptKey) {
    const [order] = await db
      .select()
      .from(billingOrders)
      .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.attemptKey, input.attemptKey)))
      .limit(1);
    if (order) found.push(order);
    else if (input.sessionId || input.retryOperationId)
      throw new BillingRequestError('idempotency_conflict', 'The supplied billing identifiers do not identify the same purchase.');
    else return null;
  }
  if (input.sessionId) {
    const [order] = await db
      .select()
      .from(billingOrders)
      .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.sessionId, input.sessionId)))
      .limit(1);
    if (order) found.push(order);
    else if (input.attemptKey || input.retryOperationId)
      throw new BillingRequestError('idempotency_conflict', 'The supplied billing identifiers do not identify the same purchase.');
    else return null;
  }
  let operation: typeof billingRetryOperations.$inferSelect | undefined;
  const userOperations = await db.select().from(billingRetryOperations).where(eq(billingRetryOperations.userId, input.userId));
  if (input.retryOperationId) {
    operation = userOperations.find((candidate) => candidate.id === input.retryOperationId);
    if (!operation) return null;
  }
  if (found.length > 1 && found.some((order) => order.id !== found[0].id)) {
    const linked = userOperations.filter((candidate) =>
      found.every((order) => order.id === candidate.priorOrderId || order.id === candidate.replacementOrderId),
    );
    if (linked.length !== 1 || (operation && linked[0].id !== operation.id)) {
      throw new BillingRequestError('idempotency_conflict', 'The supplied billing identifiers refer to different purchases.');
    }
    operation = linked[0];
  } else if (!operation && found.length) {
    operation = userOperations.find((candidate) => candidate.priorOrderId === found[0].id || candidate.replacementOrderId === found[0].id);
  }
  if (operation) {
    const relatedIds = [operation.priorOrderId, operation.replacementOrderId].filter((id): id is string => Boolean(id));
    if (found.some((order) => !relatedIds.includes(order.id))) {
      throw new BillingRequestError('idempotency_conflict', 'The supplied billing identifiers refer to different purchases.');
    }
    const preferredId = operation.replacementOrderId ?? operation.priorOrderId;
    const [preferred] = await db
      .select()
      .from(billingOrders)
      .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.id, preferredId)))
      .limit(1);
    if (preferred) return preferred;
  }
  if (found.length) return found[0];
  if (input.retryOperationId && operation) {
    const preferredId = operation.replacementOrderId ?? operation.priorOrderId;
    const [related] = await db
      .select()
      .from(billingOrders)
      .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.id, preferredId)))
      .limit(1);
    return related ?? null;
  }
  const [active] = await db
    .select()
    .from(billingOrders)
    .where(
      and(
        eq(billingOrders.userId, input.userId),
        sql`${billingOrders.state} in ('creating', 'open', 'processing', 'retry_required', 'retry_cancel_pending', 'needs_reconciliation')`,
      ),
    )
    .limit(1);
  return active ?? null;
}

async function checkoutStatusResponse(order: typeof billingOrders.$inferSelect | null, userId: string) {
  if (!order) return { status: 'none' as const };
  const [retry] = await db
    .select({
      id: billingRetryOperations.id,
      state: billingRetryOperations.state,
    })
    .from(billingRetryOperations)
    .where(
      and(
        eq(billingRetryOperations.userId, userId),
        or(eq(billingRetryOperations.priorOrderId, order.id), eq(billingRetryOperations.replacementOrderId, order.id)),
      ),
    )
    .limit(1);
  return {
    status: publicOrderStatus(order.state),
    kind: order.kind,
    priceId: order.priceId,
    ...(order.sessionUrl && order.state === 'open' ? { resumeUrl: order.sessionUrl } : {}),
    ...(retry && !['completed', 'expired', 'failed', 'superseded', 'retryable_failed'].includes(retry.state)
      ? { retryOperationId: retry.id }
      : {}),
    ...(order.attemptKey ? { attemptKey: order.attemptKey } : {}),
  };
}

export async function getBillingCheckoutStatus(input: {
  userId: string;
  attemptKey?: string;
  sessionId?: string;
  retryOperationId?: string;
}) {
  const order = await resolveOwnedBillingOrder(input);
  if (!order) {
    const [customerAttempt] = await db
      .select({ status: stripeCustomerAttempts.status })
      .from(stripeCustomerAttempts)
      .where(eq(stripeCustomerAttempts.userId, input.userId))
      .limit(1);
    if (customerAttempt?.status === 'reconciliation_required') return { status: 'reconciliation_required' as const };
    if (customerAttempt?.status === 'creating') return { status: 'processing' as const };
  }
  return checkoutStatusResponse(order, input.userId);
}

export async function refreshBillingCheckoutStatus(input: {
  userId: string;
  attemptKey?: string;
  sessionId?: string;
  retryOperationId?: string;
}) {
  const order = await resolveOwnedBillingOrder(input);
  if (!order) return getBillingCheckoutStatus(input);
  if (!order.sessionId) return checkoutStatusResponse(order, input.userId);
  const stripe = stripeClient();
  const session = await stripe.checkout.sessions.retrieve(order.sessionId);
  const sessionCustomerId = objectId(session.customer);
  if (session.id !== order.sessionId || sessionCustomerId !== order.stripeCustomerId || session.metadata?.order_id !== order.id) {
    await updateBillingOrderWithUserLock({
      userId: order.userId,
      orderId: order.id,
      expectedVersion: order.version,
      changes: {
        state: 'needs_reconciliation',
        version: order.version + 1,
        updatedAt: new Date(),
      },
    });
    const [current] = await db.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
    return checkoutStatusResponse(current ?? order, input.userId);
  }
  let nextState: string;
  const paymentIntentId = objectId(session.payment_intent);
  let intent: Stripe.PaymentIntent | null = null;
  if (paymentIntentId) intent = await stripe.paymentIntents.retrieve(paymentIntentId);
  if (intent && (objectId(intent.customer) !== order.stripeCustomerId || metadataOf(intent).order_id !== order.id)) {
    await updateBillingOrderWithUserLock({
      userId: order.userId,
      orderId: order.id,
      expectedVersion: order.version,
      changes: {
        state: 'needs_reconciliation',
        version: order.version + 1,
        updatedAt: new Date(),
      },
    });
    const [current] = await db.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
    return checkoutStatusResponse(current ?? order, input.userId);
  }
  if (order.state === 'retry_cancel_pending') return checkoutStatusResponse(order, input.userId);
  nextState = session.status
    ? classifyCheckoutEvidence({
        sessionStatus: session.status,
        paymentStatus: session.payment_status,
        paymentIntentStatus: intent?.status,
      })
    : 'needs_reconciliation';

  await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${order.userId} FOR UPDATE`);
    await tx.execute(sql`SELECT id FROM billing_orders WHERE id = ${order.id} FOR UPDATE`);
    const [current] = await tx.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
    if (!current || current.state === 'completed' || current.version !== order.version) return;
    await tx
      .update(billingOrders)
      .set({
        state: nextState,
        paymentIntentId: paymentIntentId ?? current.paymentIntentId,
        version: current.version + 1,
        updatedAt: new Date(),
      })
      .where(and(eq(billingOrders.id, order.id), eq(billingOrders.version, current.version)));
  });
  const [updated] = await db.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
  return checkoutStatusResponse(updated ?? order, input.userId);
}

export async function retryBillingCheckout(input: { userId: string; attemptKey?: string; retryOperationId?: string; retryKey: string }) {
  let createdOperationId: string | null = null;
  const claim = await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${input.userId} FOR UPDATE`);
    let operation: typeof billingRetryOperations.$inferSelect | undefined;
    const [sameKeyOperation] = await tx
      .select()
      .from(billingRetryOperations)
      .where(and(eq(billingRetryOperations.userId, input.userId), eq(billingRetryOperations.retryKey, input.retryKey)))
      .limit(1);
    if (sameKeyOperation) {
      if (input.retryOperationId && input.retryOperationId !== sameKeyOperation.id) {
        throw new BillingRequestError('idempotency_conflict', 'This retry key is already associated with another retry operation.');
      }
      operation = sameKeyOperation;
    } else if (input.retryOperationId) {
      [operation] = await tx
        .select()
        .from(billingRetryOperations)
        .where(and(eq(billingRetryOperations.userId, input.userId), eq(billingRetryOperations.id, input.retryOperationId)))
        .limit(1);
      if (!operation) return { kind: 'status' as const, status: 'none' as const };
      throw new BillingRequestError('idempotency_conflict', 'This retry operation was created with a different retry key.');
    }
    let priorOrder: typeof billingOrders.$inferSelect | undefined;
    if (operation) {
      [priorOrder] = await tx
        .select()
        .from(billingOrders)
        .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.id, operation.priorOrderId)))
        .limit(1);
      if (input.attemptKey) {
        const [attemptOrder] = await tx
          .select()
          .from(billingOrders)
          .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.attemptKey, input.attemptKey)))
          .limit(1);
        if (!attemptOrder || ![operation.priorOrderId, operation.replacementOrderId].includes(attemptOrder.id)) {
          throw new BillingRequestError('idempotency_conflict', 'The retry key and checkout attempt refer to different purchases.');
        }
      }
    } else if (input.attemptKey) {
      [priorOrder] = await tx
        .select()
        .from(billingOrders)
        .where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.attemptKey, input.attemptKey)))
        .limit(1);
    }
    if (!priorOrder) return { kind: 'status' as const, status: 'none' as const };
    if (!operation) {
      if (priorOrder.state !== 'retry_required' || !priorOrder.sessionId || !priorOrder.paymentIntentId) {
        return {
          kind: 'status' as const,
          status: publicOrderStatus(priorOrder.state),
        };
      }
      const [activeOperation] = await tx
        .select()
        .from(billingRetryOperations)
        .where(
          and(
            eq(billingRetryOperations.priorOrderId, priorOrder.id),
            sql`${billingRetryOperations.state} in ('cancel_pending', 'replacement_ready', 'checkout_creating')`,
          ),
        )
        .limit(1);
      if (activeOperation)
        return {
          kind: 'status' as const,
          status: 'processing' as const,
          operationId: activeOperation.id,
        };
      const operationId = randomUUID();
      createdOperationId = operationId;
      await tx.insert(billingRetryOperations).values({
        id: operationId,
        userId: input.userId,
        priorOrderId: priorOrder.id,
        retryKey: input.retryKey,
        payloadHash: createHash('sha256').update(`${priorOrder.id}:${input.retryKey}`).digest('hex'),
        state: 'cancel_pending',
        cancelGeneration: 1,
        cancelIdempotencyKey: `billing-cancel-${operationId}-1`,
        leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
      });
      await tx
        .update(billingOrders)
        .set({
          state: 'retry_cancel_pending',
          version: priorOrder.version + 1,
          updatedAt: new Date(),
        })
        .where(and(eq(billingOrders.id, priorOrder.id), eq(billingOrders.version, priorOrder.version)));
      [operation] = await tx.select().from(billingRetryOperations).where(eq(billingRetryOperations.id, operationId)).limit(1);
    } else if (!input.retryOperationId && operation.retryKey !== input.retryKey && operation.state !== 'checkout_open') {
      return {
        kind: 'status' as const,
        status: 'processing' as const,
        operationId: operation.id,
      };
    }
    if (!operation) return { kind: 'status' as const, status: 'processing' as const };
    if (['retryable_failed', 'failed', 'expired', 'completed', 'superseded', 'needs_reconciliation'].includes(operation.state)) {
      return {
        kind: 'status' as const,
        status:
          operation.state === 'retryable_failed'
            ? ('retry_required' as const)
            : operation.state === 'needs_reconciliation'
              ? ('reconciliation_required' as const)
              : (operation.state as 'failed' | 'expired' | 'completed'),
      };
    }
    if (operation.state === 'checkout_open' && operation.replacementOrderId) {
      const [replacement] = await tx.select().from(billingOrders).where(eq(billingOrders.id, operation.replacementOrderId)).limit(1);
      if (replacement?.state === 'open' && replacement.sessionUrl)
        return {
          kind: 'url' as const,
          url: replacement.sessionUrl,
          operationId: operation.id,
        };
      return {
        kind: 'status' as const,
        status: 'processing' as const,
        operationId: operation.id,
      };
    }
    if (operation.id !== createdOperationId && operation.leaseExpiresAt && operation.leaseExpiresAt > new Date())
      return {
        kind: 'status' as const,
        status: 'processing' as const,
        operationId: operation.id,
      };
    if (operation.state === 'replacement_ready' || operation.state === 'checkout_creating') {
      const [replacement] = await tx.select().from(billingOrders).where(eq(billingOrders.id, operation.replacementOrderId!)).limit(1);
      if (!replacement)
        return {
          kind: 'status' as const,
          status: 'reconciliation_required' as const,
          operationId: operation.id,
        };
      const generation = replacement.leaseGeneration + 1;
      await tx
        .update(billingRetryOperations)
        .set({
          state: 'checkout_creating',
          leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
          updatedAt: new Date(),
        })
        .where(eq(billingRetryOperations.id, operation.id));
      await tx
        .update(billingOrders)
        .set({
          leaseGeneration: generation,
          leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
          updatedAt: new Date(),
        })
        .where(eq(billingOrders.id, replacement.id));
      return {
        kind: 'create_replacement' as const,
        operationId: operation.id,
        userId: operation.userId,
        orderId: replacement.id,
        generation,
        snapshot: replacement.requestSnapshot as Stripe.Checkout.SessionCreateParams,
      };
    }
    const needsNewCancelGeneration = operation.state === 'cancel_pending' && operation.id !== createdOperationId;
    const nextGeneration = needsNewCancelGeneration ? operation.cancelGeneration + 1 : operation.cancelGeneration;
    const nextCancelKey = needsNewCancelGeneration ? `billing-cancel-${operation.id}-${nextGeneration}` : operation.cancelIdempotencyKey;
    await tx
      .update(billingRetryOperations)
      .set({
        cancelGeneration: nextGeneration,
        cancelIdempotencyKey: nextCancelKey,
        leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS),
        updatedAt: new Date(),
      })
      .where(eq(billingRetryOperations.id, operation.id));
    return {
      kind: 'cancel' as const,
      operationId: operation.id,
      order: priorOrder,
      operation: {
        ...operation,
        cancelGeneration: nextGeneration,
        cancelIdempotencyKey: nextCancelKey,
      },
    };
  });

  if (claim.kind === 'status')
    return {
      status: claim.status,
      ...(claim.operationId ? { retryOperationId: claim.operationId } : {}),
    };
  if (claim.kind === 'url')
    return {
      status: 'url' as const,
      url: claim.url,
      retryOperationId: claim.operationId,
    };
  const stripe = stripeClient();
  if (claim.kind === 'create_replacement') {
    try {
      const session = await stripe.checkout.sessions.create(claim.snapshot, {
        idempotencyKey: `billing-checkout-${claim.orderId}`,
        timeout: STRIPE_REQUEST_TIMEOUT_MS,
      });
      const nextState = session.status === 'open' && session.url ? 'open' : 'processing';
      const persisted = await db.transaction(async (tx) => {
        await tx.execute(sql`SELECT id FROM users WHERE id = ${claim.userId} FOR UPDATE`);
        const [replacement] = await tx.select().from(billingOrders).where(eq(billingOrders.id, claim.orderId)).limit(1);
        if (!replacement || replacement.state !== 'creating' || replacement.leaseGeneration !== claim.generation) return false;
        await tx
          .update(billingOrders)
          .set({
            sessionId: session.id,
            sessionUrl: session.url,
            paymentMethodTypes: session.payment_method_types ?? [],
            paymentIntentId: objectId(session.payment_intent),
            state: nextState,
            leaseExpiresAt: null,
            version: replacement.version + 1,
            updatedAt: new Date(),
          })
          .where(
            and(
              eq(billingOrders.id, claim.orderId),
              eq(billingOrders.leaseGeneration, claim.generation),
              eq(billingOrders.state, 'creating'),
            ),
          );
        await tx
          .update(billingRetryOperations)
          .set({
            state: nextState === 'open' ? 'checkout_open' : 'checkout_creating',
            leaseExpiresAt: null,
            updatedAt: new Date(),
          })
          .where(eq(billingRetryOperations.id, claim.operationId));
        return true;
      });
      if (!persisted)
        return {
          status: 'processing' as const,
          retryOperationId: claim.operationId,
        };
      return session.status === 'open' && session.url
        ? {
            status: 'url' as const,
            url: session.url,
            retryOperationId: claim.operationId,
          }
        : {
            status: 'processing' as const,
            retryOperationId: claim.operationId,
          };
    } catch (error) {
      log.warn('stripe_retry_checkout_ambiguous', {
        retry_operation_id: claim.operationId,
        error,
      });
      return {
        status: 'processing' as const,
        retryOperationId: claim.operationId,
      };
    }
  }

  let session: Stripe.Checkout.Session;
  let intent: Stripe.PaymentIntent;
  try {
    session = await stripe.checkout.sessions.retrieve(claim.order.sessionId!);
    if (
      session.id !== claim.order.sessionId ||
      objectId(session.customer) !== claim.order.stripeCustomerId ||
      session.metadata?.order_id !== claim.order.id ||
      objectId(session.payment_intent) !== claim.order.paymentIntentId
    ) {
      await db.transaction(async (tx) => {
        await tx.execute(sql`SELECT id FROM users WHERE id = ${claim.order.userId} FOR UPDATE`);
        const [current] = await tx.select().from(billingOrders).where(eq(billingOrders.id, claim.order.id)).limit(1);
        await tx
          .update(billingRetryOperations)
          .set({
            state: 'needs_reconciliation',
            leaseExpiresAt: null,
            updatedAt: new Date(),
          })
          .where(eq(billingRetryOperations.id, claim.operationId));
        if (current && current.state !== 'completed')
          await tx
            .update(billingOrders)
            .set({
              state: 'needs_reconciliation',
              version: current.version + 1,
              updatedAt: new Date(),
            })
            .where(and(eq(billingOrders.id, claim.order.id), eq(billingOrders.version, current.version)));
      });
      return {
        status: 'reconciliation_required' as const,
        retryOperationId: claim.operationId,
      };
    }
    if (session.status === 'open')
      return {
        status: 'processing' as const,
        retryOperationId: claim.operationId,
      };
    if ((session.status !== 'complete' && session.status !== 'expired') || session.payment_status !== 'unpaid') {
      if (session.payment_status === 'paid') {
        await db
          .update(billingRetryOperations)
          .set({
            state: 'needs_reconciliation',
            leaseExpiresAt: null,
            updatedAt: new Date(),
          })
          .where(eq(billingRetryOperations.id, claim.operationId));
      }
      return {
        status: session.payment_status === 'paid' ? ('reconciliation_required' as const) : ('processing' as const),
        retryOperationId: claim.operationId,
      };
    }
    if (!claim.order.paymentIntentId)
      return {
        status: 'processing' as const,
        retryOperationId: claim.operationId,
      };
    intent = await stripe.paymentIntents.retrieve(claim.order.paymentIntentId);
    if (objectId(intent.customer) !== claim.order.stripeCustomerId || metadataOf(intent).order_id !== claim.order.id) {
      await db
        .update(billingRetryOperations)
        .set({
          state: 'needs_reconciliation',
          leaseExpiresAt: null,
          updatedAt: new Date(),
        })
        .where(eq(billingRetryOperations.id, claim.operationId));
      return {
        status: 'reconciliation_required' as const,
        retryOperationId: claim.operationId,
      };
    }
  } catch (error) {
    log.warn('stripe_retry_refresh_failed', {
      retry_operation_id: claim.operationId,
      error,
    });
    return {
      status: 'processing' as const,
      retryOperationId: claim.operationId,
    };
  }
  if (intent.status === 'succeeded') {
    await db
      .update(billingRetryOperations)
      .set({
        state: 'needs_reconciliation',
        leaseExpiresAt: null,
        updatedAt: new Date(),
      })
      .where(eq(billingRetryOperations.id, claim.operationId));
    return {
      status: 'processing' as const,
      retryOperationId: claim.operationId,
    };
  }
  if (intent.status !== 'canceled' && intent.status !== 'requires_payment_method') {
    await db
      .update(billingRetryOperations)
      .set({ leaseExpiresAt: null, updatedAt: new Date() })
      .where(eq(billingRetryOperations.id, claim.operationId));
    return {
      status: 'processing' as const,
      retryOperationId: claim.operationId,
    };
  }
  let cancelRejected = false;
  if (intent.status === 'requires_payment_method') {
    try {
      intent = await cancelPaymentIntentForRetry(intent.id, claim.operation.cancelIdempotencyKey);
    } catch (error) {
      cancelRejected = error instanceof Stripe.errors.StripeInvalidRequestError;
      log.warn('stripe_payment_intent_cancel_result', {
        retry_operation_id: claim.operationId,
        definitive_rejection: cancelRejected,
        error,
      });
      try {
        intent = await stripe.paymentIntents.retrieve(intent.id);
      } catch {
        return {
          status: 'processing' as const,
          retryOperationId: claim.operationId,
        };
      }
    }
  }
  if (!canCreateRetryReplacement(intent.status)) {
    if (cancelRejected && intent.status === 'requires_payment_method') {
      await db.transaction(async (tx) => {
        await tx.execute(sql`SELECT id FROM users WHERE id = ${claim.order.userId} FOR UPDATE`);
        const [current] = await tx.select().from(billingOrders).where(eq(billingOrders.id, claim.order.id)).limit(1);
        await tx
          .update(billingRetryOperations)
          .set({
            state: 'retryable_failed',
            leaseExpiresAt: null,
            updatedAt: new Date(),
          })
          .where(eq(billingRetryOperations.id, claim.operationId));
        if (current && current.state !== 'completed')
          await tx
            .update(billingOrders)
            .set({
              state: 'retry_required',
              version: current.version + 1,
              updatedAt: new Date(),
            })
            .where(and(eq(billingOrders.id, claim.order.id), eq(billingOrders.version, current.version)));
      });
      return {
        status: 'retry_required' as const,
        retryOperationId: claim.operationId,
      };
    }
    return {
      status: 'processing' as const,
      retryOperationId: claim.operationId,
    };
  }

  const replacementId = randomUUID();
  const replacementAttemptKey = randomUUID();
  const requestSnapshot = structuredClone(claim.order.requestSnapshot) as Stripe.Checkout.SessionCreateParams;
  const replacementMetadata = {
    ...(requestSnapshot.metadata ?? {}),
    order_id: replacementId,
  };
  requestSnapshot.metadata = replacementMetadata;
  if (requestSnapshot.payment_intent_data) requestSnapshot.payment_intent_data.metadata = replacementMetadata;
  if (requestSnapshot.subscription_data) requestSnapshot.subscription_data.metadata = replacementMetadata;
  await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${claim.order.userId} FOR UPDATE`);
    const [currentOrder] = await tx.select().from(billingOrders).where(eq(billingOrders.id, claim.order.id)).limit(1);
    const [currentOperation] = await tx
      .select()
      .from(billingRetryOperations)
      .where(eq(billingRetryOperations.id, claim.operationId))
      .limit(1);
    if (!currentOrder || !currentOperation || currentOrder.state === 'completed') return;
    await tx
      .update(billingOrders)
      .set({
        state: 'failed',
        version: currentOrder.version + 1,
        updatedAt: new Date(),
      })
      .where(and(eq(billingOrders.id, currentOrder.id), eq(billingOrders.version, currentOrder.version)));
    await tx.insert(billingOrders).values({
      id: replacementId,
      userId: claim.order.userId,
      attemptKey: replacementAttemptKey,
      intentHash: claim.order.intentHash,
      kind: claim.order.kind,
      priceTermsId: claim.order.priceTermsId,
      priceId: claim.order.priceId,
      tier: claim.order.tier,
      credits: claim.order.credits,
      expectedUnitAmount: claim.order.expectedUnitAmount,
      expectedTotalAmount: claim.order.expectedTotalAmount,
      currency: claim.order.currency,
      quantity: claim.order.quantity,
      interval: claim.order.interval,
      intervalCount: claim.order.intervalCount,
      stripeCustomerId: claim.order.stripeCustomerId,
      requestSnapshot,
      state: 'creating',
      leaseGeneration: 0,
    });
    await tx
      .update(billingRetryOperations)
      .set({
        state: 'replacement_ready',
        replacementOrderId: replacementId,
        leaseExpiresAt: null,
        updatedAt: new Date(),
      })
      .where(eq(billingRetryOperations.id, claim.operationId));
  });
  return { status: 'processing' as const, retryOperationId: claim.operationId };
}

function unixSecondsToDate(value: unknown): Date {
  return typeof value === 'number' && Number.isFinite(value) ? new Date(value * 1000) : new Date();
}

function objectId(value: unknown): string | null {
  if (typeof value === 'string') return value;
  if (value && typeof value === 'object' && 'id' in value && typeof (value as { id: unknown }).id === 'string')
    return (value as { id: string }).id;
  return null;
}

function metadataOf(value: unknown): Record<string, string> {
  return value &&
    typeof value === 'object' &&
    'metadata' in value &&
    (value as { metadata?: unknown }).metadata &&
    typeof (value as { metadata: unknown }).metadata === 'object'
    ? (value as { metadata: Record<string, string> }).metadata
    : {};
}

function subscriptionPriceId(subscription: Stripe.Subscription): string | null {
  const item = (subscription as any).items?.data?.[0] as any;
  return item?.price?.id ?? item?.pricing?.price_details?.price ?? null;
}

function subscriptionCurrentPeriodEnd(subscription: Stripe.Subscription): Date {
  const item = (subscription as any).items?.data?.[0] as any;
  return unixSecondsToDate(item?.current_period_end ?? (subscription as any).current_period_end);
}

type BillingDb = typeof db;

async function updateBillingOrderWithUserLock(input: {
  userId: string;
  orderId: string;
  changes: Partial<typeof billingOrders.$inferInsert>;
  expectedVersion?: number;
  expectedGeneration?: number;
}) {
  return db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${input.userId} FOR UPDATE`);
    const [current] = await tx.select().from(billingOrders).where(eq(billingOrders.id, input.orderId)).limit(1);
    if (
      !current ||
      current.state === 'completed' ||
      (input.expectedVersion !== undefined && current.version !== input.expectedVersion) ||
      (input.expectedGeneration !== undefined && current.leaseGeneration !== input.expectedGeneration)
    )
      return false;
    await tx
      .update(billingOrders)
      .set(input.changes)
      .where(and(eq(billingOrders.id, input.orderId), eq(billingOrders.version, current.version)));
    return true;
  });
}

async function userByStripeCustomer(database: BillingDb, customerId: string | null): Promise<typeof users.$inferSelect | null> {
  if (!customerId) return null;
  const [user] = await database.select().from(users).where(eq(users.stripeCustomerId, customerId)).limit(1);
  return user ?? null;
}

async function grantPaidCredits(database: BillingDb, userId: string, credits: number, reason: string, refId: string) {
  if (credits <= 0) return;
  await database
    .insert(creditLedger)
    .values({
      userId,
      delta: credits,
      kind: 'paid',
      reason,
      refId,
    })
    .onConflictDoNothing({
      target: [creditLedger.userId, creditLedger.reason, creditLedger.refId],
    });
}

async function updateUserPlan(database: BillingDb, userId: string, plan: string) {
  await database.update(users).set({ plan }).where(eq(users.id, userId));
}

async function upsertSubscription(
  database: BillingDb,
  input: {
    userId: string;
    stripeSubId: string;
    tier: string;
    status: string;
    currentPeriodEnd: Date;
    lastEventCreatedAt?: Date;
    lastEventId?: string;
  },
) {
  const versionColumns = input.lastEventCreatedAt
    ? {
        lastEventCreatedAt: input.lastEventCreatedAt,
        lastEventId: input.lastEventId ?? null,
      }
    : {};
  await database
    .insert(subscriptions)
    .values({ ...input, ...versionColumns })
    .onConflictDoUpdate({
      target: subscriptions.userId,
      set: {
        stripeSubId: input.stripeSubId,
        tier: input.tier,
        status: input.status,
        currentPeriodEnd: input.currentPeriodEnd,
        ...versionColumns,
      },
    });
  await updateUserPlan(database, input.userId, isActiveBillingSubscriptionStatus(input.status) ? input.tier : 'free');
}

export function isActiveBillingSubscriptionStatus(status: string): boolean {
  return status === 'active' || status === 'trialing' || status === 'past_due';
}

async function retrieveSubscription(subscriptionId: string): Promise<Stripe.Subscription> {
  return stripeClient().subscriptions.retrieve(subscriptionId);
}

type EventOutcome = {
  state: 'processed' | 'needs_reconciliation';
  reason?: string;
};

type PreparedStripeEvent = {
  event: Stripe.Event;
  session?: Stripe.Checkout.Session;
  signedSession?: Stripe.Checkout.Session;
  lineItems?: Stripe.ApiList<Stripe.LineItem>;
  paymentIntent?: Stripe.PaymentIntent;
  invoice?: Stripe.Invoice;
  subscription: Stripe.Subscription | null;
};

async function orderForCheckout(database: BillingDb, session: Stripe.Checkout.Session) {
  const orderId = session.metadata?.order_id;
  if (orderId) {
    const [order] = await database.select().from(billingOrders).where(eq(billingOrders.id, orderId)).limit(1);
    return order ?? null;
  }
  const [order] = await database.select().from(billingOrders).where(eq(billingOrders.sessionId, session.id)).limit(1);
  return order ?? null;
}

async function lockBillingUser(database: BillingDb, userId: string) {
  await database.execute(sql`SELECT id FROM users WHERE id = ${userId} FOR UPDATE`);
}

export async function updateBillingOrderCAS(
  database: BillingDb,
  input: {
    userId: string;
    orderId: string;
    changes: Partial<typeof billingOrders.$inferInsert>;
  },
) {
  await lockBillingUser(database, input.userId);
  const [current] = await database.select().from(billingOrders).where(eq(billingOrders.id, input.orderId)).limit(1);
  if (!current || current.state === 'completed') return null;
  const [updated] = await database
    .update(billingOrders)
    .set({ ...input.changes, version: current.version + 1, updatedAt: new Date() })
    .where(and(eq(billingOrders.id, current.id), eq(billingOrders.version, current.version), eq(billingOrders.state, current.state)))
    .returning();
  return updated ?? null;
}

async function reconcileBillingOrder(database: BillingDb, order: typeof billingOrders.$inferSelect, reason: string): Promise<EventOutcome> {
  const updated = await updateBillingOrderCAS(database, {
    userId: order.userId,
    orderId: order.id,
    changes: { state: 'needs_reconciliation' },
  });
  return updated ? { state: 'needs_reconciliation', reason } : { state: 'processed' };
}

function checkoutTermsMatch(
  order: typeof billingOrders.$inferSelect,
  session: Stripe.Checkout.Session,
  lines: Stripe.ApiList<Stripe.LineItem> | undefined,
) {
  const line = lines?.data?.[0];
  return Boolean(
    line &&
      lines?.data.length === 1 &&
      line.price?.id === order.priceId &&
      line.quantity === order.quantity &&
      line.price.unit_amount === order.expectedUnitAmount &&
      line.price.billing_scheme === 'per_unit' &&
      Number(line.price.unit_amount_decimal ?? line.price.unit_amount) === order.expectedUnitAmount &&
      line.currency.toLowerCase() === order.currency &&
      line.amount_total === order.expectedTotalAmount &&
      session.amount_subtotal === order.expectedTotalAmount &&
      session.amount_total === order.expectedTotalAmount &&
      session.currency?.toLowerCase() === order.currency &&
      objectId(session.customer) === order.stripeCustomerId &&
      session.metadata?.order_id === order.id &&
      session.metadata?.user_id === order.userId &&
      session.metadata?.price_id === order.priceId &&
      session.metadata?.billing_kind === order.kind &&
      session.metadata?.credits === String(order.credits) &&
      (order.kind !== 'subscription' || session.metadata?.tier === order.tier) &&
      session.client_reference_id === order.userId &&
      session.mode === (order.kind === 'subscription' ? 'subscription' : 'payment'),
  );
}

async function completeBillingOrder(
  database: BillingDb,
  order: typeof billingOrders.$inferSelect,
  grantRef: string,
  stripeEventId: string,
  stripeEvidence: { sessionId?: string; sessionUrl?: string | null; paymentIntentId?: string | null } = {},
) {
  await lockBillingUser(database, order.userId);
  const [storedOrder] = await database.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
  if (!storedOrder || storedOrder.state === 'completed') return { state: 'processed' as const };
  const currentOrder = { ...storedOrder, ...stripeEvidence };
  const [terms] = await database.select().from(billingPriceTerms).where(eq(billingPriceTerms.id, order.priceTermsId)).limit(1);
  if (
    !terms ||
    terms.priceId !== currentOrder.priceId ||
    terms.credits !== currentOrder.credits ||
    terms.kind !== currentOrder.kind ||
    terms.tier !== currentOrder.tier ||
    terms.unitAmount !== currentOrder.expectedUnitAmount ||
    terms.currency !== currentOrder.currency ||
    terms.quantity !== currentOrder.quantity ||
    terms.interval !== currentOrder.interval ||
    terms.intervalCount !== currentOrder.intervalCount ||
    currentOrder.expectedTotalAmount !== currentOrder.expectedUnitAmount * currentOrder.quantity
  ) {
    await updateBillingOrderCAS(database, {
      userId: currentOrder.userId,
      orderId: currentOrder.id,
      changes: { state: 'needs_reconciliation', ...stripeEvidence },
    });
    return {
      state: 'needs_reconciliation' as const,
      reason: 'order_terms_mismatch',
    };
  }
  const reason = currentOrder.kind === 'top_up' ? 'stripe_top_up' : 'stripe_checkout_subscription';
  await grantPaidCredits(database, currentOrder.userId, currentOrder.credits, reason, grantRef);
  if (currentOrder.state === 'failed') {
    const [retry] = await database
      .select()
      .from(billingRetryOperations)
      .where(eq(billingRetryOperations.priorOrderId, currentOrder.id))
      .limit(1);
    const otherOrders = await database
      .select({ id: billingOrders.id })
      .from(billingOrders)
      .where(eq(billingOrders.userId, currentOrder.userId));
    const replacementOrderId = retry?.replacementOrderId ?? otherOrders.find((candidate) => candidate.id !== currentOrder.id)?.id ?? null;
    if (replacementOrderId)
      await database
        .insert(billingDuplicateChargeConflicts)
        .values({
          originalOrderId: currentOrder.id,
          replacementOrderId,
          stripeObjectId: currentOrder.paymentIntentId ?? grantRef,
          objectKind: currentOrder.paymentIntentId ? 'payment_intent' : 'checkout_session',
          stripeEventId,
        })
        .onConflictDoNothing();
  }
  if (currentOrder.kind === 'subscription') {
    await updateUserPlan(database, currentOrder.userId, currentOrder.tier ?? terms.tier ?? 'pro');
  }
  const completed = await updateBillingOrderCAS(database, {
    userId: currentOrder.userId,
    orderId: currentOrder.id,
    changes: { ...stripeEvidence, state: 'completed', leaseExpiresAt: null },
  });
  if (!completed) return { state: 'processed' as const };
  await database
    .update(billingRetryOperations)
    .set({ state: 'superseded', leaseExpiresAt: null, updatedAt: new Date() })
    .where(
      and(
        eq(billingRetryOperations.priorOrderId, currentOrder.id),
        sql`${billingRetryOperations.state} in ('cancel_pending', 'replacement_ready', 'checkout_creating')`,
      ),
    );
  return { state: 'processed' as const };
}

async function ensureSubscriptionPriceVersion(
  database: BillingDb,
  input: {
    order: typeof billingOrders.$inferSelect;
    terms: typeof billingPriceTerms.$inferSelect;
    subscription: Stripe.Subscription;
    eventId: string;
    effectiveFrom: Date;
  },
) {
  if (!input.terms.tier || !input.terms.interval || !input.terms.intervalCount) return false;
  const [existing] = await database
    .select()
    .from(billingSubscriptionPriceVersions)
    .where(
      and(
        eq(billingSubscriptionPriceVersions.subscriptionId, input.subscription.id),
        eq(billingSubscriptionPriceVersions.effectiveFrom, input.effectiveFrom),
      ),
    )
    .limit(1);
  if (existing) return existing.priceTermsId === input.terms.id;
  await database
    .insert(billingSubscriptionPriceVersions)
    .values({
      subscriptionId: input.subscription.id,
      userId: input.order.userId,
      priceTermsId: input.terms.id,
      priceId: input.terms.priceId,
      tier: input.terms.tier,
      credits: input.terms.credits,
      interval: input.terms.interval,
      intervalCount: input.terms.intervalCount,
      unitAmount: input.terms.unitAmount,
      currency: input.terms.currency,
      quantity: input.terms.quantity,
      effectiveFrom: input.effectiveFrom,
      sourceOrderId: input.order.id,
      sourceEventId: input.eventId,
    })
    .onConflictDoNothing();
  return true;
}

async function handleCheckoutCompleted(database: BillingDb, prepared: PreparedStripeEvent): Promise<EventOutcome> {
  const session = prepared.session;
  if (!session)
    return {
      state: 'needs_reconciliation',
      reason: 'checkout_session_missing',
    };
  const order = await orderForCheckout(database, session);
  if (!order)
    return {
      state: 'needs_reconciliation',
      reason: 'paid_checkout_without_order',
    };
  await lockBillingUser(database, order.userId);
  const [storedOrder] = await database.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
  if (!storedOrder || storedOrder.state === 'completed') return { state: 'processed' };
  const paymentIntentId = objectId(session.payment_intent);
  if (
    (storedOrder.sessionId && storedOrder.sessionId !== session.id) ||
    (storedOrder.paymentIntentId && storedOrder.paymentIntentId !== paymentIntentId)
  ) {
    return reconcileBillingOrder(database, storedOrder, 'checkout_stripe_id_mismatch');
  }
  const stripeEvidence = { sessionId: session.id, sessionUrl: session.url, paymentIntentId };
  const currentOrder = await updateBillingOrderCAS(database, {
    userId: storedOrder.userId,
    orderId: storedOrder.id,
    changes: stripeEvidence,
  });
  if (!currentOrder) return { state: 'processed' };
  if (!checkoutTermsMatch(currentOrder, session, prepared.lineItems)) {
    return reconcileBillingOrder(database, currentOrder, 'checkout_order_mismatch');
  }
  if (session.payment_status !== 'paid') {
    const intent = prepared.paymentIntent;
    const state =
      session.status === 'open'
        ? 'open'
        : intent?.status === 'requires_payment_method'
          ? 'retry_required'
          : intent?.status === 'canceled'
            ? 'failed'
            : session.status === 'expired' && !intent
              ? 'expired'
              : 'processing';
    await updateBillingOrderCAS(database, {
      userId: currentOrder.userId,
      orderId: currentOrder.id,
      changes: { state, paymentIntentId: intent?.id ?? currentOrder.paymentIntentId },
    });
    return { state: 'processed' };
  }
  if (currentOrder.kind === 'top_up') {
    const intent = prepared.paymentIntent;
    if (
      !intent ||
      intent.status !== 'succeeded' ||
      intent.amount_received !== currentOrder.expectedTotalAmount ||
      intent.currency.toLowerCase() !== currentOrder.currency ||
      objectId(intent.customer) !== currentOrder.stripeCustomerId ||
      metadataOf(intent).order_id !== currentOrder.id ||
      metadataOf(intent).user_id !== currentOrder.userId ||
      metadataOf(intent).price_id !== currentOrder.priceId ||
      metadataOf(intent).billing_kind !== currentOrder.kind ||
      metadataOf(intent).credits !== String(currentOrder.credits)
    ) {
      return reconcileBillingOrder(database, currentOrder, 'checkout_payment_intent_mismatch');
    }
    return completeBillingOrder(database, currentOrder, intent.id, prepared.event.id, stripeEvidence);
  }
  const subscription = prepared.subscription;
  if (
    !subscription ||
    subscription.id !== objectId(session.subscription) ||
    subscriptionPriceId(subscription) !== currentOrder.priceId ||
    metadataOf(subscription).order_id !== currentOrder.id ||
    metadataOf(subscription).user_id !== currentOrder.userId
  ) {
    return reconcileBillingOrder(database, currentOrder, 'checkout_subscription_mismatch');
  }
  const [terms] = await database.select().from(billingPriceTerms).where(eq(billingPriceTerms.id, currentOrder.priceTermsId)).limit(1);
  if (
    !terms ||
    !(await ensureSubscriptionPriceVersion(database, {
      order: currentOrder,
      terms,
      subscription,
      eventId: prepared.event.id,
      effectiveFrom: unixSecondsToDate(
        (subscription as any).items?.data?.[0]?.current_period_start ?? (subscription as any).start_date ?? session.created,
      ),
    }))
  )
    return reconcileBillingOrder(database, currentOrder, 'subscription_terms_unavailable');
  await upsertSubscription(database, {
    userId: currentOrder.userId,
    stripeSubId: subscription.id,
    tier: terms.tier ?? currentOrder.tier ?? 'pro',
    status: subscription.status,
    currentPeriodEnd: subscriptionCurrentPeriodEnd(subscription),
  });
  return completeBillingOrder(database, currentOrder, session.id, prepared.event.id, stripeEvidence);
}

function intervalEnd(start: Date, interval: string, count: number): Date {
  const end = new Date(start);
  if (interval === 'day') end.setUTCDate(end.getUTCDate() + count);
  else if (interval === 'week') end.setUTCDate(end.getUTCDate() + count * 7);
  else if (interval === 'month' || interval === 'year') {
    const day = end.getUTCDate();
    if (interval === 'month') end.setUTCMonth(end.getUTCMonth() + count, 1);
    else end.setUTCFullYear(end.getUTCFullYear() + count, end.getUTCMonth(), 1);
    const lastDay = new Date(Date.UTC(end.getUTCFullYear(), end.getUTCMonth() + 1, 0)).getUTCDate();
    end.setUTCDate(Math.min(day, lastDay));
  }
  return end;
}

async function handleInvoicePaid(database: BillingDb, prepared: PreparedStripeEvent): Promise<EventOutcome> {
  const invoice = prepared.invoice;
  if (!invoice) return { state: 'needs_reconciliation', reason: 'invoice_missing' };
  if ((invoice as any).billing_reason !== 'subscription_cycle') return { state: 'processed' };
  const subscriptionId = invoiceSubscriptionId(invoice);
  if (!subscriptionId)
    return {
      state: 'needs_reconciliation',
      reason: 'invoice_subscription_missing',
    };
  const lines = (invoice as any).lines?.data ?? [];
  if (lines.length !== 1)
    return {
      state: 'needs_reconciliation',
      reason: 'unsupported_invoice_lines',
    };
  const line = lines[0] as any;
  const invoiceRecord = invoice as any;
  const lineDetails = line.parent?.subscription_item_details ?? line.subscription_item_details ?? {};
  const noAdjustments =
    !lineDetails.proration &&
    !line.proration &&
    !invoiceRecord.total_discount_amounts?.length &&
    !invoiceRecord.total_taxes?.length &&
    !invoiceRecord.default_tax_rates?.length &&
    !invoiceRecord.discounts?.length &&
    !invoiceRecord.customer_tax_ids?.length &&
    !invoiceRecord.amount_shipping &&
    !invoiceRecord.shipping_cost?.amount &&
    invoiceRecord.automatic_tax?.enabled !== true &&
    !invoiceRecord.total_pretax_credit_amounts?.length &&
    !invoiceRecord.pre_payment_credit_notes_amount &&
    !invoiceRecord.post_payment_credit_notes_amount &&
    !invoiceRecord.starting_balance &&
    !invoiceRecord.ending_balance &&
    !line.discount_amounts?.length &&
    !line.discounts?.length &&
    !line.taxes?.length &&
    !line.tax_rates?.length &&
    !line.pretax_credit_amounts?.length &&
    !line.tax_amounts?.length &&
    !line.discount_amount;
  if (!noAdjustments) return { state: 'needs_reconciliation', reason: 'renewal_adjustments_unsupported' };
  if (!Number.isSafeInteger(line.period?.start) || !Number.isSafeInteger(line.period?.end)) {
    return { state: 'needs_reconciliation', reason: 'renewal_period_missing' };
  }
  const periodStart = unixSecondsToDate(line.period.start);
  const periodEnd = unixSecondsToDate(line.period.end);
  const versions = await database
    .select()
    .from(billingSubscriptionPriceVersions)
    .where(eq(billingSubscriptionPriceVersions.subscriptionId, subscriptionId));
  const version = versions
    .filter((candidate) => candidate.effectiveFrom <= periodStart && (!candidate.effectiveTo || candidate.effectiveTo > periodStart))
    .sort((left, right) => right.effectiveFrom.getTime() - left.effectiveFrom.getTime())[0];
  if (!version)
    return {
      state: 'needs_reconciliation',
      reason: 'renewal_terms_unavailable',
    };
  const [owner] = await database
    .select({ stripeCustomerId: users.stripeCustomerId })
    .from(users)
    .where(eq(users.id, version.userId))
    .limit(1);
  if (
    !owner?.stripeCustomerId ||
    objectId((invoice as any).customer) !== owner.stripeCustomerId ||
    (prepared.subscription && prepared.subscription.id !== subscriptionId)
  ) {
    return {
      state: 'needs_reconciliation',
      reason: 'renewal_identity_mismatch',
    };
  }
  const linePriceId = line.price?.id ?? line.pricing?.price_details?.price ?? line.parent?.subscription_item_details?.price;
  const expandedPrice = line.price && typeof line.price === 'object' ? line.price : null;
  const invoiceCurrency = String((invoice as any).currency ?? '').toLowerCase();
  const totalFieldsMatch =
    (invoice as any).amount_due === version.unitAmount &&
    (invoice as any).amount_paid === version.unitAmount &&
    (invoice as any).total === version.unitAmount &&
    invoiceCurrency === version.currency;
  const lineMatches =
    linePriceId === version.priceId &&
    line.quantity === version.quantity &&
    line.amount === version.unitAmount &&
    line.subtotal === version.unitAmount &&
    line.quantity_decimal == null &&
    expandedPrice?.unit_amount === version.unitAmount &&
    expandedPrice.billing_scheme === 'per_unit' &&
    expandedPrice.recurring?.interval === version.interval &&
    expandedPrice.recurring.interval_count === version.intervalCount &&
    expandedPrice.recurring.usage_type !== 'metered' &&
    String(line.currency ?? invoiceCurrency).toLowerCase() === version.currency &&
    periodEnd.getTime() === intervalEnd(periodStart, version.interval, version.intervalCount).getTime();
  if (!totalFieldsMatch || !lineMatches)
    return {
      state: 'needs_reconciliation',
      reason: 'renewal_amount_or_period_mismatch',
    };
  await lockBillingUser(database, version.userId);
  await grantPaidCredits(database, version.userId, version.credits, 'stripe_subscription_renewal', invoice.id);
  if (prepared.subscription)
    await upsertSubscription(database, {
      userId: version.userId,
      stripeSubId: prepared.subscription.id,
      tier: version.tier,
      status: prepared.subscription.status,
      currentPeriodEnd: subscriptionCurrentPeriodEnd(prepared.subscription),
    });
  return { state: 'processed' };
}

async function handlePaymentIntentSucceeded(database: BillingDb, prepared: PreparedStripeEvent): Promise<EventOutcome> {
  const intent = prepared.paymentIntent;
  if (!intent)
    return {
      state: 'needs_reconciliation',
      reason: 'succeeded_payment_missing',
    };
  const metadata = metadataOf(intent);
  if (!metadata.order_id)
    return metadata.billing_kind === 'top_up'
      ? {
          state: 'needs_reconciliation',
          reason: 'top_up_payment_without_order_id',
        }
      : { state: 'processed' };
  const [order] = await database.select().from(billingOrders).where(eq(billingOrders.id, metadata.order_id)).limit(1);
  if (!order)
    return {
      state: 'needs_reconciliation',
      reason: 'succeeded_payment_without_order',
    };
  await lockBillingUser(database, order.userId);
  const [storedOrder] = await database.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
  if (!storedOrder || storedOrder.state === 'completed') return { state: 'processed' };
  if (storedOrder.paymentIntentId && storedOrder.paymentIntentId !== intent.id) {
    return reconcileBillingOrder(database, storedOrder, 'payment_intent_id_mismatch');
  }
  if (prepared.session && storedOrder.sessionId && storedOrder.sessionId !== prepared.session.id) {
    return reconcileBillingOrder(database, storedOrder, 'payment_intent_session_id_mismatch');
  }
  const stripeEvidence = {
    ...(prepared.session ? { sessionId: prepared.session.id, sessionUrl: prepared.session.url } : {}),
    paymentIntentId: intent.id,
  };
  const currentOrder = await updateBillingOrderCAS(database, {
    userId: storedOrder.userId,
    orderId: storedOrder.id,
    changes: stripeEvidence,
  });
  if (!currentOrder) return { state: 'processed' };
  if (
    currentOrder.kind !== 'top_up' ||
    intent.status !== 'succeeded' ||
    intent.amount_received !== currentOrder.expectedTotalAmount ||
    intent.currency.toLowerCase() !== currentOrder.currency ||
    objectId(intent.customer) !== currentOrder.stripeCustomerId ||
    metadata.price_id !== currentOrder.priceId ||
    metadata.credits !== String(currentOrder.credits)
  ) {
    return reconcileBillingOrder(database, currentOrder, 'succeeded_payment_order_mismatch');
  }
  if (
    !prepared.session ||
    !prepared.lineItems ||
    objectId(prepared.session.payment_intent) !== intent.id ||
    prepared.session.payment_status !== 'paid' ||
    !checkoutTermsMatch(currentOrder, prepared.session, prepared.lineItems)
  ) {
    return reconcileBillingOrder(database, currentOrder, 'succeeded_payment_session_unverified');
  }
  return completeBillingOrder(database, currentOrder, intent.id, prepared.event.id, stripeEvidence);
}

async function handleCheckoutFailure(database: BillingDb, prepared: PreparedStripeEvent): Promise<EventOutcome> {
  const session = prepared.session;
  if (!session)
    return {
      state: 'needs_reconciliation',
      reason: 'failed_checkout_session_missing',
    };
  const order = await orderForCheckout(database, session);
  if (!order)
    return {
      state: 'needs_reconciliation',
      reason: 'failed_checkout_without_order',
    };
  await lockBillingUser(database, order.userId);
  const [storedOrder] = await database.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
  if (!storedOrder || storedOrder.state === 'completed') return { state: 'processed' };
  if (
    (storedOrder.sessionId && session.id !== storedOrder.sessionId) ||
    (storedOrder.paymentIntentId && objectId(session.payment_intent) !== storedOrder.paymentIntentId) ||
    session.metadata?.order_id !== storedOrder.id ||
    objectId(session.customer) !== storedOrder.stripeCustomerId
  ) {
    return reconcileBillingOrder(database, storedOrder, 'failed_checkout_order_mismatch');
  }
  const intent = prepared.paymentIntent;
  const signed = prepared.signedSession;
  const nextState = classifyAsyncPaymentFailure({ signed, current: session, intent, eventType: prepared.event.type });
  if (nextState === 'needs_reconciliation') {
    return reconcileBillingOrder(database, storedOrder, 'async_payment_failure_snapshot_conflict');
  }
  const currentOrder = await updateBillingOrderCAS(database, {
    userId: storedOrder.userId,
    orderId: storedOrder.id,
    changes: { sessionId: session.id, sessionUrl: session.url, paymentIntentId: intent?.id ?? storedOrder.paymentIntentId },
  });
  if (!currentOrder) return { state: 'processed' };
  if (session.payment_status === 'paid') {
    return reconcileBillingOrder(database, currentOrder, 'checkout_failure_event_conflicts_with_paid_session');
  }
  if (prepared.event.type === 'checkout.session.expired' && session.status !== 'expired') {
    return reconcileBillingOrder(database, currentOrder, 'expired_event_conflicts_with_session');
  }
  if (intent && (objectId(intent.customer) !== currentOrder.stripeCustomerId || metadataOf(intent).order_id !== currentOrder.id)) {
    return reconcileBillingOrder(database, currentOrder, 'failure_payment_intent_mismatch');
  }
  await updateBillingOrderCAS(database, { userId: currentOrder.userId, orderId: currentOrder.id, changes: { state: nextState } });
  return { state: 'processed' };
}

export function classifyAsyncPaymentFailure(input: {
  signed?: Stripe.Checkout.Session;
  current: Stripe.Checkout.Session;
  intent?: Stripe.PaymentIntent;
  eventType?: Stripe.Event['type'];
}): 'open' | 'retry_required' | 'failed' | 'expired' | 'processing' | 'needs_reconciliation' {
  const { signed, current, intent } = input;
  if (
    signed &&
    ((signed.status != null && signed.status !== current.status) ||
      (signed.payment_status != null && signed.payment_status !== current.payment_status) ||
      (objectId(signed.payment_intent) && objectId(signed.payment_intent) !== objectId(current.payment_intent)) ||
      (typeof signed.payment_intent === 'object' &&
        intent &&
        ((signed.payment_intent as Stripe.PaymentIntent).status !== intent.status ||
          (signed.payment_intent as Stripe.PaymentIntent).amount_received !== intent.amount_received ||
          (signed.payment_intent as Stripe.PaymentIntent).currency !== intent.currency)))
  )
    return 'needs_reconciliation';
  if (input.eventType === 'checkout.session.async_payment_failed' && intent?.status === 'succeeded') return 'needs_reconciliation';
  if (current.payment_status === 'paid') return 'needs_reconciliation';
  if (current.status === 'open') return 'open';
  if (current.status === 'complete' && intent?.status === 'requires_payment_method') return 'retry_required';
  if (current.status === 'complete' && intent?.status === 'canceled') return 'failed';
  if (current.status === 'expired' && (!intent || intent.status === 'canceled')) return 'expired';
  if (current.status === 'expired' && intent?.status === 'requires_payment_method') return 'retry_required';
  if (current.status === 'complete' || current.status === 'expired') return 'processing';
  return 'needs_reconciliation';
}

type StoredSubscriptionVersion = {
  status: string;
  lastEventCreatedAt: Date | null;
};

function isStaleSubscriptionStatusEvent(
  incomingCreatedAt: Date,
  eventType: Stripe.Event['type'],
  existing: StoredSubscriptionVersion,
): boolean {
  if (!existing.lastEventCreatedAt) return false;
  const incoming = incomingCreatedAt.getTime();
  const stored = existing.lastEventCreatedAt.getTime();
  if (incoming > stored) return false;
  if (incoming < stored) return true;
  if (eventType === 'customer.subscription.deleted') return false;
  return !isActiveBillingSubscriptionStatus(existing.status);
}

async function handleSubscriptionStatus(
  database: BillingDb,
  event: Stripe.Event,
  subscription: Stripe.Subscription,
): Promise<EventOutcome> {
  const customerId = objectId(subscription.customer);
  const user = await userByStripeCustomer(database, customerId);
  if (!user)
    return {
      state: 'needs_reconciliation',
      reason: 'subscription_customer_unmapped',
    };
  await lockBillingUser(database, user.id);
  const [existing] = await database
    .select({
      stripeSubId: subscriptions.stripeSubId,
      tier: subscriptions.tier,
      status: subscriptions.status,
      lastEventCreatedAt: subscriptions.lastEventCreatedAt,
    })
    .from(subscriptions)
    .where(eq(subscriptions.userId, user.id))
    .limit(1);
  if (existing && existing.stripeSubId !== subscription.id && isActiveBillingSubscriptionStatus(existing.status))
    return {
      state: 'needs_reconciliation',
      reason: 'multiple_active_subscriptions',
    };

  const incomingCreatedAt = unixSecondsToDate(event.created);
  if (existing && existing.stripeSubId === subscription.id && isStaleSubscriptionStatusEvent(incomingCreatedAt, event.type, existing)) {
    log.info('stripe_subscription_event_stale', {
      user_id: user.id,
      stripe_event_id: event.id,
      stripe_event_type: event.type,
      stripe_subscription_id: subscription.id,
    });
    return { state: 'processed' };
  }

  const priceId = subscriptionPriceId(subscription);
  let tier = existing?.tier ?? (user.plan !== 'free' ? user.plan : 'pro');
  if (event.type === 'customer.subscription.updated') {
    const termsRows = priceId ? await database.select().from(billingPriceTerms).where(eq(billingPriceTerms.priceId, priceId)) : [];
    const terms = termsRows
      .filter((row) => row.kind === 'subscription' && row.approvedAt < incomingCreatedAt)
      .sort((left, right) => right.approvedAt.getTime() - left.approvedAt.getTime() || right.version - left.version)[0];
    const observedItem = (subscription as any).items?.data?.[0] as any;
    const observedPrice = observedItem?.price;
    const factsMatch =
      terms &&
      (subscription as any).items?.data?.length === 1 &&
      observedItem?.quantity === terms.quantity &&
      (observedPrice?.id ?? observedItem?.pricing?.price_details?.price) === terms.priceId &&
      observedPrice?.unit_amount === terms.unitAmount &&
      String(observedPrice?.currency ?? '').toLowerCase() === terms.currency &&
      observedPrice?.billing_scheme === 'per_unit' &&
      observedPrice?.recurring?.interval === terms.interval &&
      observedPrice?.recurring?.interval_count === terms.intervalCount &&
      observedPrice?.recurring?.usage_type !== 'metered';
    if (!terms?.tier || !terms.interval || !terms.intervalCount || !factsMatch)
      return {
        state: 'needs_reconciliation',
        reason: 'subscription_price_facts_unapproved',
      };
    tier = terms.tier;
    const existingVersions = await database
      .select()
      .from(billingSubscriptionPriceVersions)
      .where(eq(billingSubscriptionPriceVersions.subscriptionId, subscription.id));
    const latestVersion = existingVersions.sort((left, right) => right.effectiveFrom.getTime() - left.effectiveFrom.getTime())[0];
    let effectiveFrom = unixSecondsToDate((subscription as any).items?.data?.[0]?.current_period_start ?? event.created);
    if (latestVersion && effectiveFrom <= latestVersion.effectiveFrom) effectiveFrom = new Date(incomingCreatedAt.getTime() + 1);
    if (!latestVersion || latestVersion.priceTermsId !== terms.id) {
      if (latestVersion)
        await database
          .update(billingSubscriptionPriceVersions)
          .set({ effectiveTo: effectiveFrom })
          .where(eq(billingSubscriptionPriceVersions.id, latestVersion.id));
      await database
        .insert(billingSubscriptionPriceVersions)
        .values({
          subscriptionId: subscription.id,
          userId: user.id,
          priceTermsId: terms.id,
          priceId: terms.priceId,
          tier: terms.tier,
          credits: terms.credits,
          interval: terms.interval,
          intervalCount: terms.intervalCount,
          unitAmount: terms.unitAmount,
          currency: terms.currency,
          quantity: terms.quantity,
          effectiveFrom,
          sourceEventId: event.id,
        })
        .onConflictDoNothing();
    }
  } else if (event.type !== 'customer.subscription.deleted') {
    return { state: 'processed' };
  }
  await upsertSubscription(database, {
    userId: user.id,
    stripeSubId: subscription.id,
    tier,
    status: subscription.status,
    currentPeriodEnd: subscriptionCurrentPeriodEnd(subscription),
    lastEventCreatedAt: incomingCreatedAt,
    lastEventId: event.id,
  });
  return { state: 'processed' };
}

function invoiceSubscriptionId(invoice: Stripe.Invoice): string | null {
  return objectId((invoice as any).subscription) ?? objectId((invoice as any).parent?.subscription_details?.subscription);
}

async function prepareStripeEvent(event: Stripe.Event): Promise<PreparedStripeEvent> {
  const stripe = stripeClient();
  if (
    event.type === 'checkout.session.completed' ||
    event.type === 'checkout.session.async_payment_succeeded' ||
    event.type === 'checkout.session.async_payment_failed' ||
    event.type === 'checkout.session.expired'
  ) {
    const eventSession = event.data.object as Stripe.Checkout.Session;
    const session = await stripe.checkout.sessions.retrieve(eventSession.id);
    const paymentIntentId = objectId(session.payment_intent);
    const subscriptionId = objectId(session.subscription);
    const paymentIntent = paymentIntentId ? await stripe.paymentIntents.retrieve(paymentIntentId) : undefined;
    const lineItems =
      event.type === 'checkout.session.completed' || event.type === 'checkout.session.async_payment_succeeded'
        ? await stripe.checkout.sessions.listLineItems(session.id, {
            limit: 100,
          })
        : undefined;
    return {
      event,
      session,
      signedSession: eventSession,
      paymentIntent,
      lineItems,
      subscription: subscriptionId ? await retrieveSubscription(subscriptionId) : null,
    };
  }
  if (event.type === 'invoice.paid') {
    const eventInvoice = event.data.object as Stripe.Invoice;
    const invoice = await stripe.invoices.retrieve(eventInvoice.id, {
      expand: ['lines.data.price'],
    });
    const subscriptionId = invoiceSubscriptionId(invoice);
    return {
      event,
      invoice,
      subscription: subscriptionId ? await retrieveSubscription(subscriptionId) : null,
    };
  }
  if (event.type === 'payment_intent.succeeded') {
    const eventIntent = event.data.object as Stripe.PaymentIntent;
    const paymentIntent = await stripe.paymentIntents.retrieve(eventIntent.id);
    const orderId = metadataOf(paymentIntent).order_id;
    if (!orderId) return { event, paymentIntent, subscription: null };
    const [order] = await db.select().from(billingOrders).where(eq(billingOrders.id, orderId)).limit(1);
    if (!order) return { event, paymentIntent, subscription: null };
    let sessionId = order.sessionId;
    if (!sessionId) {
      let startingAfter: string | undefined;
      let terminal = false;
      const matches: Stripe.Checkout.Session[] = [];
      for (let pageNumber = 0; pageNumber < 3; pageNumber += 1) {
        const page = await stripe.checkout.sessions.list({
          customer: order.stripeCustomerId,
          created: { gte: Math.floor(order.firstAttemptAt.getTime() / 1000) },
          limit: 100,
          ...(startingAfter ? { starting_after: startingAfter } : {}),
        });
        matches.push(...page.data.filter((candidate) => candidate.metadata?.order_id === order.id));
        if (!page.has_more) {
          terminal = true;
          break;
        }
        startingAfter = page.data.at(-1)?.id;
        if (!startingAfter) break;
      }
      if (!terminal || matches.length !== 1) return { event, paymentIntent, subscription: null };
      sessionId = matches[0].id;
    }
    const session = await stripe.checkout.sessions.retrieve(sessionId);
    const lineItems = await stripe.checkout.sessions.listLineItems(session.id, {
      limit: 100,
    });
    return { event, paymentIntent, session, lineItems, subscription: null };
  }
  if (event.type === 'customer.subscription.deleted' || event.type === 'customer.subscription.updated') {
    const eventSubscription = event.data.object as Stripe.Subscription;
    return {
      event,
      subscription: await retrieveSubscription(eventSubscription.id),
    };
  }
  return { event, subscription: null };
}

async function handleStripeEvent(database: BillingDb, prepared: PreparedStripeEvent): Promise<EventOutcome> {
  switch (prepared.event.type) {
    case 'checkout.session.completed':
    case 'checkout.session.async_payment_succeeded':
      return handleCheckoutCompleted(database, prepared);
    case 'checkout.session.async_payment_failed':
    case 'checkout.session.expired':
      return handleCheckoutFailure(database, prepared);
    case 'invoice.paid':
      return handleInvoicePaid(database, prepared);
    case 'payment_intent.succeeded':
      return handlePaymentIntentSucceeded(database, prepared);
    case 'customer.subscription.deleted':
    case 'customer.subscription.updated':
      if (!prepared.subscription)
        return {
          state: 'needs_reconciliation',
          reason: 'subscription_retrieval_missing',
        };
      return handleSubscriptionStatus(database, prepared.event, prepared.subscription);
    default:
      return { state: 'processed' };
  }
}

export async function processStripeEvent(event: Stripe.Event): Promise<{ processed: boolean; needsReconciliation?: boolean }> {
  const [stored] = await db.select({ state: stripeEvents.state }).from(stripeEvents).where(eq(stripeEvents.eventId, event.id)).limit(1);
  if (stored)
    return {
      processed: false,
      ...(stored.state === 'needs_reconciliation' ? { needsReconciliation: true } : {}),
    };
  // All Stripe GETs happen before the transaction so DB locks are never held
  // across provider latency. Any retrieval failure leaves the event retryable.
  const prepared = await prepareStripeEvent(event);
  return db.transaction(async (tx) => {
    const database = tx as unknown as BillingDb;
    const inserted = await database
      .insert(stripeEvents)
      .values({
        eventId: event.id,
        type: event.type,
        apiVersion: event.api_version ?? null,
        payload: event as any,
      })
      .onConflictDoNothing({ target: stripeEvents.eventId })
      .returning({ eventId: stripeEvents.eventId });

    if (inserted.length === 0) return { processed: false };
    const outcome = await handleStripeEvent(database, prepared);
    await database
      .update(stripeEvents)
      .set({
        state: outcome.state,
        reconciliationReason: outcome.reason ?? null,
        processedAt: new Date(),
      })
      .where(eq(stripeEvents.eventId, event.id));
    log.info(outcome.state === 'processed' ? 'stripe_event_processed' : 'stripe_event_needs_reconciliation', {
      stripe_event_id: prepared.event.id,
      stripe_event_type: prepared.event.type,
      reason: outcome.reason,
    });
    return {
      processed: true,
      ...(outcome.state === 'needs_reconciliation' ? { needsReconciliation: true } : {}),
    };
  });
}

export function constructStripeWebhookEvent(payload: string, signature: string | null): Stripe.Event {
  const webhookSecret = env('STRIPE_WEBHOOK_SECRET');
  if (!webhookSecret) throw new BillingConfigError('STRIPE_WEBHOOK_SECRET is not configured');
  if (!signature) throw new Error('Missing stripe-signature header');
  return stripeClient().webhooks.constructEvent(payload, signature, webhookSecret);
}

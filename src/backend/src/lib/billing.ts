import Stripe from 'stripe';
import { and, eq, isNull, or, sql } from 'drizzle-orm';
import { createHash, randomUUID } from 'node:crypto';
import { db } from '../db/client';
import { billingOrders, billingPriceTerms, billingRetryOperations, creditLedger, stripeCustomerAttempts, stripeEvents, subscriptions, users } from '../db/schema';
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

type StripeLike = Pick<Stripe, 'checkout' | 'billingPortal' | 'customers' | 'subscriptions' | 'prices' | 'paymentIntents' | 'webhooks'>;

let stripeOverride: StripeLike | null = null;

export class BillingConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'BillingConfigError';
  }
}

export class BillingEligibilityError extends Error {
  constructor(public type: 'top_up_requires_subscription' | 'already_subscribed', message: string) {
    super(message);
    this.name = 'BillingEligibilityError';
  }
}

export class BillingRequestError extends Error {
  constructor(public type: 'unknown_price' | 'idempotency_conflict', message: string) {
    super(message);
    this.name = 'BillingRequestError';
  }
}

export class BillingPendingError extends Error {
  constructor(public status: 'processing' | 'retry_required' | 'completed' | 'expired' | 'failed' | 'reconciliation_required') {
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
  const currency = String(raw.currency ?? '').trim().toUpperCase();
  if (!Number.isSafeInteger(amount) || amount <= 0 || !/^[A-Z]{3}$/.test(currency)) return null;

  const interval = String(raw.interval ?? raw.recurringInterval ?? raw.recurring_interval ?? '').trim().toLowerCase();
  if (!requiresInterval) return { amount, currency };
  if (interval !== 'day' && interval !== 'week' && interval !== 'month' && interval !== 'year') return null;
  return { amount, currency, interval };
}

export function formatBillingPrice(price: BillingPriceDisplay): string | null {
  try {
    const formatter = new Intl.NumberFormat('en', { style: 'currency', currency: price.currency });
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
    const rows = Array.isArray(parsed) ? parsed : Object.entries(parsed).map(([key, val]) => ({ ...(val as object), priceId: key }));
    return rows.map(normalize).filter((row): row is T => Boolean(row));
  } catch (error) {
    throw new BillingConfigError(`${name} must be valid JSON`);
  }
}

export function billingCatalog(): BillingCatalog {
  const subscriptionsFromJson = parseCatalogArray<SubscriptionTier>('STRIPE_SUBSCRIPTION_TIERS', (raw) => {
    const priceId = String(raw.priceId ?? raw.price_id ?? '').trim();
    const tier = String(raw.tier ?? raw.plan ?? '').trim();
    const credits = parsePositiveInt(String(raw.credits ?? ''), 0);
    const displayPrice = parseDisplayPrice(raw, true);
    return priceId && tier && credits ? {
      kind: 'subscription',
      priceId,
      tier,
      credits,
      ...(displayPrice?.interval ? { displayPrice: { ...displayPrice, interval: displayPrice.interval } } : {}),
    } : null;
  });
  const topUpsFromJson = parseCatalogArray<TopUpPack>('STRIPE_TOP_UP_PACKS', (raw) => {
    const priceId = String(raw.priceId ?? raw.price_id ?? '').trim();
    const credits = parsePositiveInt(String(raw.credits ?? ''), 0);
    const displayPrice = parseDisplayPrice(raw, false);
    return priceId && credits ? {
      kind: 'top_up',
      priceId,
      credits,
      ...(displayPrice ? { displayPrice } : {}),
    } : null;
  });

  const fallbackSubscriptionPriceId = env('STRIPE_SUBSCRIPTION_PRICE_ID');
  const fallbackTopUpPriceId = env('STRIPE_TOP_UP_PRICE_ID');
  const fallbackSubscriptionCredits = parsePositiveInt(env('STRIPE_SUBSCRIPTION_CREDITS'), 0);
  const fallbackTopUpCredits = parsePositiveInt(env('STRIPE_TOP_UP_CREDITS'), 0);
  const fallbackSubscriptions: SubscriptionTier[] = fallbackSubscriptionPriceId && fallbackSubscriptionCredits > 0 ? [{
    kind: 'subscription',
    priceId: fallbackSubscriptionPriceId,
    tier: env('STRIPE_SUBSCRIPTION_TIER') ?? 'pro',
    credits: fallbackSubscriptionCredits,
  }] : [];
  const fallbackTopUps: TopUpPack[] = fallbackTopUpPriceId && fallbackTopUpCredits > 0 ? [{
    kind: 'top_up',
    priceId: fallbackTopUpPriceId,
    credits: fallbackTopUpCredits,
  }] : [];
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

function findItemByPriceId(priceId: string | null | undefined): BillingCatalogItem | null {
  if (!priceId) return null;
  const catalog = billingCatalog();
  return [...catalog.subscriptions, ...catalog.topUps].find((item) => item.priceId === priceId) ?? null;
}

function publicUrl(path: string): string {
  try {
    const configured = env('VERCEL_ENV') === 'preview'
      ? (env('VERCEL_URL') ? `https://${env('VERCEL_URL')}` : '')
      : env('VERCEL_ENV') === 'production' || env('NODE_ENV') === 'production'
        ? productionWebOrigin()
        : env('PUBLIC_DASHBOARD_URL')?.match(/^https?:\/\/localhost(?::\d+)?\/?$/)
          ? env('PUBLIC_DASHBOARD_URL')!
          : productionWebOrigin();
    const policy = env('VERCEL_ENV') === 'preview' ? 'preview' : configured.startsWith('http://localhost') ? 'local' : 'production_web';
    const base = policy === 'preview'
      ? parsePublicOrigin(configured, 'preview', configured)
      : parsePublicOrigin(configured, policy);
    return new URL(path, `${base}/`).toString();
  } catch {
    throw new BillingConfigError('Public web origin configuration is invalid');
  }
}

export function rejectCrossOriginBillingPost(request: Request): Response | null {
  const origin = request.headers.get('origin');
  if (!origin) return null;
  if (origin === new URL(request.url).origin) return null;
  return Response.json({ error: { type: 'forbidden', message: 'Cross-origin billing request rejected' } }, { status: 403 });
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
      return { kind: 'owner' as const, key: idempotencyKey, payload: customerPayload };
    }
    if (attempt.status === 'linked' && attempt.customerId) return { kind: 'linked' as const, id: attempt.customerId };
    if (attempt.status === 'creating' && attempt.leaseExpiresAt && attempt.leaseExpiresAt > new Date()) return { kind: 'pending' as const };
    if (Date.now() - attempt.createdAt.getTime() >= 23 * 60 * 60 * 1000 || attempt.status === 'reconciliation_required') {
      await tx.update(stripeCustomerAttempts).set({ status: 'reconciliation_required', leaseExpiresAt: null, updatedAt: new Date() })
        .where(eq(stripeCustomerAttempts.userId, user.id));
      return { kind: 'lookup' as const, payload: attempt.payload as typeof customerPayload };
    }
    const storedPayload = attempt.payload as typeof customerPayload;
    await tx.update(stripeCustomerAttempts).set({
      status: 'creating', leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS), updatedAt: new Date(),
    }).where(eq(stripeCustomerAttempts.userId, user.id));
    return { kind: 'owner' as const, key: attempt.idempotencyKey, payload: storedPayload };
  });
  if (claimed.kind === 'linked') return claimed.id;
  if (claimed.kind === 'pending') throw new BillingPendingError('processing');
  if (claimed.kind === 'lookup') {
    const customers = await stripe.customers.list({ email: claimed.payload.email, limit: 100 });
    const matches = customers.data.filter((candidate) => candidate.metadata.user_id === user.id);
    if (customers.has_more || matches.length !== 1) throw new BillingPendingError('reconciliation_required');
    const customer = matches[0];
    await db.transaction(async (tx) => {
      await tx.execute(sql`SELECT id FROM users WHERE id = ${user.id} FOR UPDATE`);
      await tx.update(users).set({ stripeCustomerId: customer.id }).where(and(eq(users.id, user.id), isNull(users.stripeCustomerId)));
      await tx.update(stripeCustomerAttempts).set({ customerId: customer.id, status: 'linked', updatedAt: new Date() })
        .where(eq(stripeCustomerAttempts.userId, user.id));
    });
    return customer.id;
  }
  try {
    const customer = await stripe.customers.create(claimed.payload, { idempotencyKey: claimed.key, timeout: STRIPE_REQUEST_TIMEOUT_MS });
    await db.transaction(async (tx) => {
      await tx.execute(sql`SELECT id FROM users WHERE id = ${user.id} FOR UPDATE`);
      await tx.update(users).set({ stripeCustomerId: customer.id }).where(and(eq(users.id, user.id), isNull(users.stripeCustomerId)));
      await tx.update(stripeCustomerAttempts).set({ customerId: customer.id, status: 'linked', leaseExpiresAt: null, updatedAt: new Date() })
        .where(eq(stripeCustomerAttempts.userId, user.id));
    });
    return customer.id;
  } catch (error) {
    await db.update(stripeCustomerAttempts).set({
      status: 'ambiguous', leaseExpiresAt: null, lastError: String(error).slice(0, 500), updatedAt: new Date(),
    }).where(eq(stripeCustomerAttempts.userId, user.id));
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

export async function createBillingCheckout(input: { user: typeof users.$inferSelect; kind: BillingKind; priceId?: string; tier?: string; attemptKey?: string }) {
  const cancelUrl = publicUrl('/profile?billing=canceled');
  const item = findCatalogItem(input);
  if (!item) {
    if (input.priceId || input.tier) throw new BillingRequestError('unknown_price', 'Unknown Stripe billing price.');
    throw new BillingConfigError(`No Stripe ${input.kind} price is configured`);
  }
  // Fail closed until an owner-approved, versioned price snapshot exists.
  // Retrieve Stripe facts before creating a Customer or entering any DB tx.
  const [terms] = await db.select().from(billingPriceTerms).where(eq(billingPriceTerms.priceId, item.priceId)).limit(1);
  if (!terms || terms.kind !== item.kind || terms.credits !== item.credits ||
      terms.tier !== (item.kind === 'subscription' ? item.tier : null) || terms.quantity !== 1 ||
      !item.displayPrice || terms.unitAmount !== item.displayPrice.amount ||
      terms.currency !== item.displayPrice.currency.toLowerCase() ||
      terms.interval !== (item.kind === 'subscription' ? item.displayPrice.interval : null) ||
      terms.intervalCount !== (item.kind === 'subscription' ? 1 : null)) {
    throw new BillingRequestError('unknown_price', 'This Stripe price has no matching approved purchase terms. Checkout is unavailable until the price is reviewed.');
  }
  const verifiedPrice = await stripeClient().prices.retrieve(item.priceId);
  const recurringInterval = verifiedPrice.recurring?.interval ?? null;
  if (!verifiedPrice.active || verifiedPrice.billing_scheme !== 'per_unit' || verifiedPrice.unit_amount === null ||
      verifiedPrice.unit_amount !== terms.unitAmount || verifiedPrice.currency.toLowerCase() !== terms.currency ||
      (verifiedPrice.type === 'recurring') !== (terms.kind === 'subscription') ||
      verifiedPrice.recurring?.interval !== terms.interval ||
      (verifiedPrice.recurring?.interval_count ?? null) !== terms.intervalCount ||
      recurringInterval !== terms.interval) {
    throw new BillingRequestError('unknown_price', 'The Stripe price no longer matches its approved purchase terms.');
  }
  const hasActiveSubscription = await userHasActiveSubscription(db, input.user.id);
  if (item.kind === 'subscription' && hasActiveSubscription) {
    throw new BillingEligibilityError('already_subscribed', 'You already have an active Pro subscription. Manage it from billing settings.');
  }
  if (item.kind === 'top_up' && !hasActiveSubscription) {
    throw new BillingEligibilityError('top_up_requires_subscription', 'Top-ups are available after subscribing to Pro.');
  }
  const customer = await getOrCreateStripeCustomer(input.user);
  const stripe = stripeClient();
  const attemptKey = input.attemptKey ?? randomUUID();
  const intentHash = createHash('sha256').update(JSON.stringify({ kind: item.kind, priceId: item.priceId, tier: item.kind === 'subscription' ? item.tier : null })).digest('hex');
  const claimed = await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${input.user.id} FOR UPDATE`);
    const [existing] = await tx.select().from(billingOrders)
      .where(and(eq(billingOrders.userId, input.user.id), eq(billingOrders.attemptKey, attemptKey))).limit(1);
    if (existing) {
      if (existing.intentHash !== intentHash) throw new BillingRequestError('idempotency_conflict', 'This idempotency key was already used for a different purchase.');
      if (existing.state === 'open' && existing.sessionUrl) return { kind: 'url' as const, url: existing.sessionUrl };
      if (['completed', 'expired', 'failed', 'retry_required', 'needs_reconciliation'].includes(existing.state)) {
        const status = existing.state === 'needs_reconciliation' ? 'reconciliation_required' : existing.state;
        return { kind: 'status' as const, status: status as BillingPendingError['status'] };
      }
      if (existing.leaseExpiresAt && existing.leaseExpiresAt > new Date()) return { kind: 'status' as const, status: 'processing' as const };
      if (Date.now() - existing.firstAttemptAt.getTime() > 23 * 60 * 60 * 1000) {
        await tx.update(billingOrders).set({ state: 'needs_reconciliation', version: existing.version + 1, updatedAt: new Date() })
          .where(eq(billingOrders.id, existing.id));
        return { kind: 'status' as const, status: 'reconciliation_required' as const };
      }
      const generation = existing.leaseGeneration + 1;
      await tx.update(billingOrders).set({ leaseGeneration: generation, leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS), updatedAt: new Date() })
        .where(eq(billingOrders.id, existing.id));
      return { kind: 'owner' as const, orderId: existing.id, generation, snapshot: existing.requestSnapshot as Stripe.Checkout.SessionCreateParams };
    }
    const [active] = await tx.select().from(billingOrders).where(and(
      eq(billingOrders.userId, input.user.id),
      sql`${billingOrders.state} in ('creating', 'open', 'processing', 'retry_required', 'retry_cancel_pending', 'needs_reconciliation')`,
    )).limit(1);
    if (active) return { kind: 'status' as const, status: 'processing' as const };
    const [latestUser] = await tx.select({ stripeCustomerId: users.stripeCustomerId }).from(users).where(eq(users.id, input.user.id)).limit(1);
    if (!latestUser?.stripeCustomerId || latestUser.stripeCustomerId !== customer) throw new BillingPendingError('processing');
    const [subscription] = await tx.select({ status: subscriptions.status }).from(subscriptions).where(eq(subscriptions.userId, input.user.id)).limit(1);
    const eligible = subscription ? isActiveBillingSubscriptionStatus(subscription.status) : false;
    if (item.kind === 'top_up' && !eligible) throw new BillingEligibilityError('top_up_requires_subscription', 'Top-ups are available after subscribing to Pro.');
    if (item.kind === 'subscription' && eligible) throw new BillingEligibilityError('already_subscribed', 'You already have an active Pro subscription. Manage it from billing settings.');
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
  if (claimed.kind === 'status') throw new BillingPendingError(claimed.status);
  try {
    const session = await stripe.checkout.sessions.create(claimed.snapshot, {
      idempotencyKey: `billing-checkout-${claimed.orderId}`,
      timeout: STRIPE_REQUEST_TIMEOUT_MS,
    });
    const persisted = await db.update(billingOrders).set({
      sessionId: session.id,
      sessionUrl: session.url,
      paymentMethodTypes: session.payment_method_types ?? [],
      paymentIntentId: objectId(session.payment_intent),
      state: session.status === 'open' && session.url ? 'open' : 'processing',
      leaseExpiresAt: null,
      version: sql`${billingOrders.version} + 1`,
      updatedAt: new Date(),
    }).where(and(eq(billingOrders.id, claimed.orderId), eq(billingOrders.leaseGeneration, claimed.generation), eq(billingOrders.state, 'creating')))
      .returning({ state: billingOrders.state });
    if (persisted.length === 0) throw new BillingPendingError('processing');
    if (session.status === 'open' && session.url) return { status: 'url' as const, url: session.url };
    return { status: 'processing' as const };
  } catch (error) {
    if (error instanceof BillingPendingError) throw error;
    if (error instanceof Stripe.errors.StripeIdempotencyError) throw new BillingPendingError('processing');
    log.warn('stripe_checkout_create_ambiguous', { order_id: claimed.orderId, error });
    throw new BillingPendingError('processing');
  }
}

export async function createBillingPortal(input: { user: typeof users.$inferSelect }) {
  if (!input.user.stripeCustomerId) throw new BillingConfigError('No Stripe customer exists for this user');
  const stripe = stripeClient();
  return stripe.billingPortal.sessions.create({
    customer: input.user.stripeCustomerId,
    return_url: publicUrl('/profile'),
  });
}

export type PublicCheckoutStatus = 'none' | 'open' | 'processing' | 'retry_required' | 'completed' | 'expired' | 'failed' | 'reconciliation_required';

function publicOrderStatus(state: string): PublicCheckoutStatus {
  if (state === 'needs_reconciliation') return 'reconciliation_required';
  if (state === 'retry_cancel_pending' || state === 'creating') return 'processing';
  if (state === 'open' || state === 'processing' || state === 'retry_required' || state === 'completed' || state === 'expired' || state === 'failed') return state;
  return 'processing';
}

async function resolveOwnedBillingOrder(input: {
  userId: string;
  attemptKey?: string;
  sessionId?: string;
  retryOperationId?: string;
}) {
  const found: Array<typeof billingOrders.$inferSelect> = [];
  if (input.attemptKey) {
    const [order] = await db.select().from(billingOrders).where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.attemptKey, input.attemptKey))).limit(1);
    if (order) found.push(order);
    else return null;
  }
  if (input.sessionId) {
    const [order] = await db.select().from(billingOrders).where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.sessionId, input.sessionId))).limit(1);
    if (order) found.push(order);
    else return null;
  }
  if (input.retryOperationId) {
    const [operation] = await db.select().from(billingRetryOperations).where(and(eq(billingRetryOperations.userId, input.userId), eq(billingRetryOperations.id, input.retryOperationId))).limit(1);
    if (!operation) return null;
    const relatedOrderIds = [operation.priorOrderId, operation.replacementOrderId].filter((id): id is string => Boolean(id));
    if (found.some((order) => !relatedOrderIds.includes(order.id))) throw new BillingRequestError('idempotency_conflict', 'The supplied billing identifiers refer to different purchases.');
    if (!found.length) {
      const orderId = operation.replacementOrderId ?? operation.priorOrderId;
      const [order] = await db.select().from(billingOrders).where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.id, orderId))).limit(1);
      if (!order) return null;
      found.push(order);
    }
  }
  if (found.length && found.some((order) => order.id !== found[0].id)) throw new BillingRequestError('idempotency_conflict', 'The supplied billing identifiers refer to different purchases.');
  if (found.length) return found[0];
  const [active] = await db.select().from(billingOrders).where(and(
    eq(billingOrders.userId, input.userId),
    sql`${billingOrders.state} in ('creating', 'open', 'processing', 'retry_required', 'retry_cancel_pending', 'needs_reconciliation')`,
  )).limit(1);
  return active ?? null;
}

async function checkoutStatusResponse(order: typeof billingOrders.$inferSelect | null, userId: string) {
  if (!order) return { status: 'none' as const };
  const [retry] = await db.select({ id: billingRetryOperations.id, state: billingRetryOperations.state })
    .from(billingRetryOperations).where(and(eq(billingRetryOperations.userId, userId), or(eq(billingRetryOperations.priorOrderId, order.id), eq(billingRetryOperations.replacementOrderId, order.id))))
    .limit(1);
  return {
    status: publicOrderStatus(order.state),
    ...(order.sessionUrl && order.state === 'open' ? { resumeUrl: order.sessionUrl } : {}),
    ...(retry && !['completed', 'expired', 'failed', 'superseded', 'retryable_failed'].includes(retry.state) ? { retryOperationId: retry.id } : {}),
    ...(order.attemptKey ? { attemptKey: order.attemptKey } : {}),
  };
}

export async function getBillingCheckoutStatus(input: { userId: string; attemptKey?: string; sessionId?: string; retryOperationId?: string }) {
  const order = await resolveOwnedBillingOrder(input);
  return checkoutStatusResponse(order, input.userId);
}

export async function refreshBillingCheckoutStatus(input: { userId: string; attemptKey?: string; sessionId?: string; retryOperationId?: string }) {
  const order = await resolveOwnedBillingOrder(input);
  if (!order || !order.sessionId) return checkoutStatusResponse(order, input.userId);
  const stripe = stripeClient();
  const session = await stripe.checkout.sessions.retrieve(order.sessionId);
  const sessionCustomerId = objectId(session.customer);
  if (session.id !== order.sessionId || sessionCustomerId !== order.stripeCustomerId || session.metadata?.order_id !== order.id) {
    await db.update(billingOrders).set({ state: 'needs_reconciliation', version: order.version + 1, updatedAt: new Date() })
      .where(and(eq(billingOrders.id, order.id), eq(billingOrders.version, order.version)));
    return checkoutStatusResponse({ ...order, state: 'needs_reconciliation' }, input.userId);
  }
  let nextState: string;
  const paymentIntentId = objectId(session.payment_intent);
  let intent: Stripe.PaymentIntent | null = null;
  if (paymentIntentId) intent = await stripe.paymentIntents.retrieve(paymentIntentId);
  if (session.payment_status === 'paid') nextState = 'processing';
  else if (session.status === 'open') nextState = 'open';
  else if (session.status === 'expired') {
    nextState = !intent || intent.status === 'canceled' ? 'expired' : intent.status === 'requires_payment_method' ? 'retry_required' : 'processing';
  } else if (session.status === 'complete') {
    nextState = intent?.status === 'requires_payment_method' ? 'retry_required' : intent?.status === 'canceled' ? 'failed' : 'processing';
  } else nextState = 'needs_reconciliation';

  await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM billing_orders WHERE id = ${order.id} FOR UPDATE`);
    const [current] = await tx.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
    if (!current || current.state === 'completed' || current.version !== order.version) return;
    await tx.update(billingOrders).set({
      state: nextState,
      paymentIntentId: paymentIntentId ?? current.paymentIntentId,
      version: current.version + 1,
      updatedAt: new Date(),
    }).where(and(eq(billingOrders.id, order.id), eq(billingOrders.version, current.version)));
  });
  const [updated] = await db.select().from(billingOrders).where(eq(billingOrders.id, order.id)).limit(1);
  return checkoutStatusResponse(updated ?? order, input.userId);
}

export async function retryBillingCheckout(input: {
  userId: string;
  attemptKey?: string;
  retryOperationId?: string;
  retryKey: string;
}) {
  let createdOperationId: string | null = null;
  const claim = await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${input.userId} FOR UPDATE`);
    let operation: typeof billingRetryOperations.$inferSelect | undefined;
    if (input.retryOperationId) {
      [operation] = await tx.select().from(billingRetryOperations).where(and(
        eq(billingRetryOperations.userId, input.userId), eq(billingRetryOperations.id, input.retryOperationId),
      )).limit(1);
      if (!operation) return { kind: 'status' as const, status: 'none' as const };
    }
    let priorOrder: typeof billingOrders.$inferSelect | undefined;
    if (operation) {
      [priorOrder] = await tx.select().from(billingOrders).where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.id, operation.priorOrderId))).limit(1);
    } else if (input.attemptKey) {
      [priorOrder] = await tx.select().from(billingOrders).where(and(eq(billingOrders.userId, input.userId), eq(billingOrders.attemptKey, input.attemptKey))).limit(1);
    }
    if (!priorOrder) return { kind: 'status' as const, status: 'none' as const };
    if (!operation) {
      if (priorOrder.state !== 'retry_required' || !priorOrder.sessionId || !priorOrder.paymentIntentId) {
        return { kind: 'status' as const, status: publicOrderStatus(priorOrder.state) };
      }
      const [activeOperation] = await tx.select().from(billingRetryOperations).where(and(
        eq(billingRetryOperations.priorOrderId, priorOrder.id),
        sql`${billingRetryOperations.state} in ('cancel_pending', 'replacement_ready', 'checkout_creating')`,
      )).limit(1);
      if (activeOperation) return { kind: 'status' as const, status: 'processing' as const, operationId: activeOperation.id };
      const [sameKey] = await tx.select().from(billingRetryOperations).where(and(
        eq(billingRetryOperations.userId, input.userId), eq(billingRetryOperations.retryKey, input.retryKey),
      )).limit(1);
      if (sameKey) return { kind: 'status' as const, status: 'processing' as const, operationId: sameKey.id };
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
      await tx.update(billingOrders).set({ state: 'retry_cancel_pending', version: priorOrder.version + 1, updatedAt: new Date() })
        .where(and(eq(billingOrders.id, priorOrder.id), eq(billingOrders.version, priorOrder.version)));
      [operation] = await tx.select().from(billingRetryOperations).where(eq(billingRetryOperations.id, operationId)).limit(1);
    } else if (!input.retryOperationId && operation.retryKey !== input.retryKey && operation.state !== 'checkout_open') {
      return { kind: 'status' as const, status: 'processing' as const, operationId: operation.id };
    }
    if (!operation) return { kind: 'status' as const, status: 'processing' as const };
    if (['retryable_failed', 'failed', 'expired', 'completed', 'superseded', 'needs_reconciliation'].includes(operation.state)) {
      return { kind: 'status' as const, status: operation.state === 'retryable_failed' ? 'retry_required' as const : operation.state === 'needs_reconciliation' ? 'reconciliation_required' as const : operation.state as 'failed' | 'expired' | 'completed' };
    }
    if (operation.state === 'checkout_open' && operation.replacementOrderId) {
      const [replacement] = await tx.select().from(billingOrders).where(eq(billingOrders.id, operation.replacementOrderId)).limit(1);
      if (replacement?.state === 'open' && replacement.sessionUrl) return { kind: 'url' as const, url: replacement.sessionUrl, operationId: operation.id };
      return { kind: 'status' as const, status: 'processing' as const, operationId: operation.id };
    }
    if (operation.id !== createdOperationId && operation.leaseExpiresAt && operation.leaseExpiresAt > new Date()) return { kind: 'status' as const, status: 'processing' as const, operationId: operation.id };
    if (operation.state === 'replacement_ready' || operation.state === 'checkout_creating') {
      const [replacement] = await tx.select().from(billingOrders).where(eq(billingOrders.id, operation.replacementOrderId!)).limit(1);
      if (!replacement) return { kind: 'status' as const, status: 'reconciliation_required' as const, operationId: operation.id };
      const generation = replacement.leaseGeneration + 1;
      await tx.update(billingRetryOperations).set({ state: 'checkout_creating', leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS), updatedAt: new Date() })
        .where(eq(billingRetryOperations.id, operation.id));
      await tx.update(billingOrders).set({ leaseGeneration: generation, leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS), updatedAt: new Date() })
        .where(eq(billingOrders.id, replacement.id));
      return { kind: 'create_replacement' as const, operationId: operation.id, orderId: replacement.id, generation, snapshot: replacement.requestSnapshot as Stripe.Checkout.SessionCreateParams };
    }
    const needsNewCancelGeneration = operation.state === 'cancel_pending' && operation.id !== createdOperationId;
    const nextGeneration = needsNewCancelGeneration ? operation.cancelGeneration + 1 : operation.cancelGeneration;
    const nextCancelKey = needsNewCancelGeneration ? `billing-cancel-${operation.id}-${nextGeneration}` : operation.cancelIdempotencyKey;
    await tx.update(billingRetryOperations).set({ cancelGeneration: nextGeneration, cancelIdempotencyKey: nextCancelKey,
      leaseExpiresAt: new Date(Date.now() + BILLING_LEASE_MS), updatedAt: new Date() })
      .where(eq(billingRetryOperations.id, operation.id));
    return { kind: 'cancel' as const, operationId: operation.id, order: priorOrder,
      operation: { ...operation, cancelGeneration: nextGeneration, cancelIdempotencyKey: nextCancelKey } };
  });

  if (claim.kind === 'status') return { status: claim.status, ...(claim.operationId ? { retryOperationId: claim.operationId } : {}) };
  if (claim.kind === 'url') return { status: 'url' as const, url: claim.url, retryOperationId: claim.operationId };
  const stripe = stripeClient();
  if (claim.kind === 'create_replacement') {
    try {
      const session = await stripe.checkout.sessions.create(claim.snapshot, {
        idempotencyKey: `billing-checkout-${claim.orderId}`,
        timeout: STRIPE_REQUEST_TIMEOUT_MS,
      });
      const nextState = session.status === 'open' && session.url ? 'open' : 'processing';
      await db.transaction(async (tx) => {
        await tx.update(billingOrders).set({ sessionId: session.id, sessionUrl: session.url, paymentMethodTypes: session.payment_method_types ?? [],
          paymentIntentId: objectId(session.payment_intent), state: nextState, leaseExpiresAt: null, version: sql`${billingOrders.version} + 1`, updatedAt: new Date() })
          .where(and(eq(billingOrders.id, claim.orderId), eq(billingOrders.leaseGeneration, claim.generation), eq(billingOrders.state, 'creating')));
        await tx.update(billingRetryOperations).set({ state: nextState === 'open' ? 'checkout_open' : 'checkout_creating', leaseExpiresAt: null, updatedAt: new Date() })
          .where(eq(billingRetryOperations.id, claim.operationId));
      });
      return session.status === 'open' && session.url
        ? { status: 'url' as const, url: session.url, retryOperationId: claim.operationId }
        : { status: 'processing' as const, retryOperationId: claim.operationId };
    } catch (error) {
      log.warn('stripe_retry_checkout_ambiguous', { retry_operation_id: claim.operationId, error });
      return { status: 'processing' as const, retryOperationId: claim.operationId };
    }
  }

  let session: Stripe.Checkout.Session;
  let intent: Stripe.PaymentIntent;
  try {
    session = await stripe.checkout.sessions.retrieve(claim.order.sessionId!);
    if (session.id !== claim.order.sessionId || objectId(session.customer) !== claim.order.stripeCustomerId || session.metadata?.order_id !== claim.order.id) {
      await db.update(billingRetryOperations).set({ state: 'needs_reconciliation', leaseExpiresAt: null, updatedAt: new Date() }).where(eq(billingRetryOperations.id, claim.operationId));
      await db.update(billingOrders).set({ state: 'needs_reconciliation', version: claim.order.version + 1, updatedAt: new Date() }).where(eq(billingOrders.id, claim.order.id));
      return { status: 'reconciliation_required' as const, retryOperationId: claim.operationId };
    }
    if (session.status === 'open') return { status: 'processing' as const, retryOperationId: claim.operationId };
    if (session.status !== 'complete' && session.status !== 'expired') return { status: 'processing' as const, retryOperationId: claim.operationId };
    if (!claim.order.paymentIntentId) return { status: 'processing' as const, retryOperationId: claim.operationId };
    intent = await stripe.paymentIntents.retrieve(claim.order.paymentIntentId);
  } catch (error) {
    log.warn('stripe_retry_refresh_failed', { retry_operation_id: claim.operationId, error });
    return { status: 'processing' as const, retryOperationId: claim.operationId };
  }
  if (intent.status === 'succeeded') {
    await db.update(billingRetryOperations).set({ state: 'needs_reconciliation', leaseExpiresAt: null, updatedAt: new Date() }).where(eq(billingRetryOperations.id, claim.operationId));
    return { status: 'processing' as const, retryOperationId: claim.operationId };
  }
  if (intent.status !== 'canceled' && intent.status !== 'requires_payment_method') {
    await db.update(billingRetryOperations).set({ leaseExpiresAt: null, updatedAt: new Date() }).where(eq(billingRetryOperations.id, claim.operationId));
    return { status: 'processing' as const, retryOperationId: claim.operationId };
  }
  let cancelRejected = false;
  if (intent.status === 'requires_payment_method') {
    try {
      await stripe.paymentIntents.cancel(intent.id, {}, {
        idempotencyKey: claim.operation.cancelIdempotencyKey,
        timeout: STRIPE_REQUEST_TIMEOUT_MS,
      });
    } catch (error) {
      cancelRejected = error instanceof Stripe.errors.StripeInvalidRequestError;
      log.warn('stripe_payment_intent_cancel_result', { retry_operation_id: claim.operationId, definitive_rejection: cancelRejected, error });
    }
    try { intent = await stripe.paymentIntents.retrieve(intent.id); }
    catch { return { status: 'processing' as const, retryOperationId: claim.operationId }; }
  }
  if (intent.status !== 'canceled') {
    if (cancelRejected && intent.status === 'requires_payment_method') {
      await db.transaction(async (tx) => {
        await tx.update(billingRetryOperations).set({ state: 'retryable_failed', leaseExpiresAt: null, updatedAt: new Date() }).where(eq(billingRetryOperations.id, claim.operationId));
        await tx.update(billingOrders).set({ state: 'retry_required', version: claim.order.version + 1, updatedAt: new Date() }).where(eq(billingOrders.id, claim.order.id));
      });
      return { status: 'retry_required' as const, retryOperationId: claim.operationId };
    }
    return { status: 'processing' as const, retryOperationId: claim.operationId };
  }

  const replacementId = randomUUID();
  const replacementAttemptKey = randomUUID();
  const requestSnapshot = structuredClone(claim.order.requestSnapshot) as Stripe.Checkout.SessionCreateParams;
  const replacementMetadata = { ...(requestSnapshot.metadata ?? {}), order_id: replacementId };
  requestSnapshot.metadata = replacementMetadata;
  if (requestSnapshot.payment_intent_data) requestSnapshot.payment_intent_data.metadata = replacementMetadata;
  if (requestSnapshot.subscription_data) requestSnapshot.subscription_data.metadata = replacementMetadata;
  await db.transaction(async (tx) => {
    await tx.execute(sql`SELECT id FROM users WHERE id = ${claim.order.userId} FOR UPDATE`);
    const [currentOrder] = await tx.select().from(billingOrders).where(eq(billingOrders.id, claim.order.id)).limit(1);
    const [currentOperation] = await tx.select().from(billingRetryOperations).where(eq(billingRetryOperations.id, claim.operationId)).limit(1);
    if (!currentOrder || !currentOperation || currentOrder.state === 'completed') return;
    await tx.update(billingOrders).set({ state: 'failed', version: currentOrder.version + 1, updatedAt: new Date() })
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
    await tx.update(billingRetryOperations).set({ state: 'replacement_ready', replacementOrderId: replacementId, leaseExpiresAt: null, updatedAt: new Date() })
      .where(eq(billingRetryOperations.id, claim.operationId));
  });
  return { status: 'processing' as const, retryOperationId: claim.operationId };
}

function unixSecondsToDate(value: unknown): Date {
  return typeof value === 'number' && Number.isFinite(value) ? new Date(value * 1000) : new Date();
}

function objectId(value: unknown): string | null {
  if (typeof value === 'string') return value;
  if (value && typeof value === 'object' && 'id' in value && typeof (value as { id: unknown }).id === 'string') return (value as { id: string }).id;
  return null;
}

function metadataOf(value: unknown): Record<string, string> {
  return value && typeof value === 'object' && 'metadata' in value && (value as { metadata?: unknown }).metadata && typeof (value as { metadata: unknown }).metadata === 'object'
    ? (value as { metadata: Record<string, string> }).metadata
    : {};
}

function firstInvoicePriceId(invoice: Stripe.Invoice): string | null {
  const lines = (invoice as any).lines?.data ?? [];
  const line = lines[0] as any;
  return line?.price?.id ?? line?.pricing?.price_details?.price ?? line?.parent?.subscription_item_details?.price ?? null;
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

async function userByStripeCustomer(database: BillingDb, customerId: string | null): Promise<typeof users.$inferSelect | null> {
  if (!customerId) return null;
  const [user] = await database.select().from(users).where(eq(users.stripeCustomerId, customerId)).limit(1);
  return user ?? null;
}

async function userForStripeObject(database: BillingDb, object: { customer?: unknown; client_reference_id?: string | null; metadata?: Record<string, string> | null }) {
  const userId = object.metadata?.user_id ?? object.client_reference_id;
  const customerId = objectId(object.customer);
  if (userId) {
    const [user] = await database.select().from(users).where(eq(users.id, userId)).limit(1);
    if (user && (!customerId || !user.stripeCustomerId || user.stripeCustomerId === customerId)) return user;
  }
  return userByStripeCustomer(database, customerId);
}

async function grantPaidCredits(database: BillingDb, userId: string, credits: number, reason: string, refId: string) {
  if (credits <= 0) return;
  await database.insert(creditLedger).values({
    userId,
    delta: credits,
    kind: 'paid',
    reason,
    refId,
  }).onConflictDoNothing({ target: [creditLedger.userId, creditLedger.reason, creditLedger.refId] });
}

async function updateUserPlan(database: BillingDb, userId: string, plan: string) {
  await database.update(users).set({ plan }).where(eq(users.id, userId));
}

async function upsertSubscription(database: BillingDb, input: {
  userId: string;
  stripeSubId: string;
  tier: string;
  status: string;
  currentPeriodEnd: Date;
  lastEventCreatedAt?: Date;
  lastEventId?: string;
}) {
  const versionColumns = input.lastEventCreatedAt
    ? { lastEventCreatedAt: input.lastEventCreatedAt, lastEventId: input.lastEventId ?? null }
    : {};
  await database.insert(subscriptions).values({ ...input, ...versionColumns }).onConflictDoUpdate({
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

async function handleCheckoutCompleted(database: BillingDb, session: Stripe.Checkout.Session, subscription: Stripe.Subscription | null) {
  if (session.mode !== 'subscription') return;
  const user = await userForStripeObject(database, session as any);
  const subscriptionId = objectId(session.subscription);
  if (!user || !subscriptionId || !subscription) return;
  const item = findItemByPriceId(session.metadata?.price_id) as SubscriptionTier | null;
  if (!item || item.kind !== 'subscription') return;
  await upsertSubscription(database, {
    userId: user.id,
    stripeSubId: subscription.id,
    tier: item.tier,
    status: subscription.status,
    currentPeriodEnd: subscriptionCurrentPeriodEnd(subscription),
  });
  if (session.payment_status === 'paid') {
    await grantPaidCredits(database, user.id, item.credits, 'stripe_checkout_subscription', session.id);
  }
}

async function handleInvoicePaid(database: BillingDb, invoice: Stripe.Invoice, subscription: Stripe.Subscription | null) {
  if ((invoice as any).billing_reason !== 'subscription_cycle') return;
  const customerId = objectId((invoice as any).customer);
  const user = await userByStripeCustomer(database, customerId);
  if (!user) return;
  const item = findItemByPriceId(firstInvoicePriceId(invoice));
  if (!item || item.kind !== 'subscription') return;
  if (subscription) {
    await upsertSubscription(database, {
      userId: user.id,
      stripeSubId: subscription.id,
      tier: item.tier,
      status: subscription.status,
      currentPeriodEnd: subscriptionCurrentPeriodEnd(subscription),
    });
  }
  await grantPaidCredits(database, user.id, item.credits, 'stripe_subscription_renewal', invoice.id);
}

async function handlePaymentIntentSucceeded(database: BillingDb, intent: Stripe.PaymentIntent) {
  const metadata = metadataOf(intent);
  if (metadata.billing_kind !== 'top_up') return;
  const user = await userForStripeObject(database, { customer: intent.customer, metadata });
  if (!user) return;
  const item = findItemByPriceId(metadata.price_id);
  if (!item || item.kind !== 'top_up') return;
  if (!(await userHasActiveSubscription(database, user.id))) {
    log.warn('stripe_top_up_rejected_without_subscription', { user_id: user.id, payment_intent_id: intent.id });
    return;
  }
  await grantPaidCredits(database, user.id, item.credits, 'stripe_top_up', intent.id);
}

type StoredSubscriptionVersion = { status: string; lastEventCreatedAt: Date | null };

function isStaleSubscriptionStatusEvent(incomingCreatedAt: Date, eventType: Stripe.Event['type'], existing: StoredSubscriptionVersion): boolean {
  if (!existing.lastEventCreatedAt) return false;
  const incoming = incomingCreatedAt.getTime();
  const stored = existing.lastEventCreatedAt.getTime();
  if (incoming > stored) return false;
  if (incoming < stored) return true;
  if (eventType === 'customer.subscription.deleted') return false;
  return !isActiveBillingSubscriptionStatus(existing.status);
}

async function handleSubscriptionStatus(database: BillingDb, event: Stripe.Event, subscription: Stripe.Subscription) {
  const customerId = objectId(subscription.customer);
  const user = await userByStripeCustomer(database, customerId);
  if (!user) return;
  const [existing] = await database
    .select({ stripeSubId: subscriptions.stripeSubId, tier: subscriptions.tier, status: subscriptions.status, lastEventCreatedAt: subscriptions.lastEventCreatedAt })
    .from(subscriptions)
    .where(eq(subscriptions.userId, user.id))
    .limit(1);
  if (existing && existing.stripeSubId !== subscription.id && isActiveBillingSubscriptionStatus(existing.status)) return;

  const incomingCreatedAt = unixSecondsToDate(event.created);
  if (existing && existing.stripeSubId === subscription.id && isStaleSubscriptionStatusEvent(incomingCreatedAt, event.type, existing)) {
    log.info('stripe_subscription_event_stale', {
      user_id: user.id,
      stripe_event_id: event.id,
      stripe_event_type: event.type,
      stripe_subscription_id: subscription.id,
    });
    return;
  }

  const priceId = subscriptionPriceId(subscription);
  const item = findItemByPriceId(priceId);
  const tier = item?.kind === 'subscription' ? item.tier : existing?.tier ?? (user.plan !== 'free' ? user.plan : 'pro');
  await upsertSubscription(database, {
    userId: user.id,
    stripeSubId: subscription.id,
    tier,
    status: subscription.status,
    currentPeriodEnd: subscriptionCurrentPeriodEnd(subscription),
    lastEventCreatedAt: incomingCreatedAt,
    lastEventId: event.id,
  });
}

type PreparedStripeEvent = {
  event: Stripe.Event;
  subscription: Stripe.Subscription | null;
};

function invoiceSubscriptionId(invoice: Stripe.Invoice): string | null {
  return objectId((invoice as any).subscription) ?? objectId((invoice as any).parent?.subscription_details?.subscription);
}

async function prepareStripeEvent(event: Stripe.Event): Promise<PreparedStripeEvent> {
  if (event.type === 'checkout.session.completed' || event.type === 'checkout.session.async_payment_succeeded') {
    const session = event.data.object as Stripe.Checkout.Session;
    const subscriptionId = objectId(session.subscription);
    return { event, subscription: subscriptionId ? await retrieveSubscription(subscriptionId) : null };
  }
  if (event.type === 'invoice.paid') {
    const invoice = event.data.object as Stripe.Invoice;
    const subscriptionId = invoiceSubscriptionId(invoice);
    return { event, subscription: subscriptionId ? await retrieveSubscription(subscriptionId) : null };
  }
  return { event, subscription: null };
}

async function handleStripeEvent(database: BillingDb, prepared: PreparedStripeEvent) {
  switch (prepared.event.type) {
    case 'checkout.session.completed':
    case 'checkout.session.async_payment_succeeded':
      await handleCheckoutCompleted(database, prepared.event.data.object as Stripe.Checkout.Session, prepared.subscription);
      break;
    case 'invoice.paid':
      await handleInvoicePaid(database, prepared.event.data.object as Stripe.Invoice, prepared.subscription);
      break;
    case 'payment_intent.succeeded':
      await handlePaymentIntentSucceeded(database, prepared.event.data.object as Stripe.PaymentIntent);
      break;
    case 'customer.subscription.deleted':
    case 'customer.subscription.updated':
      await handleSubscriptionStatus(database, prepared.event, prepared.event.data.object as Stripe.Subscription);
      break;
  }
}

export async function processStripeEvent(event: Stripe.Event): Promise<{ processed: boolean }> {
  return db.transaction(async (tx) => {
    const database = tx as unknown as BillingDb;
    const inserted = await database.insert(stripeEvents).values({
      eventId: event.id,
      type: event.type,
      apiVersion: event.api_version ?? null,
      payload: event as any,
    }).onConflictDoNothing({ target: stripeEvents.eventId }).returning({ eventId: stripeEvents.eventId });

    if (inserted.length === 0) return { processed: false };
    const prepared = await prepareStripeEvent(event);
    await handleStripeEvent(database, prepared);
    log.info('stripe_event_processed', { stripe_event_id: prepared.event.id, stripe_event_type: prepared.event.type });
    return { processed: true };
  });
}

export function constructStripeWebhookEvent(payload: string, signature: string | null): Stripe.Event {
  const webhookSecret = env('STRIPE_WEBHOOK_SECRET');
  if (!webhookSecret) throw new BillingConfigError('STRIPE_WEBHOOK_SECRET is not configured');
  if (!signature) throw new Error('Missing stripe-signature header');
  return stripeClient().webhooks.constructEvent(payload, signature, webhookSecret);
}

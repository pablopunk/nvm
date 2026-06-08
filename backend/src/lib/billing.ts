import Stripe from 'stripe';
import { and, eq, isNull } from 'drizzle-orm';
import { db } from '../db/client';
import { creditLedger, stripeEvents, subscriptions, users } from '../db/schema';
import { env } from './env';
import { log } from './log';

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

type StripeLike = Pick<Stripe, 'checkout' | 'billingPortal' | 'customers' | 'subscriptions' | 'webhooks'>;

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
  constructor(public type: 'unknown_price', message: string) {
    super(message);
    this.name = 'BillingRequestError';
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
    return priceId && tier && credits ? { kind: 'subscription', priceId, tier, credits } : null;
  });
  const topUpsFromJson = parseCatalogArray<TopUpPack>('STRIPE_TOP_UP_PACKS', (raw) => {
    const priceId = String(raw.priceId ?? raw.price_id ?? '').trim();
    const credits = parsePositiveInt(String(raw.credits ?? ''), 0);
    return priceId && credits ? { kind: 'top_up', priceId, credits } : null;
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
  const base = env('PUBLIC_DASHBOARD_URL') ?? 'http://localhost:4321';
  return new URL(path, base).toString();
}

export function rejectCrossOriginBillingPost(request: Request): Response | null {
  const origin = request.headers.get('origin');
  if (!origin) return null;
  if (origin === new URL(request.url).origin) return null;
  return Response.json({ error: { type: 'forbidden', message: 'Cross-origin billing request rejected' } }, { status: 403 });
}

export async function getOrCreateStripeCustomer(user: typeof users.$inferSelect): Promise<string> {
  if (user.stripeCustomerId) return user.stripeCustomerId;
  const stripe = stripeClient();
  const customer = await stripe.customers.create({
    email: user.email,
    metadata: { user_id: user.id },
  });
  const [updated] = await db
    .update(users)
    .set({ stripeCustomerId: customer.id })
    .where(and(eq(users.id, user.id), isNull(users.stripeCustomerId)))
    .returning({ stripeCustomerId: users.stripeCustomerId });
  if (updated?.stripeCustomerId) return updated.stripeCustomerId;

  const [current] = await db.select({ stripeCustomerId: users.stripeCustomerId }).from(users).where(eq(users.id, user.id)).limit(1);
  return current?.stripeCustomerId ?? customer.id;
}

async function userHasActiveSubscription(database: BillingDb, userId: string): Promise<boolean> {
  const [subscription] = await database
    .select({ status: subscriptions.status })
    .from(subscriptions)
    .where(eq(subscriptions.userId, userId))
    .limit(1);
  return subscription ? isActiveBillingSubscriptionStatus(subscription.status) : false;
}

export async function createBillingCheckout(input: { user: typeof users.$inferSelect; kind: BillingKind; priceId?: string; tier?: string }) {
  const item = findCatalogItem(input);
  if (!item) {
    if (input.priceId || input.tier) throw new BillingRequestError('unknown_price', 'Unknown Stripe billing price.');
    throw new BillingConfigError(`No Stripe ${input.kind} price is configured`);
  }
  const hasActiveSubscription = await userHasActiveSubscription(db, input.user.id);
  if (item.kind === 'subscription' && hasActiveSubscription) {
    throw new BillingEligibilityError('already_subscribed', 'You already have an active Pro subscription. Manage it from billing settings.');
  }
  if (item.kind === 'top_up' && !hasActiveSubscription) {
    throw new BillingEligibilityError('top_up_requires_subscription', 'Top-ups are available after subscribing to Pro.');
  }
  const customer = await getOrCreateStripeCustomer(input.user);
  const metadata: Stripe.MetadataParam = {
    billing_kind: item.kind,
    user_id: input.user.id,
    price_id: item.priceId,
    credits: String(item.credits),
    ...(item.kind === 'subscription' ? { tier: item.tier } : {}),
  };
  const stripe = stripeClient();
  return stripe.checkout.sessions.create({
    mode: item.kind === 'subscription' ? 'subscription' : 'payment',
    customer,
    client_reference_id: input.user.id,
    line_items: [{ price: item.priceId, quantity: 1 }],
    success_url: publicUrl('/profile?billing=success'),
    cancel_url: publicUrl('/profile?billing=canceled'),
    metadata,
    ...(item.kind === 'subscription' ? { subscription_data: { metadata } } : { payment_intent_data: { metadata } }),
  });
}

export async function createBillingPortal(input: { user: typeof users.$inferSelect }) {
  if (!input.user.stripeCustomerId) throw new BillingConfigError('No Stripe customer exists for this user');
  const stripe = stripeClient();
  return stripe.billingPortal.sessions.create({
    customer: input.user.stripeCustomerId,
    return_url: publicUrl('/profile'),
  });
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
}) {
  await database.insert(subscriptions).values(input).onConflictDoUpdate({
    target: subscriptions.userId,
    set: {
      stripeSubId: input.stripeSubId,
      tier: input.tier,
      status: input.status,
      currentPeriodEnd: input.currentPeriodEnd,
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
    currentPeriodEnd: unixSecondsToDate((subscription as any).current_period_end),
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
      currentPeriodEnd: unixSecondsToDate((subscription as any).current_period_end),
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

async function handleSubscriptionStatus(database: BillingDb, subscription: Stripe.Subscription) {
  const customerId = objectId(subscription.customer);
  const user = await userByStripeCustomer(database, customerId);
  if (!user) return;
  const [existing] = await database
    .select({ stripeSubId: subscriptions.stripeSubId, tier: subscriptions.tier, status: subscriptions.status })
    .from(subscriptions)
    .where(eq(subscriptions.userId, user.id))
    .limit(1);
  if (existing && existing.stripeSubId !== subscription.id && isActiveBillingSubscriptionStatus(existing.status)) return;

  const priceId = subscriptionPriceId(subscription);
  const item = findItemByPriceId(priceId);
  const tier = item?.kind === 'subscription' ? item.tier : existing?.tier ?? (user.plan !== 'free' ? user.plan : 'pro');
  await upsertSubscription(database, {
    userId: user.id,
    stripeSubId: subscription.id,
    tier,
    status: subscription.status,
    currentPeriodEnd: unixSecondsToDate((subscription as any).current_period_end),
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
      await handleSubscriptionStatus(database, prepared.event.data.object as Stripe.Subscription);
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

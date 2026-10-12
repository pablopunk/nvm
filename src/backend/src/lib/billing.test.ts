import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import {
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
import { POST as postWebhook } from '../pages/api/billing/webhook';
import {
  BillingConfigError,
  BillingEligibilityError,
  BillingPendingError,
  BillingRequestError,
  billingCatalog,
  canCreateRetryReplacement,
  cancelPaymentIntentForRetry,
  classifyAsyncPaymentFailure,
  classifyCheckoutEvidence,
  createBillingCheckout,
  formatBillingPrice,
  OWNER_APPROVED_PRICE_TERMS_SOURCE,
  applyAdminRetryReconciliation,
  processStripeEvent,
  rejectCrossOriginBillingPost,
  retryBillingCheckout,
  setStripeForTests,
  updateBillingOrderCAS,
} from './billing';
import {
  acquireCheckoutLock,
  releaseCheckoutLock,
  setCheckoutLockForTests,
} from './ratelimit';

Object.assign(process.env, {
  VERCEL_ENV: 'production',
  PRODUCTION_ORIGIN: 'https://www.nvm.fyi',
  PUBLIC_DASHBOARD_URL: 'https://www.nvm.fyi',
  PREVIEW_GATEWAY_ORIGIN: 'https://www.nvm.fyi',
});

const user = {
  id: '11111111-1111-1111-1111-111111111111',
  workosUserId: 'workos_1',
  email: 'pablo@example.com',
  plan: 'free',
  role: 'user',
  stripeCustomerId: 'cus_123',
  createdAt: new Date(),
};

type InsertCall = {
  table: unknown;
  values: any;
  conflict?: 'nothing' | 'update';
};
type UpdateCall = { table: unknown; values: any };
type FakeDb = {
  inserts: InsertCall[];
  updates: UpdateCall[];
  select: () => any;
  insert: (table: unknown) => any;
  update: (table: unknown) => any;
  execute: () => Promise<unknown>;
  transaction: (callback: (tx: FakeDb) => Promise<unknown>) => Promise<unknown>;
};

function createFakeBillingDb(
  input: {
    selects?: unknown[];
    existingEvents?: string[];
    existingLedgerRefs?: string[];
    trackSubscriptions?: boolean;
    billingTerms?: unknown[];
    subscriptionRows?: unknown[];
    orders?: unknown[];
    orderResults?: unknown[][];
    retryOperationResults?: unknown[][];
  } = {},
): FakeDb {
  const selects = [...(input.selects ?? [])];
  const eventIds = new Set(input.existingEvents ?? []);
  const ledgerRefs = new Set(input.existingLedgerRefs ?? []);
  const inserts: InsertCall[] = [];
  const updates: UpdateCall[] = [];
  let subscriptionRow: any = null;
  let orderSelectCount = 0;
  let retryOperationSelectCount = 0;

  function createInsertChain(table: unknown) {
    let values: any;
    let conflict: InsertCall['conflict'];
    const resolve = () => Promise.resolve([]);
    const chain = {
      values(next: any) {
        values = next;
        return chain;
      },
      onConflictDoNothing() {
        conflict = 'nothing' as const;
        return chain;
      },
      onConflictDoUpdate() {
        conflict = 'update' as const;
        if (input.trackSubscriptions && table === subscriptions)
          subscriptionRow = { ...values };
        inserts.push({ table, values, conflict });
        return chain;
      },
      returning() {
        if (table === stripeEvents) {
          if (eventIds.has(values.eventId)) return Promise.resolve([]);
          eventIds.add(values.eventId);
        }
        inserts.push({ table, values, conflict });
        return Promise.resolve([{ eventId: values.eventId }]);
      },
      then(
        resolveThen: Parameters<Promise<unknown>['then']>[0],
        rejectThen: Parameters<Promise<unknown>['then']>[1],
      ) {
        if (table !== stripeEvents && conflict !== 'update') {
          const ledgerRef =
            table === creditLedger && values?.refId
              ? `${values.userId}:${values.reason}:${values.refId}`
              : null;
          if (!ledgerRef || !ledgerRefs.has(ledgerRef)) {
            if (ledgerRef) ledgerRefs.add(ledgerRef);
            inserts.push({ table, values, conflict });
          }
        }
        return resolve().then(resolveThen, rejectThen);
      },
      catch(rejectThen: Parameters<Promise<unknown>['catch']>[0]) {
        return resolve().catch(rejectThen);
      },
    };
    return chain;
  }

  function createSelectChain() {
    let fromTable: unknown;
    const promise = () => {
      if (input.trackSubscriptions && fromTable === subscriptions) {
        return Promise.resolve(subscriptionRow ? [subscriptionRow] : []);
      }
      if (fromTable === stripeEvents) return Promise.resolve([]);
      if (fromTable === billingPriceTerms)
        return Promise.resolve(input.billingTerms ?? []);
      if (fromTable === billingOrders)
        return Promise.resolve(
          input.orderResults?.[orderSelectCount++] ?? input.orders ?? [],
        );
      if (fromTable === billingRetryOperations)
        return Promise.resolve(
          input.retryOperationResults?.[retryOperationSelectCount++] ?? [],
        );
      if (
        fromTable === billingSubscriptionPriceVersions ||
        fromTable === stripeCustomerAttempts
      )
        return Promise.resolve([]);
      if (fromTable === subscriptions)
        return Promise.resolve(input.subscriptionRows ?? []);
      if (fromTable === users) return Promise.resolve(selects.shift() ?? []);
      return Promise.resolve(selects.shift() ?? []);
    };
    const chain = {
      from: (table: unknown) => {
        fromTable = table;
        return chain;
      },
      where: () => chain,
      for: () => chain,
      limit: () => promise(),
      then: (
        resolveThen: Parameters<Promise<unknown>['then']>[0],
        rejectThen: Parameters<Promise<unknown>['then']>[1],
      ) => promise().then(resolveThen, rejectThen),
      catch: (rejectThen: Parameters<Promise<unknown>['catch']>[0]) =>
        promise().catch(rejectThen),
    };
    return chain;
  }

  function createUpdateChain(table: unknown) {
    let values: any;
    const promise = () => Promise.resolve([]);
    const chain = {
      set(next: any) {
        values = next;
        return chain;
      },
      where() {
        updates.push({ table, values });
        return chain;
      },
      returning() {
        if (table === billingOrders && input.orders?.length) {
          return Promise.resolve([
            { ...(input.orders[0] as Record<string, unknown>), ...values },
          ]);
        }
        return Promise.resolve([
          { stripeCustomerId: values?.stripeCustomerId },
        ]);
      },
      then(
        resolveThen: Parameters<Promise<unknown>['then']>[0],
        rejectThen: Parameters<Promise<unknown>['then']>[1],
      ) {
        return promise().then(resolveThen, rejectThen);
      },
      catch(rejectThen: Parameters<Promise<unknown>['catch']>[0]) {
        return promise().catch(rejectThen);
      },
    };
    return chain;
  }

  let db: FakeDb;
  db = {
    inserts,
    updates,
    select: () => createSelectChain(),
    insert: (table: unknown) => createInsertChain(table),
    update: (table: unknown) => createUpdateChain(table),
    execute: async () => [],
    transaction: async (callback: (tx: FakeDb) => Promise<unknown>) => {
      const eventSnapshot = new Set(eventIds);
      const ledgerSnapshot = new Set(ledgerRefs);
      const subscriptionSnapshot = subscriptionRow
        ? { ...subscriptionRow }
        : null;
      const insertsLength = inserts.length;
      const updatesLength = updates.length;
      try {
        return await callback(db);
      } catch (error) {
        eventIds.clear();
        eventSnapshot.forEach((id) => eventIds.add(id));
        ledgerRefs.clear();
        ledgerSnapshot.forEach((ref) => ledgerRefs.add(ref));
        subscriptionRow = subscriptionSnapshot;
        inserts.length = insertsLength;
        updates.length = updatesLength;
        throw error;
      }
    },
  };
  return db;
}

function installDb(db: FakeDb) {
  setDbForTests(db as any);
  return db;
}

function approvedTerms(
  input: Partial<typeof billingPriceTerms.$inferSelect> = {},
) {
  return {
    id: '00000000-0000-4000-8000-000000000001',
    priceId: 'price_pro',
    version: 1,
    kind: 'subscription',
    tier: 'pro',
    credits: 1000,
    unitAmount: 1000,
    currency: 'usd',
    quantity: 1,
    interval: 'month',
    intervalCount: 1,
    source: OWNER_APPROVED_PRICE_TERMS_SOURCE,
    approvedAt: new Date(0),
    createdAt: new Date(0),
    ...input,
  };
}

const stripeObjects = new Map<string, Record<string, unknown>>();

function installStripe(
  subscription: Record<string, unknown> = {},
  lineItems: unknown[] = [],
) {
  setStripeForTests({
    checkout: {
      sessions: {
        retrieve: async (id: string) =>
          stripeObjects.get(id) ?? {
            id,
            status: 'open',
            payment_status: 'unpaid',
          },
        listLineItems: async () => ({ data: lineItems, has_more: false }),
        list: async () => ({ data: [], has_more: false }),
      },
    },
    invoices: {
      retrieve: async (id: string) =>
        stripeObjects.get(id) ?? { id, lines: { data: [] } },
    },
    paymentIntents: {
      retrieve: async (id: string) =>
        stripeObjects.get(id) ?? { id, status: 'requires_payment_method' },
      cancel: async (id: string) => ({ id, status: 'canceled' }),
    },
    prices: {
      retrieve: async (id: string) => ({
        id,
        active: true,
        billing_scheme: 'per_unit',
        unit_amount: id.includes('topup') ? 500 : 1000,
        currency: 'usd',
        type: id.includes('topup') ? 'one_time' : 'recurring',
        recurring: id.includes('topup')
          ? null
          : { interval: 'month', interval_count: 1, usage_type: 'licensed' },
        tiers_mode: null,
        transform_quantity: null,
      }),
    },
    subscriptions: {
      retrieve: async (id: string) =>
        stripeObjects.get(id) ?? {
          id,
          status: subscription.status ?? 'active',
          current_period_end: 1_800_000_000,
          ...subscription,
        },
    },
    webhooks: {
      constructEvent: () => {
        throw new Error('bad signature');
      },
    },
  } as any);
}

function stripeEvent(
  type: string,
  object: Record<string, unknown>,
  id = `evt_${type}`,
  created = 1,
) {
  const event = {
    id,
    object: 'event',
    api_version: '2025-01-27.acacia',
    created,
    livemode: false,
    pending_webhooks: 1,
    request: null,
    type,
    data: { object },
  } as any;
  if (object.id) stripeObjects.set(String(object.id), object);
  return event;
}

afterEach(() => {
  resetDbForTests();
  setStripeForTests(null);
  setCheckoutLockForTests(null);
  stripeObjects.clear();
  delete process.env.STRIPE_SUBSCRIPTION_TIERS;
  delete process.env.STRIPE_TOP_UP_PACKS;
  delete process.env.STRIPE_WEBHOOK_SECRET;
});

test('billing catalog exposes validated display metadata from configured prices', () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    {
      priceId: 'price_pro',
      tier: 'pro',
      credits: 1000,
      amount: 2000,
      currency: 'usd',
      interval: 'month',
    },
  ]);
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([
    {
      priceId: 'price_topup',
      credits: 500,
      unit_amount: 500,
      currency: 'EUR',
    },
  ]);

  assert.deepEqual(billingCatalog(), {
    subscriptions: [
      {
        kind: 'subscription',
        priceId: 'price_pro',
        tier: 'pro',
        credits: 1000,
        displayPrice: { amount: 2000, currency: 'USD', interval: 'month' },
      },
    ],
    topUps: [
      {
        kind: 'top_up',
        priceId: 'price_topup',
        credits: 500,
        displayPrice: { amount: 500, currency: 'EUR' },
      },
    ],
  });
  assert.equal(
    formatBillingPrice({ amount: 2000, currency: 'USD', interval: 'month' }),
    '$20.00',
  );
});

test('production billing rejects stale dashboard aliases as configuration without calling Stripe', async () => {
  const previousEnvironment = process.env.VERCEL_ENV;
  const previousProduction = process.env.PRODUCTION_ORIGIN;
  const previousDashboard = process.env.PUBLIC_DASHBOARD_URL;
  process.env.VERCEL_ENV = 'production';
  delete process.env.PRODUCTION_ORIGIN;
  process.env.PUBLIC_DASHBOARD_URL = 'https://nvm.fyi';
  try {
    await assert.rejects(
      () => createBillingCheckout({ user, kind: 'subscription' }),
      (error: unknown) =>
        error instanceof BillingConfigError &&
        /origin configuration is invalid/.test(error.message),
    );
  } finally {
    if (previousEnvironment === undefined) delete process.env.VERCEL_ENV;
    else process.env.VERCEL_ENV = previousEnvironment;
    if (previousProduction === undefined) delete process.env.PRODUCTION_ORIGIN;
    else process.env.PRODUCTION_ORIGIN = previousProduction;
    if (previousDashboard === undefined)
      delete process.env.PUBLIC_DASHBOARD_URL;
    else process.env.PUBLIC_DASHBOARD_URL = previousDashboard;
  }
});

test('billing catalog keeps billing items while omitting incomplete display metadata', () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    {
      priceId: 'price_pro',
      tier: 'pro',
      credits: 1000,
      amount: 2000,
      currency: 'usd',
    },
  ]);
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([
    {
      priceId: 'price_topup',
      credits: 500,
      amount: 500.5,
      currency: 'US',
    },
  ]);

  const catalog = billingCatalog();
  assert.deepEqual(catalog.subscriptions[0], {
    kind: 'subscription',
    priceId: 'price_pro',
    tier: 'pro',
    credits: 1000,
  });
  assert.deepEqual(catalog.topUps[0], {
    kind: 'top_up',
    priceId: 'price_topup',
    credits: 500,
  });
});

test('checkout fails closed without owner-approved terms before creating a Stripe customer', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    {
      priceId: 'price_unapproved',
      tier: 'pro',
      credits: 1000,
      amount: 2000,
      currency: 'usd',
      interval: 'month',
    },
  ]);
  let customerCreates = 0;
  setStripeForTests({
    customers: {
      create: async () => {
        customerCreates += 1;
        throw new Error('unexpected customer creation');
      },
    },
  } as any);
  installDb(
    createFakeBillingDb({
      billingTerms: [
        approvedTerms({ source: 'admin_approved_verified_stripe_price' }),
      ],
    }),
  );

  await assert.rejects(
    () =>
      createBillingCheckout({
        user,
        kind: 'subscription',
        priceId: 'price_unapproved',
      }),
    (error) =>
      error instanceof BillingRequestError && error.type === 'unknown_price',
  );
  assert.equal(customerCreates, 0);
});

test('same-key checkout recovers a stored session when catalog entry disappears and Stripe Price is inactive', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = '[]';
  process.env.STRIPE_TOP_UP_PACKS = '[]';
  const attemptKey = '22222222-2222-4222-8222-222222222222';
  const intentHash = createHash('sha256')
    .update(
      JSON.stringify({
        kind: 'subscription',
        priceId: 'price_removed',
        selectedPriceId: 'price_removed',
        selectedTier: 'pro',
      }),
    )
    .digest('hex');
  let priceReads = 0;
  let sessionCreates = 0;
  const order = {
    id: '33333333-3333-4333-8333-333333333333',
    userId: user.id,
    attemptKey,
    intentHash,
    priceTermsId: 'term-owner-approved',
    priceId: 'price_removed',
    state: 'open',
    sessionId: 'cs_already_created',
    sessionUrl: 'https://checkout.stripe.test/cs_already_created',
  };
  installDb(
    createFakeBillingDb({
      orders: [order],
      billingTerms: [
        approvedTerms({ id: 'term-owner-approved', priceId: 'price_removed' }),
      ],
    }),
  );
  setStripeForTests({
    prices: {
      retrieve: async () => {
        priceReads += 1;
        throw new Error('inactive price must not be read for recovery');
      },
    },
    checkout: {
      sessions: {
        create: async () => {
          sessionCreates += 1;
          throw new Error('existing session should be reused');
        },
      },
    },
  } as any);

  assert.deepEqual(
    await createBillingCheckout({
      user,
      kind: 'subscription',
      priceId: 'price_removed',
      tier: 'pro',
      attemptKey,
    }),
    {
      status: 'url',
      url: order.sessionUrl,
    },
  );
  assert.equal(priceReads, 0);
  assert.equal(sessionCreates, 0);
});

test('same-key checkout does not resume a session with non-owner-approved terms', async () => {
  const attemptKey = '22222222-2222-4222-8222-222222222222';
  const intentHash = createHash('sha256')
    .update(
      JSON.stringify({
        kind: 'subscription',
        priceId: 'price_legacy',
        selectedPriceId: 'price_legacy',
        selectedTier: 'pro',
      }),
    )
    .digest('hex');
  const order = {
    id: '33333333-3333-4333-8333-333333333333',
    userId: user.id,
    attemptKey,
    intentHash,
    priceTermsId: 'term-legacy',
    priceId: 'price_legacy',
    state: 'open',
    sessionId: 'cs_legacy',
    sessionUrl: 'https://checkout.stripe.test/cs_legacy',
  };
  installDb(
    createFakeBillingDb({
      orders: [order],
      billingTerms: [
        approvedTerms({
          id: 'term-legacy',
          priceId: 'price_legacy',
          source: 'admin_approved_verified_stripe_price',
        }),
      ],
    }),
  );

  await assert.rejects(
    () =>
      createBillingCheckout({
        user,
        kind: 'subscription',
        priceId: 'price_legacy',
        tier: 'pro',
        attemptKey,
      }),
    (error) =>
      error instanceof BillingPendingError &&
      error.status === 'reconciliation_required',
  );
});

test('same retry key resumes the linked replacement after a lost response and failed prior order', async () => {
  const attemptKey = '22222222-2222-4222-8222-222222222222';
  const retryKey = '44444444-4444-4444-8444-444444444444';
  const priorOrder = {
    id: 'order_prior',
    userId: user.id,
    attemptKey,
    state: 'failed',
    priceTermsId: 'term-owner-approved',
  };
  const replacementOrder = {
    id: 'order_replacement',
    userId: user.id,
    attemptKey: '33333333-3333-4333-8333-333333333333',
    state: 'open',
    sessionUrl: 'https://checkout.stripe.test/cs_replacement',
  };
  const operation = {
    id: 'retry_operation',
    userId: user.id,
    priorOrderId: priorOrder.id,
    replacementOrderId: replacementOrder.id,
    retryKey,
    state: 'checkout_open',
  };
  installDb(
    createFakeBillingDb({
      orderResults: [[priorOrder], [priorOrder], [replacementOrder]],
      retryOperationResults: [[operation]],
      billingTerms: [approvedTerms({ id: 'term-owner-approved' })],
    }),
  );

  assert.deepEqual(
    await retryBillingCheckout({ userId: user.id, attemptKey, retryKey }),
    {
      status: 'url',
      url: replacementOrder.sessionUrl,
      retryOperationId: operation.id,
    },
  );
});

test('billing retry refuses to issue a replacement for non-owner-approved terms', async () => {
  const retryKey = '44444444-4444-4444-8444-444444444444';
  const priorOrder = {
    id: 'order_legacy_retry',
    userId: user.id,
    state: 'retry_required',
    priceTermsId: 'term-legacy',
  };
  const operation = {
    id: 'retry_legacy_operation',
    userId: user.id,
    priorOrderId: priorOrder.id,
    retryKey,
    state: 'cancel_pending',
  };
  installDb(
    createFakeBillingDb({
      orderResults: [[priorOrder]],
      retryOperationResults: [[operation]],
      billingTerms: [
        approvedTerms({
          id: 'term-legacy',
          source: 'admin_approved_verified_stripe_price',
        }),
      ],
    }),
  );

  assert.deepEqual(
    await retryBillingCheckout({
      userId: user.id,
      retryOperationId: operation.id,
      retryKey,
    }),
    { status: 'reconciliation_required', retryOperationId: operation.id },
  );
});

test('admin retry reconciliation closes canceled legacy terms without making a replacement', async () => {
  const operation = {
    id: 'retry_legacy_operation',
    userId: user.id,
    priorOrderId: 'order_legacy_retry',
    retryKey: '44444444-4444-4444-8444-444444444444',
    state: 'cancel_pending',
    replacementOrderId: null,
  };
  const order = {
    id: operation.priorOrderId,
    userId: user.id,
    state: 'retry_cancel_pending',
    version: 3,
    paymentIntentId: 'pi_legacy_retry',
    sessionId: 'cs_legacy_retry',
    stripeCustomerId: 'cus_legacy_retry',
    priceTermsId: 'term-legacy',
    priceId: 'price_legacy',
    kind: 'subscription',
    credits: 1000,
  };
  const db = installDb(
    createFakeBillingDb({
      orderResults: [[order]],
      retryOperationResults: [[operation]],
      billingTerms: [
        approvedTerms({
          id: 'term-legacy',
          priceId: 'price_legacy',
          source: 'admin_approved_verified_stripe_price',
        }),
      ],
    }),
  );

  const result = await applyAdminRetryReconciliation(db as any, {
    actorUserId: 'admin_1',
    evidence: {
      operation: operation as any,
      order: order as any,
      paymentIntent: {
        id: order.paymentIntentId,
        status: 'canceled',
        customer: order.stripeCustomerId,
        metadata: {
          order_id: order.id,
          user_id: order.userId,
          price_id: order.priceId,
          billing_kind: order.kind,
          credits: String(order.credits),
        },
      } as any,
      session: {
        id: order.sessionId,
        status: 'complete',
        payment_status: 'unpaid',
        customer: order.stripeCustomerId,
        metadata: { order_id: order.id },
        payment_intent: order.paymentIntentId,
      } as any,
      lineItems: null,
      subscription: null,
      observedAt: new Date(0).toISOString(),
    },
  });

  assert.equal((result.body as any).status, 'failed');
  assert.equal((result.body as any).reason, 'price_terms_not_owner_approved');
  assert.equal(
    db.inserts.some((insert) => insert.table === billingOrders),
    false,
  );
  assert.ok(
    db.updates.some(
      (update) =>
        update.table === billingRetryOperations &&
        update.values.state === 'failed',
    ),
  );
  assert.ok(
    db.updates.some(
      (update) =>
        update.table === billingOrders && update.values.state === 'failed',
    ),
  );
});

test('retry persists a replacement only after Stripe confirms cancellation; succeeded or ambiguous intents stay blocked', async () => {
  const retryKey = '44444444-4444-4444-8444-444444444444';
  const operation = {
    id: '55555555-5555-4555-8555-555555555555',
    userId: user.id,
    priorOrderId: 'order_retry_prior',
    retryKey,
    state: 'cancel_pending',
    cancelGeneration: 1,
    cancelIdempotencyKey: 'billing-cancel-op-1',
    leaseExpiresAt: new Date(0),
  };
  const priorOrder = {
    id: operation.priorOrderId,
    userId: user.id,
    attemptKey: '22222222-2222-4222-8222-222222222222',
    state: 'retry_cancel_pending',
    version: 4,
    leaseGeneration: 0,
    sessionId: 'cs_retry_prior',
    paymentIntentId: 'pi_retry_prior',
    priceTermsId: 'term-owner-approved',
    stripeCustomerId: 'cus_123',
    intentHash: 'request-hash',
    kind: 'top_up',
    priceTermsId: 'term-1',
    priceId: 'price_pack',
    tier: null,
    credits: 500,
    expectedUnitAmount: 500,
    expectedTotalAmount: 500,
    currency: 'usd',
    quantity: 1,
    interval: null,
    intervalCount: null,
    requestSnapshot: {
      mode: 'payment',
      metadata: { order_id: operation.priorOrderId },
    },
  };
  const currentSession = {
    id: priorOrder.sessionId,
    status: 'complete',
    payment_status: 'unpaid',
    customer: priorOrder.stripeCustomerId,
    metadata: { order_id: priorOrder.id },
    payment_intent: priorOrder.paymentIntentId,
  };

  async function runRetry(
    initialIntentStatus: string,
    postCancelStatus?: string,
  ) {
    const db = installDb(
      createFakeBillingDb({
        orderResults: [[priorOrder], [priorOrder]],
        retryOperationResults: [[operation], [operation]],
        billingTerms: [
          approvedTerms({
            id: 'term-owner-approved',
            priceId: 'price_pack',
            kind: 'top_up',
            tier: null,
            credits: 500,
            unitAmount: 500,
            interval: null,
            intervalCount: null,
          }),
        ],
      }),
    );
    let retrieves = 0;
    let cancelCalls = 0;
    let checkoutCreates = 0;
    setStripeForTests({
      checkout: {
        sessions: {
          retrieve: async () => currentSession,
          create: async () => {
            checkoutCreates += 1;
            return {
              id: 'cs_replacement',
              status: 'open',
              url: 'https://checkout.stripe.test/new',
            };
          },
        },
      },
      paymentIntents: {
        retrieve: async () => ({
          id: priorOrder.paymentIntentId,
          status:
            retrieves++ === 0
              ? initialIntentStatus
              : (postCancelStatus ?? initialIntentStatus),
          customer: priorOrder.stripeCustomerId,
          metadata: { order_id: priorOrder.id },
        }),
        cancel: async () => {
          cancelCalls += 1;
          return { id: priorOrder.paymentIntentId, status: 'canceled' };
        },
      },
    } as any);
    const result = await retryBillingCheckout({
      userId: user.id,
      retryOperationId: operation.id,
      retryKey,
    });
    return {
      result,
      cancelCalls,
      checkoutCreates,
      replacements: db.inserts.filter(
        (call) =>
          call.table === billingOrders &&
          (call.values as any)?.id !== priorOrder.id,
      ),
    };
  }

  const verifiedCanceled = await runRetry(
    'requires_payment_method',
    'canceled',
  );
  assert.equal(verifiedCanceled.result.status, 'processing');
  assert.equal(verifiedCanceled.cancelCalls, 1);
  assert.equal(verifiedCanceled.replacements.length, 1);
  assert.equal(verifiedCanceled.checkoutCreates, 0);

  for (const status of ['requires_payment_method', 'processing']) {
    const ambiguous = await runRetry('requires_payment_method', status);
    assert.equal(ambiguous.result.status, 'processing');
    assert.equal(ambiguous.cancelCalls, 1);
    assert.equal(ambiguous.replacements.length, 0);
    assert.equal(ambiguous.checkoutCreates, 0);
  }

  const initiallyProcessing = await runRetry('processing');
  assert.equal(initiallyProcessing.cancelCalls, 0);
  assert.equal(initiallyProcessing.replacements.length, 0);

  const succeeded = await runRetry('succeeded');
  assert.equal(succeeded.result.status, 'processing');
  assert.equal(succeeded.cancelCalls, 0);
  assert.equal(succeeded.replacements.length, 0);
  assert.equal(succeeded.checkoutCreates, 0);
});

test('checkout state classifier keeps retryable and ambiguous payment evidence active', () => {
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'open',
      paymentStatus: 'unpaid',
      paymentIntentStatus: 'requires_payment_method',
    }),
    'open',
  );
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'complete',
      paymentStatus: 'unpaid',
      paymentIntentStatus: 'requires_payment_method',
    }),
    'retry_required',
  );
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'expired',
      paymentStatus: 'unpaid',
      paymentIntentStatus: 'requires_payment_method',
    }),
    'retry_required',
  );
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'complete',
      paymentStatus: 'unpaid',
      paymentIntentStatus: 'canceled',
    }),
    'failed',
  );
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'expired',
      paymentStatus: 'unpaid',
      paymentIntentStatus: 'canceled',
    }),
    'expired',
  );
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'expired',
      paymentStatus: 'unpaid',
    }),
    'expired',
  );
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'expired',
      paymentStatus: 'no_payment_required',
    }),
    'processing',
  );
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'complete',
      paymentStatus: 'unpaid',
      paymentIntentStatus: 'processing',
    }),
    'processing',
  );
  assert.equal(
    classifyCheckoutEvidence({
      sessionStatus: 'complete',
      paymentStatus: 'paid',
      paymentIntentStatus: 'succeeded',
    }),
    'processing',
  );
});

test('retry creates a replacement only after a durable cancel request is verified as canceled', async () => {
  const calls: unknown[][] = [];
  setStripeForTests({
    paymentIntents: {
      cancel: async (...args: unknown[]) => {
        calls.push(['cancel', ...args]);
        return {};
      },
      retrieve: async (...args: unknown[]) => {
        calls.push(['retrieve', ...args]);
        return { id: 'pi_retry', status: 'canceled' };
      },
    },
  } as any);

  const intent = await cancelPaymentIntentForRetry(
    'pi_retry',
    'billing-cancel-operation-1',
  );
  assert.equal(intent.status, 'canceled');
  assert.deepEqual(calls, [
    [
      'cancel',
      'pi_retry',
      {},
      { idempotencyKey: 'billing-cancel-operation-1', timeout: 20_000 },
    ],
    ['retrieve', 'pi_retry'],
  ]);
  assert.equal(canCreateRetryReplacement(intent.status), true);
  assert.equal(canCreateRetryReplacement('requires_payment_method'), false);
  assert.equal(canCreateRetryReplacement('processing'), false);
  assert.equal(canCreateRetryReplacement('succeeded'), false);
});

test('retry replacement gate remains closed when cancellation retrieval is ambiguous or succeeded', async () => {
  for (const retrievedStatus of [
    'requires_payment_method',
    'processing',
    'succeeded',
  ]) {
    setStripeForTests({
      paymentIntents: {
        cancel: async () => ({ status: 'canceled' }),
        retrieve: async () => ({ id: 'pi_retry', status: retrievedStatus }),
      },
    } as any);
    const retrieved = await cancelPaymentIntentForRetry(
      'pi_retry',
      `cancel-${retrievedStatus}`,
    );
    assert.equal(
      canCreateRetryReplacement(retrieved.status),
      false,
      `${retrievedStatus} must block replacement`,
    );
  }
});

function orderTransitionDb(currentOrder: Record<string, unknown>) {
  const changes: Record<string, unknown>[] = [];
  let predicate: unknown;
  const database = {
    execute: async () => [],
    select() {
      const query = {
        from: () => query,
        where: () => query,
        limit: async () => [currentOrder],
      };
      return query;
    },
    update() {
      const query = {
        set(value: Record<string, unknown>) {
          changes.push(value);
          return query;
        },
        where(value: unknown) {
          predicate = value;
          return query;
        },
        returning: async () => [{ ...currentOrder, ...changes.at(-1) }],
      };
      return query;
    },
  };
  return {
    database,
    changes,
    get predicate() {
      return predicate;
    },
  };
}

test('late reconciliation evidence cannot downgrade an order completed by a winning webhook', async () => {
  const db = orderTransitionDb({
    id: 'order_race',
    userId: user.id,
    state: 'completed',
    version: 8,
  });
  const transitioned = await updateBillingOrderCAS(db.database as any, {
    userId: user.id,
    orderId: 'order_race',
    changes: { state: 'needs_reconciliation' },
  });

  assert.equal(transitioned, null);
  assert.equal(db.changes.length, 0);
});

test('webhook completion persists Stripe identifiers when it wins the create-response race', async () => {
  const db = orderTransitionDb({
    id: 'order_race',
    userId: user.id,
    state: 'creating',
    version: 4,
  });
  const transitioned = await updateBillingOrderCAS(db.database as any, {
    userId: user.id,
    orderId: 'order_race',
    changes: {
      state: 'completed',
      sessionId: 'cs_webhook_first',
      sessionUrl: 'https://checkout.stripe.test/cs_webhook_first',
      paymentIntentId: 'pi_webhook_first',
    },
  });

  assert.equal(transitioned?.sessionId, 'cs_webhook_first');
  assert.equal(transitioned?.paymentIntentId, 'pi_webhook_first');
  assert.ok(db.predicate);
});

test('checkout completion without a durable order is retained for reconciliation', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  installStripe();
  const db = installDb(
    createFakeBillingDb({
      selects: [[user], []],
      existingEvents: ['evt_duplicate'],
    }),
  );

  const event = stripeEvent(
    'checkout.session.completed',
    {
      id: 'cs_123',
      mode: 'subscription',
      customer: 'cus_123',
      subscription: 'sub_123',
      payment_status: 'paid',
      client_reference_id: user.id,
      metadata: { price_id: 'price_pro', user_id: user.id },
    },
    'evt_checkout',
  );

  assert.deepEqual(await processStripeEvent(event), {
    processed: true,
    needsReconciliation: true,
  });
  assert.deepEqual(
    await processStripeEvent({ ...event, id: 'evt_duplicate' }),
    { processed: false },
  );

  const creditInserts = db.inserts.filter(
    (call) => call.table === creditLedger,
  );
  assert.equal(creditInserts.length, 0);
  assert.equal(
    db.inserts.some((call) => call.table === subscriptions),
    false,
  );
});

test('checkout completion cannot infer purchase terms from the current catalog', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  installStripe({
    current_period_end: undefined,
    items: { data: [{ current_period_end: 1_900_000_000 }] },
  });
  const db = installDb(createFakeBillingDb({ selects: [[user], []] }));

  await processStripeEvent(
    stripeEvent(
      'checkout.session.completed',
      {
        id: 'cs_item_period',
        mode: 'subscription',
        customer: 'cus_123',
        subscription: 'sub_123',
        payment_status: 'paid',
        client_reference_id: user.id,
        metadata: { price_id: 'price_pro', user_id: user.id },
      },
      'evt_item_period',
    ),
  );

  assert.equal(
    db.inserts.some((call) => call.table === subscriptions),
    false,
  );
  assert.equal(
    db.inserts.filter((call) => call.table === creditLedger).length,
    0,
  );
});

test('subscription checkout records and grants only when observed subscription facts exactly match approved terms', async () => {
  const terms = approvedTerms();
  const order = {
    id: 'order_sub_checkout',
    userId: user.id,
    attemptKey: '22222222-2222-4222-8222-222222222222',
    intentHash: 'checkout-hash',
    kind: 'subscription',
    priceTermsId: terms.id,
    priceId: terms.priceId,
    tier: terms.tier,
    credits: terms.credits,
    expectedUnitAmount: terms.unitAmount,
    expectedTotalAmount: terms.unitAmount,
    currency: terms.currency,
    quantity: terms.quantity,
    interval: terms.interval,
    intervalCount: terms.intervalCount,
    stripeCustomerId: 'cus_123',
    state: 'creating',
    version: 1,
    leaseGeneration: 1,
    sessionId: null,
    paymentIntentId: null,
  };
  const session = {
    id: 'cs_sub_checkout',
    mode: 'subscription',
    status: 'complete',
    payment_status: 'paid',
    customer: 'cus_123',
    subscription: 'sub_123',
    client_reference_id: user.id,
    amount_subtotal: terms.unitAmount,
    amount_total: terms.unitAmount,
    currency: terms.currency,
    created: 100,
    metadata: {
      order_id: order.id,
      user_id: user.id,
      price_id: terms.priceId,
      billing_kind: 'subscription',
      credits: String(terms.credits),
      tier: terms.tier,
    },
  };
  const subscriptionPrice = {
    id: terms.priceId,
    unit_amount: terms.unitAmount,
    currency: terms.currency,
    billing_scheme: 'per_unit',
    recurring: {
      interval: terms.interval,
      interval_count: terms.intervalCount,
      usage_type: 'licensed',
    },
  };
  const subscription = {
    id: 'sub_123',
    customer: 'cus_123',
    status: 'active',
    metadata: { order_id: order.id, user_id: user.id },
    items: {
      data: [
        {
          quantity: terms.quantity,
          current_period_start: 100,
          current_period_end: 1_800_000_000,
          price: subscriptionPrice,
        },
      ],
    },
  };
  const lineItem = {
    id: 'li_checkout',
    quantity: 1,
    price: {
      id: terms.priceId,
      unit_amount: terms.unitAmount,
      unit_amount_decimal: String(terms.unitAmount),
      billing_scheme: 'per_unit',
    },
    currency: terms.currency,
    amount_total: terms.unitAmount,
  };

  async function runCase(
    subscriptionItem: Record<string, unknown>,
    eventId: string,
    subscriptionItems: Record<string, unknown>[] = [subscriptionItem],
  ) {
    const db = installDb(
      createFakeBillingDb({ orders: [order], billingTerms: [terms] }),
    );
    installStripe({ ...subscription, items: { data: subscriptionItems } }, [
      lineItem,
    ]);
    stripeObjects.set(session.id, session);
    stripeObjects.set(subscription.id, {
      ...subscription,
      items: { data: subscriptionItems },
    });
    const outcome = await processStripeEvent(
      stripeEvent('checkout.session.completed', session, eventId),
    );
    return { db, outcome };
  }

  const valid = await runCase(
    subscription.items.data[0],
    'evt_sub_checkout_valid',
  );
  assert.deepEqual(valid.outcome, { processed: true });
  assert.equal(
    valid.db.inserts.filter((call) => call.table === creditLedger).length,
    1,
  );
  assert.equal(
    valid.db.inserts.filter(
      (call) => call.table === billingSubscriptionPriceVersions,
    ).length,
    1,
  );

  const mismatches = [
    { ...subscription.items.data[0], quantity: 2 },
    {
      ...subscription.items.data[0],
      price: { ...subscriptionPrice, id: 'price_other' },
    },
    {
      ...subscription.items.data[0],
      price: { ...subscriptionPrice, unit_amount: 2000 },
    },
    {
      ...subscription.items.data[0],
      price: { ...subscriptionPrice, currency: 'eur' },
    },
    {
      ...subscription.items.data[0],
      price: { ...subscriptionPrice, billing_scheme: 'tiered' },
    },
    {
      ...subscription.items.data[0],
      price: {
        ...subscriptionPrice,
        recurring: { ...subscriptionPrice.recurring, interval: 'year' },
      },
    },
    {
      ...subscription.items.data[0],
      price: {
        ...subscriptionPrice,
        recurring: { ...subscriptionPrice.recurring, interval_count: 2 },
      },
    },
    {
      ...subscription.items.data[0],
      price: {
        ...subscriptionPrice,
        recurring: { ...subscriptionPrice.recurring, usage_type: 'metered' },
      },
    },
  ];
  for (const [index, subscriptionItem] of mismatches.entries()) {
    const result = await runCase(
      subscriptionItem,
      `evt_sub_checkout_mismatch_${index}`,
    );
    assert.deepEqual(result.outcome, {
      processed: true,
      needsReconciliation: true,
    });
    assert.equal(
      result.db.inserts.filter((call) => call.table === creditLedger).length,
      0,
    );
    assert.equal(
      result.db.inserts.filter(
        (call) => call.table === billingSubscriptionPriceVersions,
      ).length,
      0,
    );
    assert.equal(
      result.db.updates.some(
        (call) =>
          call.table === billingOrders &&
          call.values.state === 'needs_reconciliation',
      ),
      true,
    );
  }

  const extraItemCase = await runCase(
    subscription.items.data[0],
    'evt_sub_checkout_extra_item',
    [subscription.items.data[0], subscription.items.data[0]],
  );
  assert.deepEqual(extraItemCase.outcome, {
    processed: true,
    needsReconciliation: true,
  });
  assert.equal(
    extraItemCase.db.inserts.filter((call) => call.table === creditLedger)
      .length,
    0,
  );
  assert.equal(
    extraItemCase.db.inserts.filter(
      (call) => call.table === billingSubscriptionPriceVersions,
    ).length,
    0,
  );
  assert.equal(
    extraItemCase.db.updates.some(
      (call) =>
        call.table === billingOrders &&
        call.values.state === 'needs_reconciliation',
    ),
    true,
  );
});

test('invoice.paid without a stored subscription price version requires reconciliation', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  installStripe();
  const db = installDb(createFakeBillingDb({ selects: [[user], []] }));

  assert.deepEqual(
    await processStripeEvent(
      stripeEvent(
        'invoice.paid',
        {
          id: 'in_renewal',
          customer: 'cus_123',
          subscription: 'sub_123',
          billing_reason: 'subscription_cycle',
          lines: { data: [{ price: { id: 'price_pro' } }] },
        },
        'evt_invoice',
      ),
    ),
    { processed: true, needsReconciliation: true },
  );

  const creditInserts = db.inserts.filter(
    (call) => call.table === creditLedger,
  );
  assert.equal(creditInserts.length, 0);
});

test('invoice renewal rejects proration and tax or credit adjustments before granting credits', async () => {
  installStripe();
  const db = installDb(createFakeBillingDb());
  const unsupportedInvoices = [
    { id: 'in_prorated', lines: { data: [{ proration: true }] } },
    { id: 'in_taxed', total_taxes: [{ amount: 100 }], lines: { data: [{}] } },
    {
      id: 'in_discounted',
      total_discount_amounts: [{ amount: 100 }],
      lines: { data: [{}] },
    },
    {
      id: 'in_credited',
      pre_payment_credit_notes_amount: 100,
      lines: { data: [{}] },
    },
  ];

  for (const invoice of unsupportedInvoices) {
    Object.assign(invoice, {
      customer: 'cus_123',
      subscription: 'sub_123',
      billing_reason: 'subscription_cycle',
    });
    stripeObjects.set(invoice.id, invoice);
    assert.deepEqual(
      await processStripeEvent(
        stripeEvent('invoice.paid', invoice, `evt_${invoice.id}`),
      ),
      {
        processed: true,
        needsReconciliation: true,
      },
    );
  }
  assert.equal(
    db.inserts.filter((call) => call.table === creditLedger).length,
    0,
  );
});

test('async payment failure compares signed and retrieved evidence and classifies matching recovery states', () => {
  const base = {
    id: 'cs_async',
    status: 'complete',
    payment_status: 'unpaid',
    payment_intent: 'pi_async',
  } as any;
  assert.equal(
    classifyAsyncPaymentFailure({
      signed: { ...base, payment_status: 'paid' },
      current: base,
      intent: { id: 'pi_async', status: 'requires_payment_method' } as any,
      eventType: 'checkout.session.async_payment_failed',
    }),
    'needs_reconciliation',
  );
  assert.equal(
    classifyAsyncPaymentFailure({
      signed: base,
      current: base,
      intent: { id: 'pi_async', status: 'succeeded' } as any,
      eventType: 'checkout.session.async_payment_failed',
    }),
    'needs_reconciliation',
  );
  assert.equal(
    classifyAsyncPaymentFailure({
      signed: base,
      current: base,
      intent: { id: 'pi_async', status: 'requires_payment_method' } as any,
      eventType: 'checkout.session.async_payment_failed',
    }),
    'retry_required',
  );
  assert.equal(
    classifyAsyncPaymentFailure({
      signed: { ...base, status: 'open' },
      current: { ...base, status: 'open' },
    }),
    'open',
  );
});

test('subscription deletion updates status and plan without revoking paid credits', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  installStripe({ status: 'canceled' });
  const db = installDb(createFakeBillingDb({ selects: [[user]] }));

  await processStripeEvent(
    stripeEvent(
      'customer.subscription.deleted',
      {
        id: 'sub_123',
        customer: 'cus_123',
        status: 'canceled',
        current_period_end: 1_800_000_000,
        items: { data: [{ price: { id: 'price_pro' } }] },
      },
      'evt_deleted',
    ),
  );

  assert.equal(
    db.inserts.filter((call) => call.table === creditLedger).length,
    0,
  );
  assert.equal(
    db.inserts.some(
      (call) =>
        call.table === subscriptions && call.values.status === 'canceled',
    ),
    true,
  );
  assert.equal(
    db.updates.some(
      (call) => call.table === users && call.values.plan === 'free',
    ),
    true,
  );
});

test('subscription checkout rejects already active subscribers', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    {
      priceId: 'price_pro',
      tier: 'pro',
      credits: 1000,
      amount: 1000,
      currency: 'usd',
      interval: 'month',
    },
  ]);
  installStripe();
  installDb(
    createFakeBillingDb({
      subscriptionRows: [{ status: 'active' }],
      billingTerms: [approvedTerms()],
    }),
  );

  await assert.rejects(
    () =>
      createBillingCheckout({
        user,
        kind: 'subscription',
        priceId: 'price_pro',
      }),
    (error) =>
      error instanceof BillingEligibilityError &&
      error.type === 'already_subscribed',
  );
});

test('top-up checkout requires an active subscription', async () => {
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([
    { priceId: 'price_topup', credits: 500, amount: 500, currency: 'usd' },
  ]);
  installStripe();
  installDb(
    createFakeBillingDb({
      subscriptionRows: [],
      billingTerms: [
        approvedTerms({
          id: '00000000-0000-4000-8000-000000000002',
          priceId: 'price_topup',
          kind: 'top_up',
          tier: null,
          credits: 500,
          unitAmount: 500,
          interval: null,
          intervalCount: null,
        }),
      ],
    }),
  );

  await assert.rejects(
    () =>
      createBillingCheckout({ user, kind: 'top_up', priceId: 'price_topup' }),
    (error) =>
      error instanceof BillingEligibilityError &&
      error.type === 'top_up_requires_subscription',
  );
});

test('payment intent without an order is retained for reconciliation', async () => {
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([
    { priceId: 'price_topup', credits: 500 },
  ]);
  installStripe();
  const db = installDb(
    createFakeBillingDb({ selects: [[user], [{ status: 'active' }], []] }),
  );

  assert.deepEqual(
    await processStripeEvent(
      stripeEvent(
        'payment_intent.succeeded',
        {
          id: 'pi_123',
          customer: 'cus_123',
          metadata: {
            billing_kind: 'top_up',
            user_id: user.id,
            price_id: 'price_topup',
          },
        },
        'evt_topup',
      ),
    ),
    { processed: true, needsReconciliation: true },
  );

  const creditInserts = db.inserts.filter(
    (call) => call.table === creditLedger,
  );
  assert.equal(creditInserts.length, 0);
});

test('top-up webhook does not grant credits without an active subscription', async () => {
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([
    { priceId: 'price_topup', credits: 500 },
  ]);
  installStripe();
  const db = installDb(createFakeBillingDb({ selects: [[user], []] }));

  await processStripeEvent(
    stripeEvent(
      'payment_intent.succeeded',
      {
        id: 'pi_123',
        customer: 'cus_123',
        metadata: {
          billing_kind: 'top_up',
          user_id: user.id,
          price_id: 'price_topup',
        },
      },
      'evt_topup_without_subscription',
    ),
  );

  assert.equal(
    db.inserts.filter((call) => call.table === creditLedger).length,
    0,
  );
});

test('distinct Stripe events do not double-grant an already credited payment object', async () => {
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([
    { priceId: 'price_topup', credits: 500 },
  ]);
  installStripe();
  const db = installDb(
    createFakeBillingDb({
      selects: [[user], [{ status: 'active' }]],
      existingLedgerRefs: [`${user.id}:stripe_top_up:pi_123`],
    }),
  );

  await processStripeEvent(
    stripeEvent(
      'payment_intent.succeeded',
      {
        id: 'pi_123',
        customer: 'cus_123',
        metadata: {
          billing_kind: 'top_up',
          user_id: user.id,
          price_id: 'price_topup',
        },
      },
      'evt_topup_retry_shape',
    ),
  );

  assert.equal(
    db.inserts.filter((call) => call.table === creditLedger).length,
    0,
  );
});

function subscriptionStatusEvent(
  type: string,
  status: string,
  id: string,
  created: number,
) {
  return stripeEvent(
    type,
    {
      id: 'sub_123',
      customer: 'cus_123',
      status,
      current_period_end: 1_800_000_000,
      items: {
        data: [
          {
            quantity: 1,
            current_period_start: 100,
            current_period_end: 1_800_000_000,
            price: {
              id: 'price_pro',
              unit_amount: 1000,
              currency: 'usd',
              billing_scheme: 'per_unit',
              recurring: {
                interval: 'month',
                interval_count: 1,
                usage_type: 'licensed',
              },
            },
          },
        ],
      },
    },
    id,
    created,
  );
}

function lastSubscriptionStatus(db: FakeDb): string | undefined {
  return db.inserts.filter((call) => call.table === subscriptions).at(-1)
    ?.values.status;
}

test('newer subscription event applies and reactivation restores paid plan', async () => {
  installStripe();
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  const db = installDb(
    createFakeBillingDb({
      selects: [[user], [user]],
      trackSubscriptions: true,
      billingTerms: [approvedTerms()],
    }),
  );

  await processStripeEvent(
    subscriptionStatusEvent(
      'customer.subscription.deleted',
      'canceled',
      'evt_del_1',
      100,
    ),
  );
  await processStripeEvent(
    subscriptionStatusEvent(
      'customer.subscription.updated',
      'active',
      'evt_upd_2',
      200,
    ),
  );

  assert.equal(lastSubscriptionStatus(db), 'active');
  assert.equal(
    db.updates.filter((call) => call.table === users).at(-1)?.values.plan,
    'pro',
  );
});

test('subscription update reconciles changed item quantity and never records an unapproved version', async () => {
  installStripe();
  const event = subscriptionStatusEvent(
    'customer.subscription.updated',
    'active',
    'evt_quantity_mismatch',
    200,
  );
  (event.data.object as any).items.data[0].quantity = 2;
  stripeObjects.set('sub_123', event.data.object);
  const db = installDb(
    createFakeBillingDb({
      selects: [[user]],
      billingTerms: [approvedTerms()],
    }),
  );

  assert.deepEqual(await processStripeEvent(event), {
    processed: true,
    needsReconciliation: true,
  });
  assert.equal(
    db.inserts.filter((call) => call.table === billingSubscriptionPriceVersions)
      .length,
    0,
  );
  assert.equal(
    db.inserts.filter((call) => call.table === subscriptions).length,
    0,
  );
});

test('subscription update orders approval by approvedAt rather than row creation time', async () => {
  installStripe();
  const db = installDb(
    createFakeBillingDb({
      selects: [[user]],
      billingTerms: [
        approvedTerms({
          createdAt: new Date(0),
          approvedAt: new Date(300_000),
        }),
      ],
    }),
  );
  assert.deepEqual(
    await processStripeEvent(
      subscriptionStatusEvent(
        'customer.subscription.updated',
        'active',
        'evt_not_yet_approved',
        200,
      ),
    ),
    { processed: true, needsReconciliation: true },
  );
  assert.equal(
    db.inserts.filter((call) => call.table === billingSubscriptionPriceVersions)
      .length,
    0,
  );
});

test('update then delete ends canceled and free regardless of tier resolution', async () => {
  installStripe();
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  const db = installDb(
    createFakeBillingDb({
      selects: [[user], [user]],
      trackSubscriptions: true,
      billingTerms: [approvedTerms()],
    }),
  );

  await processStripeEvent(
    subscriptionStatusEvent(
      'customer.subscription.updated',
      'active',
      'evt_upd_a',
      100,
    ),
  );
  await processStripeEvent(
    subscriptionStatusEvent(
      'customer.subscription.deleted',
      'canceled',
      'evt_del_b',
      200,
    ),
  );

  assert.equal(lastSubscriptionStatus(db), 'canceled');
  assert.equal(
    db.updates.filter((call) => call.table === users).at(-1)?.values.plan,
    'free',
  );
});

test('older update after newer delete is a stale no-op keeping the user free', async () => {
  installStripe();
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  const db = installDb(
    createFakeBillingDb({
      selects: [[user], [user]],
      trackSubscriptions: true,
    }),
  );

  await processStripeEvent(
    subscriptionStatusEvent(
      'customer.subscription.deleted',
      'canceled',
      'evt_del_new',
      200,
    ),
  );
  await processStripeEvent(
    subscriptionStatusEvent(
      'customer.subscription.updated',
      'active',
      'evt_upd_old',
      100,
    ),
  );

  assert.equal(lastSubscriptionStatus(db), 'canceled');
  assert.equal(
    db.updates.some(
      (call) => call.table === users && call.values.plan === 'pro',
    ),
    false,
  );
});

test('exact timestamp tie does not let update resurrect a terminal deleted subscription', async () => {
  installStripe();
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  const db = installDb(
    createFakeBillingDb({
      selects: [[user], [user]],
      trackSubscriptions: true,
    }),
  );

  await processStripeEvent(
    subscriptionStatusEvent(
      'customer.subscription.deleted',
      'canceled',
      'evt_del_tie',
      500,
    ),
  );
  await processStripeEvent(
    subscriptionStatusEvent(
      'customer.subscription.updated',
      'active',
      'evt_upd_tie',
      500,
    ),
  );

  assert.equal(lastSubscriptionStatus(db), 'canceled');
  assert.equal(
    db.updates.some(
      (call) => call.table === users && call.values.plan === 'pro',
    ),
    false,
  );
});

test('duplicate subscription event id is a processed no-op', async () => {
  installStripe();
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  const db = installDb(
    createFakeBillingDb({
      selects: [[user]],
      trackSubscriptions: true,
      billingTerms: [approvedTerms()],
    }),
  );

  const event = subscriptionStatusEvent(
    'customer.subscription.updated',
    'active',
    'evt_dup',
    100,
  );
  assert.deepEqual(await processStripeEvent(event), { processed: true });
  assert.deepEqual(await processStripeEvent(event), { processed: false });
  assert.equal(
    db.inserts.filter((call) => call.table === subscriptions).length,
    1,
  );
});

test('handler error rolls back so the event id is not permanently recorded', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: 'price_pro', tier: 'pro', credits: 1000 },
  ]);
  const db = installDb(createFakeBillingDb({ selects: [[user]] }));
  const event = stripeEvent(
    'invoice.paid',
    {
      id: 'in_rollback',
      customer: 'cus_123',
      subscription: 'sub_123',
      billing_reason: 'subscription_cycle',
      lines: { data: [{ price: { id: 'price_pro' } }] },
    },
    'evt_rollback',
  );

  setStripeForTests({
    subscriptions: {
      retrieve: async () => {
        throw new Error('stripe unavailable');
      },
    },
    webhooks: {
      constructEvent: () => {
        throw new Error('bad signature');
      },
    },
  } as any);
  await assert.rejects(() => processStripeEvent(event));
  assert.equal(
    db.inserts.some((call) => call.table === creditLedger),
    false,
  );

  installStripe();
  assert.deepEqual(await processStripeEvent(event), {
    processed: true,
    needsReconciliation: true,
  });
  assert.equal(
    db.inserts.filter((call) => call.table === creditLedger).length,
    0,
  );
});

test('billing POST guard rejects cross-origin cookie-auth posts', async () => {
  const response = rejectCrossOriginBillingPost(
    new Request('https://api.nvm.fyi/api/billing/checkout', {
      method: 'POST',
      headers: {
        origin: 'https://evil.example',
        'content-type': 'application/json',
      },
      body: '{}',
    }),
  );

  assert.equal(response?.status, 403);
  assert.deepEqual(await response?.json(), {
    error: {
      type: 'forbidden',
      message: 'Cross-origin billing request rejected',
    },
  });
});

test('webhook route rejects malformed or signature-invalid payloads', async () => {
  process.env.STRIPE_WEBHOOK_SECRET = 'whsec_test';
  installStripe();

  const response = await postWebhook({
    request: new Request('https://api.nvm.fyi/api/billing/webhook', {
      method: 'POST',
      headers: { 'stripe-signature': 'bad' },
      body: '{}',
    }),
  } as any);

  assert.equal(response.status, 400);
  assert.deepEqual(await response.json(), {
    error: {
      type: 'invalid_webhook',
      message: 'Invalid Stripe webhook signature or payload',
    },
  });
});

test('checkout lock prevents concurrent subscription sessions for the same user', async () => {
  const locks = new Map<string, boolean>();
  setCheckoutLockForTests({
    acquire: (userId: string) => {
      if (locks.has(userId)) return { ok: false };
      locks.set(userId, true);
      return { ok: true };
    },
    release: (userId: string) => {
      locks.delete(userId);
    },
  });

  assert.deepEqual(await acquireCheckoutLock(user.id), { ok: true });
  assert.deepEqual(await acquireCheckoutLock(user.id), { ok: false });

  releaseCheckoutLock(user.id);
  assert.deepEqual(await acquireCheckoutLock(user.id), { ok: true });

  assert.deepEqual(await acquireCheckoutLock('another_user'), { ok: true });

  setCheckoutLockForTests(null);
});

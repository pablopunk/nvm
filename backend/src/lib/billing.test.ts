import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { setDbForTests, resetDbForTests } from '../db/client';
import { creditLedger, stripeEvents, subscriptions, users } from '../db/schema';
import { BillingEligibilityError, createBillingCheckout, processStripeEvent, rejectCrossOriginBillingPost, setStripeForTests } from './billing';
import { POST as postWebhook } from '../pages/api/billing/webhook';

const user = {
  id: '11111111-1111-1111-1111-111111111111',
  workosUserId: 'workos_1',
  email: 'pablo@example.com',
  plan: 'free',
  role: 'user',
  stripeCustomerId: 'cus_123',
  createdAt: new Date(),
};

type FakeDb = ReturnType<typeof createFakeBillingDb>;

type InsertCall = { table: unknown; values: any; conflict?: 'nothing' | 'update' };
type UpdateCall = { table: unknown; values: any };

function createFakeBillingDb(input: { selects?: unknown[]; existingEvents?: string[]; existingLedgerRefs?: string[] } = {}) {
  const selects = [...(input.selects ?? [])];
  const eventIds = new Set(input.existingEvents ?? []);
  const ledgerRefs = new Set(input.existingLedgerRefs ?? []);
  const inserts: InsertCall[] = [];
  const updates: UpdateCall[] = [];

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
      then(resolveThen: Parameters<Promise<unknown>['then']>[0], rejectThen: Parameters<Promise<unknown>['then']>[1]) {
        if (table !== stripeEvents && conflict !== 'update') {
          const ledgerRef = table === creditLedger && values?.refId ? `${values.userId}:${values.reason}:${values.refId}` : null;
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
    const promise = () => Promise.resolve(selects.shift() ?? []);
    const chain = {
      from: () => chain,
      where: () => chain,
      limit: () => promise(),
      then: (resolveThen: Parameters<Promise<unknown>['then']>[0], rejectThen: Parameters<Promise<unknown>['then']>[1]) => promise().then(resolveThen, rejectThen),
      catch: (rejectThen: Parameters<Promise<unknown>['catch']>[0]) => promise().catch(rejectThen),
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
        return Promise.resolve([{ stripeCustomerId: values?.stripeCustomerId }]);
      },
      then(resolveThen: Parameters<Promise<unknown>['then']>[0], rejectThen: Parameters<Promise<unknown>['then']>[1]) {
        return promise().then(resolveThen, rejectThen);
      },
      catch(rejectThen: Parameters<Promise<unknown>['catch']>[0]) {
        return promise().catch(rejectThen);
      },
    };
    return chain;
  }

  const db = {
    inserts,
    updates,
    select: () => createSelectChain(),
    insert: (table: unknown) => createInsertChain(table),
    update: (table: unknown) => createUpdateChain(table),
    transaction: async (callback: (tx: FakeDb) => Promise<unknown>) => callback(db as FakeDb),
  };
  return db;
}

function installDb(db: FakeDb) {
  setDbForTests(db as any);
  return db;
}

function installStripe(subscription: Record<string, unknown> = {}) {
  setStripeForTests({
    subscriptions: {
      retrieve: async (id: string) => ({
        id,
        status: subscription.status ?? 'active',
        current_period_end: subscription.current_period_end ?? 1_800_000_000,
      }),
    },
    webhooks: {
      constructEvent: () => {
        throw new Error('bad signature');
      },
    },
  } as any);
}

function stripeEvent(type: string, object: Record<string, unknown>, id = `evt_${type}`) {
  return {
    id,
    object: 'event',
    api_version: '2025-01-27.acacia',
    created: 1,
    livemode: false,
    pending_webhooks: 1,
    request: null,
    type,
    data: { object },
  } as any;
}

afterEach(() => {
  resetDbForTests();
  setStripeForTests(null);
  delete process.env.STRIPE_SUBSCRIPTION_TIERS;
  delete process.env.STRIPE_TOP_UP_PACKS;
  delete process.env.STRIPE_WEBHOOK_SECRET;
});

test('checkout completion upserts subscription and grants initial paid credits once', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([{ priceId: 'price_pro', tier: 'pro', credits: 1000 }]);
  installStripe();
  const db = installDb(createFakeBillingDb({ selects: [[user], []], existingEvents: ['evt_duplicate'] }));

  const event = stripeEvent('checkout.session.completed', {
    id: 'cs_123',
    mode: 'subscription',
    customer: 'cus_123',
    subscription: 'sub_123',
    payment_status: 'paid',
    client_reference_id: user.id,
    metadata: { price_id: 'price_pro', user_id: user.id },
  }, 'evt_checkout');

  assert.deepEqual(await processStripeEvent(event), { processed: true });
  assert.deepEqual(await processStripeEvent({ ...event, id: 'evt_duplicate' }), { processed: false });

  const creditInserts = db.inserts.filter((call) => call.table === creditLedger);
  assert.equal(creditInserts.length, 1);
  assert.equal(creditInserts[0].values.delta, 1000);
  assert.equal(creditInserts[0].values.kind, 'paid');
  assert.equal(creditInserts[0].values.reason, 'stripe_checkout_subscription');
  assert.equal(creditInserts[0].values.refId, 'cs_123');
  assert.equal(db.inserts.some((call) => call.table === subscriptions && call.values.status === 'active' && call.values.tier === 'pro'), true);
  assert.equal(db.updates.some((call) => call.table === users && call.values.plan === 'pro'), true);
});

test('invoice.paid grants renewal credits only for subscription cycle invoices', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([{ priceId: 'price_pro', tier: 'pro', credits: 1000 }]);
  installStripe();
  const db = installDb(createFakeBillingDb({ selects: [[user], []] }));

  await processStripeEvent(stripeEvent('invoice.paid', {
    id: 'in_renewal',
    customer: 'cus_123',
    subscription: 'sub_123',
    billing_reason: 'subscription_cycle',
    lines: { data: [{ price: { id: 'price_pro' } }] },
  }, 'evt_invoice'));

  const creditInserts = db.inserts.filter((call) => call.table === creditLedger);
  assert.equal(creditInserts.length, 1);
  assert.equal(creditInserts[0].values.reason, 'stripe_subscription_renewal');
  assert.equal(creditInserts[0].values.refId, 'in_renewal');
  assert.equal(creditInserts[0].values.delta, 1000);
});

test('subscription deletion updates status and plan without revoking paid credits', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([{ priceId: 'price_pro', tier: 'pro', credits: 1000 }]);
  installStripe({ status: 'canceled' });
  const db = installDb(createFakeBillingDb({ selects: [[user]] }));

  await processStripeEvent(stripeEvent('customer.subscription.deleted', {
    id: 'sub_123',
    customer: 'cus_123',
    status: 'canceled',
    current_period_end: 1_800_000_000,
    items: { data: [{ price: { id: 'price_pro' } }] },
  }, 'evt_deleted'));

  assert.equal(db.inserts.filter((call) => call.table === creditLedger).length, 0);
  assert.equal(db.inserts.some((call) => call.table === subscriptions && call.values.status === 'canceled'), true);
  assert.equal(db.updates.some((call) => call.table === users && call.values.plan === 'free'), true);
});

test('subscription checkout rejects already active subscribers', async () => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([{ priceId: 'price_pro', tier: 'pro', credits: 1000 }]);
  installDb(createFakeBillingDb({ selects: [[{ status: 'active' }]] }));

  await assert.rejects(
    () => createBillingCheckout({ user, kind: 'subscription', priceId: 'price_pro' }),
    (error) => error instanceof BillingEligibilityError && error.type === 'already_subscribed',
  );
});

test('top-up checkout requires an active subscription', async () => {
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([{ priceId: 'price_topup', credits: 500 }]);
  installDb(createFakeBillingDb({ selects: [[]] }));

  await assert.rejects(
    () => createBillingCheckout({ user, kind: 'top_up', priceId: 'price_topup' }),
    (error) => error instanceof BillingEligibilityError && error.type === 'top_up_requires_subscription',
  );
});

test('payment intent succeeded grants one-time top-up credits from metadata for active subscribers', async () => {
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([{ priceId: 'price_topup', credits: 500 }]);
  installStripe();
  const db = installDb(createFakeBillingDb({ selects: [[user], [{ status: 'active' }], []] }));

  await processStripeEvent(stripeEvent('payment_intent.succeeded', {
    id: 'pi_123',
    customer: 'cus_123',
    metadata: { billing_kind: 'top_up', user_id: user.id, price_id: 'price_topup' },
  }, 'evt_topup'));

  const creditInserts = db.inserts.filter((call) => call.table === creditLedger);
  assert.equal(creditInserts.length, 1);
  assert.equal(creditInserts[0].values.reason, 'stripe_top_up');
  assert.equal(creditInserts[0].values.refId, 'pi_123');
  assert.equal(creditInserts[0].values.delta, 500);
});

test('top-up webhook does not grant credits without an active subscription', async () => {
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([{ priceId: 'price_topup', credits: 500 }]);
  installStripe();
  const db = installDb(createFakeBillingDb({ selects: [[user], []] }));

  await processStripeEvent(stripeEvent('payment_intent.succeeded', {
    id: 'pi_123',
    customer: 'cus_123',
    metadata: { billing_kind: 'top_up', user_id: user.id, price_id: 'price_topup' },
  }, 'evt_topup_without_subscription'));

  assert.equal(db.inserts.filter((call) => call.table === creditLedger).length, 0);
});

test('distinct Stripe events do not double-grant an already credited payment object', async () => {
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify([{ priceId: 'price_topup', credits: 500 }]);
  installStripe();
  const db = installDb(createFakeBillingDb({
    selects: [[user], [{ status: 'active' }]],
    existingLedgerRefs: [`${user.id}:stripe_top_up:pi_123`],
  }));

  await processStripeEvent(stripeEvent('payment_intent.succeeded', {
    id: 'pi_123',
    customer: 'cus_123',
    metadata: { billing_kind: 'top_up', user_id: user.id, price_id: 'price_topup' },
  }, 'evt_topup_retry_shape'));

  assert.equal(db.inserts.filter((call) => call.table === creditLedger).length, 0);
});

test('billing POST guard rejects cross-origin cookie-auth posts', async () => {
  const response = rejectCrossOriginBillingPost(new Request('https://api.nvm.fyi/api/billing/checkout', {
    method: 'POST',
    headers: { origin: 'https://evil.example', 'content-type': 'application/json' },
    body: '{}',
  }));

  assert.equal(response?.status, 403);
  assert.deepEqual(await response?.json(), {
    error: { type: 'forbidden', message: 'Cross-origin billing request rejected' },
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
    error: { type: 'invalid_webhook', message: 'Invalid Stripe webhook signature or payload' },
  });
});

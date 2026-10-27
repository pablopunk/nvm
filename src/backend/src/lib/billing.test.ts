import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import type Stripe from 'stripe';
import { resetDbForTests, setDbForTests } from '../db/client';
import { creditLedger, stripeEvents, subscriptions, users } from '../db/schema';
import {
  billingCatalog,
  findCatalogItem,
  formatBillingPrice,
  processStripeEvent,
  setStripeForTests,
  type StripeLike,
} from './billing';

const PRO_PRICE = 'price_pro';
const TOP_UP_PRICE = 'price_top';

type Write = { op: 'insert' | 'update'; table: unknown; values: any };

/** Records writes and serves canned rows per table; WHERE clauses are ignored. */
function installFakeDb(rows: Map<unknown, unknown[]> = new Map()) {
  const writes: Write[] = [];
  function chain(result: () => unknown) {
    const builder: any = {
      from(table: unknown) {
        builder.table = table;
        return builder;
      },
      where: () => builder,
      limit: () => builder,
      onConflictDoNothing: () => builder,
      onConflictDoUpdate: () => builder,
      returning: () => builder,
      then(resolve: (value: unknown) => unknown, reject: (error: unknown) => unknown) {
        return Promise.resolve().then(() => result.call(builder)).then(resolve, reject);
      },
    };
    return builder;
  }
  const database: any = {
    select: () =>
      chain(function selectRows(this: { table: unknown }) {
        return rows.get(this.table) ?? [];
      }),
    insert: (table: unknown) => {
      const builder = chain(() =>
        table === stripeEvents ? [{ eventId: 'evt' }] : [],
      );
      builder.values = (values: unknown) => {
        writes.push({ op: 'insert', table, values });
        return builder;
      };
      return builder;
    },
    update: (table: unknown) => {
      const builder = chain(() => []);
      builder.set = (values: unknown) => {
        writes.push({ op: 'update', table, values });
        return builder;
      };
      return builder;
    },
    transaction: (run: (tx: unknown) => unknown) => run(database),
  };
  setDbForTests(database);
  return writes;
}

function stripeSubscription(overrides: Partial<Stripe.Subscription> = {}) {
  return {
    id: 'sub_1',
    status: 'active',
    customer: 'cus_1',
    metadata: { user_id: 'user-1' },
    items: { data: [{ price: { id: PRO_PRICE }, current_period_end: 1_800_000_000 }] },
    ...overrides,
  } as unknown as Stripe.Subscription;
}

function installFakeStripe(subscription: Stripe.Subscription) {
  setStripeForTests({
    subscriptions: { retrieve: async () => subscription },
  } as unknown as StripeLike);
}

function event(type: string, object: Record<string, unknown>): Stripe.Event {
  return { id: `evt_${type}`, type, api_version: '2020-08-27', data: { object } } as unknown as Stripe.Event;
}

function ledgerGrants(writes: Write[]) {
  return writes.filter((write) => write.op === 'insert' && write.table === creditLedger).map((write) => write.values);
}

function planUpdates(writes: Write[]) {
  return writes
    .filter((write) => write.op === 'update' && write.table === users && 'plan' in write.values)
    .map((write) => write.values.plan);
}

beforeEach(() => {
  process.env.STRIPE_SUBSCRIPTION_TIERS = JSON.stringify([
    { priceId: PRO_PRICE, tier: 'pro', credits: 1000, amount: 1000, currency: 'usd', interval: 'month' },
  ]);
  process.env.STRIPE_TOP_UP_PACKS = JSON.stringify({ [TOP_UP_PRICE]: { credits: 500 } });
});

afterEach(() => {
  resetDbForTests();
  setStripeForTests(null);
  delete process.env.STRIPE_SUBSCRIPTION_TIERS;
  delete process.env.STRIPE_TOP_UP_PACKS;
});

test('catalog maps price IDs to credits and ignores legacy amount fields', () => {
  assert.deepEqual(billingCatalog(), {
    subscriptions: [{ kind: 'subscription', priceId: PRO_PRICE, tier: 'pro', credits: 1000 }],
    topUps: [{ kind: 'top_up', priceId: TOP_UP_PRICE, credits: 500 }],
  });
  assert.equal(findCatalogItem({ kind: 'top_up', priceId: 'price_unknown' }), null);
  assert.equal(findCatalogItem({ kind: 'subscription' })?.priceId, PRO_PRICE);
});

test('formatBillingPrice uses Stripe minor units', () => {
  assert.equal(formatBillingPrice({ amount: 1000, currency: 'usd' }), '$10.00');
});

test('subscription checkout grants credits under the session key and upgrades the plan', async () => {
  const writes = installFakeDb();
  installFakeStripe(stripeSubscription());
  const result = await processStripeEvent(
    event('checkout.session.completed', {
      id: 'cs_1',
      mode: 'subscription',
      payment_status: 'paid',
      customer: 'cus_1',
      subscription: 'sub_1',
      metadata: { user_id: 'user-1' },
    }),
  );
  assert.equal(result.processed, true);
  assert.deepEqual(ledgerGrants(writes), [
    { userId: 'user-1', delta: 1000, kind: 'paid', reason: 'stripe_checkout_subscription', refId: 'cs_1' },
  ]);
  assert.ok(writes.some((write) => write.table === subscriptions && write.values.stripeSubId === 'sub_1'));
  assert.deepEqual(planUpdates(writes), ['pro']);
});

test('top-up checkout grants the pack credits under the payment intent key', async () => {
  const writes = installFakeDb();
  installFakeStripe(stripeSubscription());
  await processStripeEvent(
    event('checkout.session.completed', {
      id: 'cs_2',
      mode: 'payment',
      payment_status: 'paid',
      payment_intent: 'pi_1',
      metadata: { user_id: 'user-1', price_id: TOP_UP_PRICE },
    }),
  );
  assert.deepEqual(ledgerGrants(writes), [
    { userId: 'user-1', delta: 500, kind: 'paid', reason: 'stripe_top_up', refId: 'pi_1' },
  ]);
});

test('unpaid async checkouts wait for the payment-succeeded event', async () => {
  const writes = installFakeDb();
  await processStripeEvent(
    event('checkout.session.completed', { id: 'cs_3', mode: 'payment', payment_status: 'unpaid', metadata: { user_id: 'user-1' } }),
  );
  assert.deepEqual(ledgerGrants(writes), []);
});

test('an unknown price fails the event so Stripe retries it', async () => {
  installFakeDb();
  installFakeStripe(stripeSubscription({ items: { data: [{ price: { id: 'price_other' } }] } } as any));
  await assert.rejects(
    processStripeEvent(
      event('checkout.session.completed', {
        id: 'cs_4',
        mode: 'subscription',
        payment_status: 'paid',
        subscription: 'sub_1',
        metadata: { user_id: 'user-1' },
      }),
    ),
    /not in STRIPE_SUBSCRIPTION_TIERS/,
  );
});

test('renewal invoices grant credits; the first invoice does not', async () => {
  const writes = installFakeDb(new Map([[subscriptions, [{ userId: 'user-1' }]]]));
  installFakeStripe(stripeSubscription());
  await processStripeEvent(event('invoice.paid', { id: 'in_first', billing_reason: 'subscription_create', subscription: 'sub_1' }));
  assert.deepEqual(ledgerGrants(writes), []);
  await processStripeEvent(
    event('invoice.paid', { id: 'in_2', billing_reason: 'subscription_cycle', parent: { subscription_details: { subscription: 'sub_1' } } }),
  );
  assert.deepEqual(ledgerGrants(writes), [
    { userId: 'user-1', delta: 1000, kind: 'paid', reason: 'stripe_subscription_renewal', refId: 'in_2' },
  ]);
});

test('a canceled subscription downgrades the plan', async () => {
  const writes = installFakeDb(new Map([[subscriptions, [{ userId: 'user-1', stripeSubId: 'sub_1', status: 'active', tier: 'pro' }]]]));
  installFakeStripe(stripeSubscription({ status: 'canceled' }));
  await processStripeEvent(event('customer.subscription.deleted', { id: 'sub_1' }));
  assert.deepEqual(planUpdates(writes), ['free']);
});

test('a late cancel of an old subscription does not replace a newer active one', async () => {
  const writes = installFakeDb(new Map([[subscriptions, [{ userId: 'user-1', stripeSubId: 'sub_new', status: 'active', tier: 'pro' }]]]));
  installFakeStripe(stripeSubscription({ id: 'sub_old', status: 'canceled' }));
  await processStripeEvent(event('customer.subscription.deleted', { id: 'sub_old' }));
  assert.deepEqual(planUpdates(writes), []);
  assert.ok(!writes.some((write) => write.table === subscriptions));
});

test('an already-stored event is skipped', async () => {
  const writes = installFakeDb(new Map([[stripeEvents, [{ eventId: 'evt_invoice.paid' }]]]));
  const result = await processStripeEvent(event('invoice.paid', { id: 'in_3', billing_reason: 'subscription_cycle' }));
  assert.equal(result.processed, false);
  assert.deepEqual(writes, []);
});

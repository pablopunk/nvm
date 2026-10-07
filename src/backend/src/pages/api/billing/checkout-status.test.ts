import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { afterEach, test, type TestContext } from 'node:test';

process.env.WORKOS_API_KEY = 'sk_test_billing_status';
process.env.WORKOS_CLIENT_ID = 'client_test_billing_status';
process.env.WORKOS_COOKIE_PASSWORD = 'billing-status-cookie-password-with-32-characters';

const [
  { GET, POST },
  { POST: checkoutPost },
  { POST: retryPost },
  { setDbForTests, resetDbForTests },
  { workosClient },
  { users, billingOrders, billingRetryOperations },
  { setStripeForTests, getBillingCheckoutStatus, refreshBillingCheckoutStatus },
] = await Promise.all([
  import('./checkout-status'),
  import('./checkout'),
  import('./checkout-retry'),
  import('../../../../db/client'),
  import('../../../../lib/workos'),
  import('../../../../db/schema'),
  import('../../../lib/billing'),
]);
const workos = workosClient();

const user = {
  id: '11111111-1111-1111-1111-111111111111',
  workosUserId: 'workos_billing_status',
};

function context(request: Request) {
  return { request, url: new URL(request.url) } as any;
}

function installDb(fixtures: Map<unknown, unknown[][]> = new Map()) {
  const database = {
    execute: async () => [],
    transaction: async (callback: (tx: any) => Promise<unknown>) => callback(database),
    select() {
      let table: unknown;
      const rows = () => {
        const queue = fixtures.get(table);
        if (queue?.length) return queue.shift() ?? [];
        return table === users ? [user] : [];
      };
      const query = {
        from(value: unknown) {
          table = value;
          return query;
        },
        where() {
          return query;
        },
        limit() {
          return Promise.resolve(rows());
        },
        then(resolve: (value: unknown[]) => unknown, reject: (error: unknown) => unknown) {
          return Promise.resolve(rows()).then(resolve, reject);
        },
      };
      return query;
    },
  };
  setDbForTests(database as any);
}

function mockSession(t: TestContext) {
  t.mock.method(workos.userManagement as any, 'loadSealedSession', () => ({
    authenticate: async () => ({ authenticated: true, user: { id: user.workosUserId } }),
  }));
}

afterEach(() => {
  resetDbForTests();
  setStripeForTests(null);
  delete process.env.STRIPE_SUBSCRIPTION_TIERS;
  delete process.env.STRIPE_TOP_UP_PACKS;
});

test('status and retry refresh reject cross-origin requests before authentication or database access', async () => {
  const request = (url: string) =>
    new Request(url, {
      method: 'POST',
      headers: { origin: 'https://attacker.invalid', 'content-type': 'application/json', cookie: 'nvm_session=sealed' },
      body: JSON.stringify({ attemptKey: '11111111-1111-4111-8111-111111111111' }),
    });

  assert.equal((await POST(context(request('https://www.nvm.fyi/api/billing/checkout-status')))).status, 403);
  assert.equal((await retryPost(context(request('https://www.nvm.fyi/api/billing/checkout-retry')))).status, 403);
});

test('retry POST keeps ownership scoped and rejects cross-origin mutation', async (t) => {
  installDb();
  mockSession(t);
  const foreignOrigin = await retryPost(
    context(
      new Request('https://www.nvm.fyi/api/billing/checkout-retry', {
        method: 'POST',
        headers: {
          origin: 'https://attacker.invalid',
          cookie: 'nvm_session=sealed',
          'content-type': 'application/json',
        },
        body: JSON.stringify({
          attemptKey: '22222222-2222-4222-8222-222222222222',
          retryKey: '44444444-4444-4444-8444-444444444444',
        }),
      }),
    ),
  );
  assert.equal(foreignOrigin.status, 403);

  installDb();
  const foreignOrder = await retryPost(
    context(
      new Request('https://www.nvm.fyi/api/billing/checkout-retry', {
        method: 'POST',
        headers: { cookie: 'nvm_session=sealed', 'content-type': 'application/json' },
        body: JSON.stringify({
          retryOperationId: '55555555-5555-4555-8555-555555555555',
          retryKey: '44444444-4444-4444-8444-444444444444',
        }),
      }),
    ),
  );
  assert.equal(foreignOrder.status, 200);
  assert.deepEqual(await foreignOrder.json(), { status: 'none' });
});

test('retry POST replays the durable same-key operation after its original response was lost', async (t) => {
  mockSession(t);
  const attemptKey = '22222222-2222-4222-8222-222222222222';
  const retryKey = '44444444-4444-4444-8444-444444444444';
  const prior = { id: 'order_prior', userId: user.id, attemptKey, state: 'failed' };
  const replacement = {
    id: 'order_replacement',
    userId: user.id,
    attemptKey: '33333333-3333-4333-8333-333333333333',
    state: 'open',
    sessionUrl: 'https://checkout.stripe.test/cs_replacement',
  };
  const operation = {
    id: '55555555-5555-4555-8555-555555555555',
    userId: user.id,
    priorOrderId: prior.id,
    replacementOrderId: replacement.id,
    retryKey,
    state: 'checkout_open',
  };
  installDb(
    new Map([
      [billingRetryOperations, [[operation]]],
      [billingOrders, [[prior], [prior], [replacement]]],
    ]),
  );
  const response = await retryPost(
    context(
      new Request('https://www.nvm.fyi/api/billing/checkout-retry', {
        method: 'POST',
        headers: { cookie: 'nvm_session=sealed', 'content-type': 'application/json' },
        body: JSON.stringify({ attemptKey, retryOperationId: operation.id, retryKey }),
      }),
    ),
  );
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), {
    status: 'url',
    url: replacement.sessionUrl,
    retryOperationId: operation.id,
  });
});

test('status lookup for an unknown or foreign attempt is user-scoped and returns none', async (t) => {
  installDb();
  mockSession(t);
  const request = new Request('https://www.nvm.fyi/api/billing/checkout-status?attemptKey=22222222-2222-4222-8222-222222222222', {
    headers: { cookie: 'nvm_session=sealed' },
  });

  const response = await GET(context(request));
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { status: 'none' });
});

test('status page-load and refresh paths never cancel a PaymentIntent', async () => {
  const attemptKey = '22222222-2222-4222-8222-222222222222';
  const order = {
    id: 'order_retry_pending',
    userId: user.id,
    attemptKey,
    state: 'retry_cancel_pending',
    sessionId: 'cs_retry_pending',
    paymentIntentId: 'pi_retry_pending',
    stripeCustomerId: 'cus_retry_pending',
    kind: 'top_up',
    priceId: 'price_pack',
    version: 3,
  };
  installDb(new Map([[billingOrders, [[order], [order]]]]));
  let cancelCalls = 0;
  setStripeForTests({
    checkout: {
      sessions: {
        retrieve: async () => ({
          id: order.sessionId,
          status: 'complete',
          payment_status: 'unpaid',
          customer: order.stripeCustomerId,
          metadata: { order_id: order.id },
          payment_intent: order.paymentIntentId,
        }),
      },
    },
    paymentIntents: {
      retrieve: async () => ({
        id: order.paymentIntentId,
        customer: order.stripeCustomerId,
        metadata: { order_id: order.id },
        status: 'requires_payment_method',
      }),
      cancel: async () => {
        cancelCalls += 1;
        throw new Error('status inspection must never cancel');
      },
    },
  } as any);

  assert.equal((await getBillingCheckoutStatus({ userId: user.id, attemptKey })).status, 'processing');
  assert.equal((await refreshBillingCheckoutStatus({ userId: user.id, attemptKey })).status, 'processing');
  assert.equal(cancelCalls, 0);
  setStripeForTests(null);
});

test('checkout validation failures are definitive not_created responses', async (t) => {
  installDb();
  mockSession(t);
  const invalidBody = await checkoutPost(
    context(
      new Request('https://www.nvm.fyi/api/billing/checkout', {
        method: 'POST',
        headers: { cookie: 'nvm_session=sealed', 'content-type': 'application/json' },
        body: JSON.stringify({ kind: 'invalid' }),
      }),
    ),
  );
  assert.equal(invalidBody.status, 400);
  assert.equal((await invalidBody.json()).status, 'not_created');

  const invalidKey = await checkoutPost(
    context(
      new Request('https://www.nvm.fyi/api/billing/checkout', {
        method: 'POST',
        headers: { cookie: 'nvm_session=sealed', 'content-type': 'application/json', 'idempotency-key': 'not-a-uuid' },
        body: JSON.stringify({ kind: 'subscription' }),
      }),
    ),
  );
  assert.equal(invalidKey.status, 400);
  assert.deepEqual(await invalidKey.json(), {
    status: 'not_created',
    error: { type: 'invalid_idempotency_key', message: 'Idempotency-Key must be a UUID.' },
  });
});

test('checkout POST distinguishes missing, valid, invalid keys and persisted processing recovery', async (t) => {
  mockSession(t);
  delete process.env.STRIPE_SUBSCRIPTION_TIERS;
  delete process.env.STRIPE_TOP_UP_PACKS;
  const postCheckout = (key?: string) =>
    checkoutPost(
      context(
        new Request('https://www.nvm.fyi/api/billing/checkout', {
          method: 'POST',
          headers: {
            cookie: 'nvm_session=sealed',
            'content-type': 'application/json',
            ...(key === undefined ? {} : { 'idempotency-key': key }),
          },
          body: JSON.stringify({ kind: 'top_up' }),
        }),
      ),
    );

  installDb();
  const missingKey = await postCheckout();
  assert.equal(missingKey.status, 503);
  assert.equal((await missingKey.json()).status, 'not_created');

  installDb();
  const validKey = await postCheckout('22222222-2222-4222-8222-222222222222');
  assert.equal(validKey.status, 503);
  assert.equal((await validKey.json()).status, 'not_created');

  installDb();
  const invalidKey = await postCheckout('invalid');
  assert.equal(invalidKey.status, 400);
  assert.equal((await invalidKey.json()).status, 'not_created');

  const attemptKey = '22222222-2222-4222-8222-222222222222';
  const intentHash = createHash('sha256')
    .update(
      JSON.stringify({
        kind: 'top_up',
        priceId: 'price_unconfigured',
        selectedPriceId: null,
        selectedTier: null,
      }),
    )
    .digest('hex');
  const creatingOrder = {
    id: 'order_creating',
    userId: user.id,
    attemptKey,
    intentHash,
    priceId: 'price_unconfigured',
    state: 'creating',
    leaseExpiresAt: new Date(Date.now() + 60_000),
  };
  installDb(new Map([[billingOrders, [[creatingOrder], [creatingOrder]]]]));
  const processing = await postCheckout(attemptKey);
  assert.equal(processing.status, 202);
  assert.deepEqual(await processing.json(), { status: 'processing' });
});

test('Stripe return resolves the replacement order after browser attempt storage is lost', async (t) => {
  const prior = { id: 'order_prior', userId: user.id, attemptKey: '22222222-2222-4222-8222-222222222222' };
  const replacement = {
    id: 'order_replacement',
    userId: user.id,
    attemptKey: '33333333-3333-4333-8333-333333333333',
    sessionId: 'cs_replacement',
    sessionUrl: 'https://checkout.stripe.test/cs_replacement',
    kind: 'top_up',
    priceId: 'price_pack',
    state: 'open',
  };
  const operation = {
    id: 'retry_operation',
    userId: user.id,
    priorOrderId: prior.id,
    replacementOrderId: replacement.id,
    state: 'checkout_open',
  };
  installDb(
    new Map([
      [billingOrders, [[replacement], [replacement]]],
      [billingRetryOperations, [[operation], [operation]]],
    ]),
  );
  mockSession(t);

  const request = new Request('https://www.nvm.fyi/api/billing/checkout-status', {
    method: 'POST',
    headers: { cookie: 'nvm_session=sealed', 'content-type': 'application/json' },
    body: JSON.stringify({ session_id: 'cs_replacement' }),
  });
  const response = await GET(context(request));
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), {
    status: 'open',
    kind: 'top_up',
    priceId: 'price_pack',
    resumeUrl: replacement.sessionUrl,
    retryOperationId: operation.id,
    attemptKey: replacement.attemptKey,
  });
});

test('Stripe return accepts prior attempt plus linked replacement session, but rejects unrelated orders', async (t) => {
  const prior = { id: 'order_prior', userId: user.id, attemptKey: '22222222-2222-4222-8222-222222222222' };
  const replacement = {
    id: 'order_replacement',
    userId: user.id,
    attemptKey: '33333333-3333-4333-8333-333333333333',
    sessionId: 'cs_replacement',
    sessionUrl: 'https://checkout.stripe.test/cs_replacement',
    kind: 'top_up',
    priceId: 'price_pack',
    state: 'open',
  };
  const operation = {
    id: 'retry_operation',
    userId: user.id,
    priorOrderId: prior.id,
    replacementOrderId: replacement.id,
    state: 'checkout_open',
  };
  installDb(
    new Map([
      [billingOrders, [[prior], [replacement], [replacement]]],
      [billingRetryOperations, [[operation], [operation]]],
    ]),
  );
  mockSession(t);
  const response = await POST(
    context(
      new Request('https://www.nvm.fyi/api/billing/checkout-status', {
        method: 'POST',
        headers: { cookie: 'nvm_session=sealed', 'content-type': 'application/json' },
        body: JSON.stringify({ attemptKey: prior.attemptKey, session_id: replacement.sessionId }),
      }),
    ),
  );
  assert.equal(response.status, 200);
  assert.equal((await response.json()).attemptKey, replacement.attemptKey);

  installDb(new Map([[billingOrders, [[prior], [{ id: 'order_other', userId: user.id, sessionId: 'cs_other' }]]]]));
  const conflict = await POST(
    context(
      new Request('https://www.nvm.fyi/api/billing/checkout-status', {
        method: 'POST',
        headers: { cookie: 'nvm_session=sealed', 'content-type': 'application/json' },
        body: JSON.stringify({ attemptKey: prior.attemptKey, session_id: 'cs_other' }),
      }),
    ),
  );
  assert.equal(conflict.status, 409);
});

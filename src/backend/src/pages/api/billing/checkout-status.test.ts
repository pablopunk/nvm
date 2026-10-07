import assert from 'node:assert/strict';
import { afterEach, test, type TestContext } from 'node:test';

process.env.WORKOS_API_KEY = 'sk_test_billing_status';
process.env.WORKOS_CLIENT_ID = 'client_test_billing_status';
process.env.WORKOS_COOKIE_PASSWORD = 'billing-status-cookie-password-with-32-characters';

const [
  { GET, POST },
  { POST: retryPost },
  { setDbForTests, resetDbForTests },
  { workosClient },
  { users, billingOrders, billingRetryOperations },
] = await Promise.all([
  import('./checkout-status'),
  import('./checkout-retry'),
  import('../../../../db/client'),
  import('../../../../lib/workos'),
  import('../../../../db/schema'),
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

afterEach(() => resetDbForTests());

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

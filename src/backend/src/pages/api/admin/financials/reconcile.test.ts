import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../../../../db/client';
import { POST } from './reconcile';

afterEach(() => resetDbForTests());

test('financial reconciliation rejects cross-origin requests before authentication or Stripe access', async () => {
  const response = await POST({
    request: new Request('https://api.nvm.fyi/api/admin/financials/reconcile', {
      method: 'POST',
      headers: {
        origin: 'https://attacker.example',
        'content-type': 'application/json',
      },
      body: '{}',
    }),
  } as any);

  assert.equal(response.status, 403);
});

test('financial reconciliation requires an admin session before validating the payload', async () => {
  const response = await POST({
    request: new Request('https://api.nvm.fyi/api/admin/financials/reconcile', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: '{}',
    }),
  } as any);

  assert.equal(response.status, 403);
});

test('financial reconciliation rejects a range exceeding one year for an authenticated admin', async () => {
  const previousEnvironment = {
    VERCEL_ENV: process.env.VERCEL_ENV,
    WORKOS_API_KEY: process.env.WORKOS_API_KEY,
    WORKOS_CLIENT_ID: process.env.WORKOS_CLIENT_ID,
    WORKOS_COOKIE_PASSWORD: process.env.WORKOS_COOKIE_PASSWORD,
    PREVIEW_SESSION_KEY: process.env.PREVIEW_SESSION_KEY,
  };
  process.env.VERCEL_ENV = 'preview';
  process.env.WORKOS_API_KEY = 'sk_test_preview';
  process.env.WORKOS_CLIENT_ID = 'client_test_preview';
  process.env.WORKOS_COOKIE_PASSWORD = 'production-session-key';
  process.env.PREVIEW_SESSION_KEY = 'preview-session-key';

  const [workos, { setStripeForTests }] = await Promise.all([
    import('../../../../lib/workos'),
    import('../../../../lib/billing'),
  ]);
  const token = await workos.createPreviewSessionToken({
    id: 'admin-workos',
    email: 'admin@example.test',
  });
  let stripeReads = 0;
  setStripeForTests({
    balanceTransactions: {
      list: async () => {
        stripeReads += 1;
        return { data: [], has_more: false };
      },
    },
  } as any);
  const chain = {
    from() {
      return this;
    },
    where() {
      return this;
    },
    limit() {
      return Promise.resolve([
        { id: 'admin-id', workosUserId: 'admin-workos', role: 'admin' },
      ]);
    },
  };
  setDbForTests({ select: () => chain } as any);

  try {
    const response = await POST({
      request: new Request(
        'https://api.nvm.fyi/api/admin/financials/reconcile',
        {
          method: 'POST',
          headers: {
            cookie: `${workos.PREVIEW_SESSION_COOKIE}=${token}`,
            'content-type': 'application/json',
          },
          body: JSON.stringify({ from: '2025-10-01', to: '2026-10-02' }),
        },
      ),
    } as any);

    assert.equal(response.status, 400);
    assert.equal(stripeReads, 0);
  } finally {
    setStripeForTests(null);
    resetDbForTests();
    for (const [key, value] of Object.entries(previousEnvironment)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
});

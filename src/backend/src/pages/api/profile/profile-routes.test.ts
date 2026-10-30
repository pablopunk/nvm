import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { APIContext } from 'astro';

process.env.WORKOS_API_KEY = 'sk_test_preview';
process.env.WORKOS_CLIENT_ID = 'client_test_preview';
process.env.WORKOS_COOKIE_PASSWORD = 'production-session-key';
process.env.PREVIEW_SESSION_KEY = 'preview-session-key';
process.env.VERCEL_ENV = 'preview';

const [{ GET: balances }, { GET: billing }, { createPreviewSessionToken, PREVIEW_SESSION_COOKIE }, { setDbForTests, resetDbForTests }] = await Promise.all([
  import('./balances'), import('./billing'), import('../../../lib/workos'), import('../../../db/client'),
]);

function context(cookie?: string): APIContext {
  return { request: new Request('https://example.test/api/profile', { headers: cookie ? { cookie } : {} }) } as APIContext;
}

test('profile refresh endpoints reject requests without a session', async () => {
  assert.equal((await balances(context())).status, 401);
  assert.equal((await billing(context())).status, 401);
});

test('profile refresh endpoints return only the signed-in user data', async () => {
  const token = await createPreviewSessionToken({ id: 'workos-user', email: 'user@example.test' });
  const cookie = `${PREVIEW_SESSION_COOKIE}=${token}`;
  let selects = 0;
  const user = { id: 'user-id', workosUserId: 'workos-user', stripeCustomerId: 'customer' };
  const db = {
    select() {
      selects++;
      const rows = selects % 2 === 1 ? [user] : [{ free: 7, paid: 5, status: 'active', currentPeriodEnd: new Date('2026-11-01') }];
      return {
        from() { return this; },
        where() { return this; },
        limit() { return Promise.resolve(rows); },
        then(resolve: (rows: unknown[]) => unknown) { return Promise.resolve(rows).then(resolve); },
      };
    },
  };
  setDbForTests(db as never);
  try {
    const balanceResponse = await balances(context(cookie));
    assert.equal(balanceResponse.status, 200);
    assert.equal(balanceResponse.headers.get('cache-control'), 'private, no-store');
    assert.deepEqual(await balanceResponse.json(), { free: 7, paid: 5, total: 12 });
    const billingResponse = await billing(context(cookie));
    assert.equal(billingResponse.status, 200);
    assert.equal(billingResponse.headers.get('cache-control'), 'private, no-store');
    const data = await billingResponse.json();
    assert.equal(data.subscription.status, 'active');
    assert.equal(data.hasCustomer, true);
    assert.equal('stripeCustomerId' in data, false);
  } finally { resetDbForTests(); }
});

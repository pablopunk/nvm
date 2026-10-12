import assert from 'node:assert/strict';
import { test } from 'node:test';
import { isCreditDeltaAllowedUntilPlan006 } from './credits';

test('admin credit adjustments permit bounded signed deltas and reject zero', () => {
  assert.equal(isCreditDeltaAllowedUntilPlan006(-1), true);
  assert.equal(isCreditDeltaAllowedUntilPlan006(-100), true);
  assert.equal(isCreditDeltaAllowedUntilPlan006(0), false);
  assert.equal(isCreditDeltaAllowedUntilPlan006(10), true);
  assert.equal(isCreditDeltaAllowedUntilPlan006(1_000_001), false);
});

test('authenticated admin credits POST rejects a zero adjustment before opening a transaction', async () => {
  process.env.WORKOS_API_KEY = 'sk_test_preview';
  process.env.WORKOS_CLIENT_ID = 'client_test_preview';
  process.env.WORKOS_COOKIE_PASSWORD = 'production-session-key';
  process.env.PREVIEW_SESSION_KEY = 'preview-session-key';
  process.env.VERCEL_ENV = 'preview';
  const [
    { createPreviewSessionToken, PREVIEW_SESSION_COOKIE },
    { POST },
    { setDbForTests, resetDbForTests },
  ] = await Promise.all([
    import('../../../lib/workos'),
    import('./credits'),
    import('../../../db/client'),
  ]);
  const token = await createPreviewSessionToken({
    id: 'admin-workos',
    email: 'admin@example.test',
  });
  const inserted: unknown[] = [];
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
    values(value: unknown) {
      inserted.push(value);
      return this;
    },
    then(resolve: (value: unknown[]) => unknown) {
      return Promise.resolve([]).then(resolve);
    },
  };
  setDbForTests({ select: () => chain, insert: () => chain } as any);
  try {
    const response = await POST({
      request: new Request('https://api.nvm.fyi/api/admin/credits', {
        method: 'POST',
        headers: {
          cookie: `${PREVIEW_SESSION_COOKIE}=${token}`,
          'content-type': 'application/json',
          'idempotency-key': '11111111-1111-4111-8111-111111111111',
        },
        body: JSON.stringify({
          userId: '11111111-1111-4111-8111-111111111111',
          delta: 0,
          kind: 'free',
          reason: 'No-op must not be recorded',
        }),
      }),
    } as any);
    assert.equal(response.status, 400);
    assert.equal(
      ((await response.json()) as any).error.type,
      'invalid_adjustment',
    );
    assert.deepEqual(
      inserted,
      [],
      'zero adjustment must not mutate the ledger',
    );
  } finally {
    resetDbForTests();
    process.env.VERCEL_ENV = 'test';
  }
});

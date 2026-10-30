import assert from 'node:assert/strict';
import { test } from 'node:test';
import { dashboardFetch } from './dashboard-fetch';

test('dashboard fetch retries transient responses', async (context) => {
  let attempts = 0;
  context.mock.method(globalThis, 'fetch', async () => {
    attempts++;
    return attempts < 3 ? new Response('', { status: 503 }) : Response.json({ ok: true });
  });
  assert.deepEqual(await dashboardFetch('/summary'), { ok: true });
  assert.equal(attempts, 3);
});

test('dashboard fetch does not retry forbidden responses', async (context) => {
  let attempts = 0;
  context.mock.method(globalThis, 'fetch', async () => {
    attempts++;
    return new Response('', { status: 403 });
  });
  await assert.rejects(dashboardFetch('/summary'));
  assert.equal(attempts, 1);
});

import assert from 'node:assert/strict';
import { test } from 'node:test';
import { dashboardRead, isTransientDashboardError } from './dashboard-read';

test('dashboard reads retry transient errors and recover', async () => {
  let attempts = 0;
  const result = await dashboardRead(async () => {
    attempts++;
    if (attempts < 3) throw Object.assign(new Error('temporary'), { code: 'ECONNRESET' });
    return 'ok';
  });
  assert.equal(result, 'ok');
  assert.equal(attempts, 3);
});

test('dashboard reads do not retry permanent failures', async () => {
  let attempts = 0;
  await assert.rejects(dashboardRead(async () => {
    attempts++;
    throw Object.assign(new Error('bad request'), { status: 400 });
  }));
  assert.equal(attempts, 1);
  assert.equal(isTransientDashboardError({ status: 403 }), false);
});

test('dashboard reads stop after two retries', async () => {
  let attempts = 0;
  await assert.rejects(dashboardRead(async () => {
    attempts++;
    throw Object.assign(new Error('offline'), { code: 'ECONNRESET' });
  }));
  assert.equal(attempts, 3);
});

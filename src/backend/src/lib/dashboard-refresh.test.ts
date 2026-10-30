import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createDashboardRefreshGate } from './dashboard-refresh';

test('refresh waits for the interval, skips hidden pages, and avoids overlap', async () => {
  let now = 0;
  let visible = false;
  let count = 0;
  let finish: () => void = () => {};
  const gate = createDashboardRefreshGate(() => {
    count++;
    return new Promise<void>((resolve) => { finish = resolve; });
  }, 60, () => now, () => visible);
  now = 100;
  await gate.tick();
  assert.equal(count, 0);
  visible = true;
  const first = gate.tick();
  assert.equal(count, 1);
  now = 200;
  await gate.tick();
  assert.equal(count, 1);
  finish();
  await first;
  const second = gate.tick();
  assert.equal(count, 2);
  finish();
  await second;
  gate.stop();
  now = 300;
  await gate.tick();
  assert.equal(count, 2);
});

test('failed refreshes recover on a later interval', async () => {
  let now = 0;
  let count = 0;
  const gate = createDashboardRefreshGate(async () => {
    count++;
    if (count === 1) throw new Error('offline');
  }, 60, () => now, () => true);
  now = 60;
  await gate.tick();
  now = 119;
  await gate.tick();
  assert.equal(count, 1);
  now = 120;
  await gate.tick();
  assert.equal(count, 2);
});

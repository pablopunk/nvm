import assert from 'node:assert/strict';
import { test } from 'node:test';
import { healthyRouteCandidates } from './route-breakers';

const candidates = [
  { providerId: 'primary', modelId: 'model-a' },
  { providerId: 'fallback', modelId: 'model-b' },
];

test('healthy configured fallback remains when primary route is tripped', async () => {
  const healthy = await healthyRouteCandidates('pro-smart', candidates, async (_slot, provider) => provider === 'primary');
  assert.deepEqual(healthy, [candidates[1]]);
});

test('no healthy configured fallback leaves no route to dispatch', async () => {
  const healthy = await healthyRouteCandidates('free-fast', candidates, async () => true);
  assert.deepEqual(healthy, []);
});

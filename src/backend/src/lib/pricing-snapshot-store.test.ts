import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  memoryPricingSnapshotStore,
  SNAPSHOT_CURRENT_MAX_AGE_MS,
  SNAPSHOT_MAX_AGE_MS,
  snapshotLookup,
} from './pricing-snapshot-store';

const NOW = 1_800_000_000_000;

test('a snapshot is current inside the daily window, stale inside the hard bound, and expired past it', () => {
  assert.deepEqual(snapshotLookup({ value: 'v', validatedAt: NOW }, NOW), {
    status: 'current',
    value: 'v',
  });
  assert.deepEqual(
    snapshotLookup({ value: 'v', validatedAt: NOW - SNAPSHOT_CURRENT_MAX_AGE_MS }, NOW),
    { status: 'stale', value: 'v' },
  );
  assert.deepEqual(
    snapshotLookup({ value: 'v', validatedAt: NOW - SNAPSHOT_MAX_AGE_MS }, NOW),
    { status: 'expired' },
  );
  assert.deepEqual(snapshotLookup(undefined, NOW), { status: 'unverified' });
});

test('the hard bound is seven days and outlasts a missed daily refresh', () => {
  assert.equal(SNAPSHOT_MAX_AGE_MS, 7 * 24 * 60 * 60 * 1000);
  assert.ok(SNAPSHOT_CURRENT_MAX_AGE_MS > 24 * 60 * 60 * 1000);
});

test('the durable store never rolls a snapshot back to an older validation', async () => {
  const store = memoryPricingSnapshotStore();
  await store.save({ key: 'models_dev:opencode', validatedAt: 200, payload: 'newer' });
  await store.save({ key: 'models_dev:opencode', validatedAt: 100, payload: 'older' });

  assert.deepEqual(await store.load('models_dev:opencode'), {
    key: 'models_dev:opencode',
    validatedAt: 200,
    payload: 'newer',
  });
});

test('expiring the memory store backdates every record it holds', async () => {
  const store = memoryPricingSnapshotStore();
  await store.save({ key: 'k', validatedAt: NOW, payload: {} });
  store.expire?.(NOW - 1);

  assert.equal((await store.load('k'))?.validatedAt, NOW - 1);
});

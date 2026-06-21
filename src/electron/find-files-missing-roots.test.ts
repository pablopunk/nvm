import assert from 'node:assert/strict';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { partitionRootsByExistence } from './file-utils';

test('partitionRootsByExistence separates existing from missing roots', async () => {
  const home = os.homedir();
  const exists = path.join(home, 'Desktop');
  const missing = path.join(home, 'nonexistent-root-xyz');
  const { existing, missing: missingRoots } = await partitionRootsByExistence([
    exists,
    missing,
  ]);

  assert.deepEqual(existing, [exists]);
  assert.deepEqual(missingRoots, [missing]);
});

test('partitionRootsByExistence returns empty arrays for empty input', async () => {
  const { existing, missing } = await partitionRootsByExistence([]);
  assert.deepEqual(existing, []);
  assert.deepEqual(missing, []);
});

test('partitionRootsByExistence handles all-existing roots', async () => {
  const home = os.homedir();
  const desktop = path.join(home, 'Desktop');
  const downloads = path.join(home, 'Downloads');
  const { existing, missing } = await partitionRootsByExistence([
    desktop,
    downloads,
  ]);

  assert.ok(existing.includes(desktop));
  assert.ok(existing.includes(downloads));
  assert.deepEqual(missing, []);
});

test('partitionRootsByExistence handles all-missing roots', async () => {
  const home = os.homedir();
  const a = path.join(home, 'does-not-exist-a');
  const b = path.join(home, 'does-not-exist-b');
  const { existing, missing } = await partitionRootsByExistence([a, b]);

  assert.deepEqual(existing, []);
  assert.equal(missing.length, 2);
  assert.ok(missing.includes(a));
  assert.ok(missing.includes(b));
});

test('partitionRootsByExistence separates mixed roots correctly', async () => {
  const home = os.homedir();
  const exists1 = path.join(home, 'Desktop');
  const missing1 = path.join(home, 'no-exist-1');
  const exists2 = path.join(home, 'Downloads');
  const missing2 = path.join(home, 'no-exist-2');
  const { existing, missing } = await partitionRootsByExistence([
    exists1,
    missing1,
    exists2,
    missing2,
  ]);

  assert.ok(existing.includes(exists1));
  assert.ok(existing.includes(exists2));
  assert.ok(missing.includes(missing1));
  assert.ok(missing.includes(missing2));
  assert.equal(existing.length, 2);
  assert.equal(missing.length, 2);
});

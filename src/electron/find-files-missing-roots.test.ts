import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { partitionRootsByExistence } from './file-utils';

test('partitionRootsByExistence separates existing from missing roots', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-test-'));
  try {
    const missing = path.join(dir, 'nonexistent');
    const { existing, missing: missingRoots } = await partitionRootsByExistence(
      [dir, missing],
    );

    assert.deepEqual(existing, [dir]);
    assert.deepEqual(missingRoots, [missing]);
  } finally {
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test('partitionRootsByExistence returns empty arrays for empty input', async () => {
  const { existing, missing } = await partitionRootsByExistence([]);
  assert.deepEqual(existing, []);
  assert.deepEqual(missing, []);
});

test('partitionRootsByExistence handles all-existing roots', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-test-'));
  try {
    const a = path.join(dir, 'a');
    const b = path.join(dir, 'b');
    await fs.mkdir(a);
    await fs.mkdir(b);
    const { existing, missing } = await partitionRootsByExistence([a, b]);

    assert.ok(existing.includes(a));
    assert.ok(existing.includes(b));
    assert.deepEqual(missing, []);
  } finally {
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test('partitionRootsByExistence handles all-missing roots', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-test-'));
  try {
    const a = path.join(dir, 'no-a');
    const b = path.join(dir, 'no-b');
    const { existing, missing } = await partitionRootsByExistence([a, b]);

    assert.deepEqual(existing, []);
    assert.equal(missing.length, 2);
    assert.ok(missing.includes(a));
    assert.ok(missing.includes(b));
  } finally {
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test('partitionRootsByExistence separates mixed roots correctly', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-test-'));
  try {
    const exists1 = path.join(dir, 'real-a');
    const missing1 = path.join(dir, 'ghost-1');
    const exists2 = path.join(dir, 'real-b');
    const missing2 = path.join(dir, 'ghost-2');
    await fs.mkdir(exists1);
    await fs.mkdir(exists2);
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
  } finally {
    await fs.rm(dir, { recursive: true, force: true });
  }
});

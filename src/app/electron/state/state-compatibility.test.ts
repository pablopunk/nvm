import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  advanceStateCompatibility,
  supportsStateDowngrade,
} from './state-compatibility';

test('persists a high-water mark and refuses an older writer', async (t) => {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-state-epoch-'),
  );
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const first = await advanceStateCompatibility(directory, 1);
  assert.equal(first.trusted, false);
  assert.equal(supportsStateDowngrade(first, 1), false);
  await advanceStateCompatibility(directory, 2);
  await assert.rejects(
    advanceStateCompatibility(directory, 1),
    /newer persisted state/,
  );
});

test('only a trusted matching epoch allows a downgrade', () => {
  const marker = { version: 1 as const, highestEpoch: 2, trusted: true };
  assert.equal(supportsStateDowngrade(marker, 2), true);
  assert.equal(supportsStateDowngrade(marker, 1), false);
  assert.equal(supportsStateDowngrade(marker, undefined), false);
  assert.equal(supportsStateDowngrade(null, 2), false);
});

test('corrupt marker fails closed without replacing its contents', async (t) => {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-state-epoch-'),
  );
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const file = path.join(directory, 'state-compatibility.json');
  await fs.writeFile(file, '{broken');
  await assert.rejects(advanceStateCompatibility(directory, 1));
  assert.equal(await fs.readFile(file, 'utf8'), '{broken');
});

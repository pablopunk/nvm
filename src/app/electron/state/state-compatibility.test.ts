import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { acquireSharedStateLease } from '../platform/shared-state-lease';
import {
  advanceStateCompatibility,
  supportsStateDowngrade,
  stateRootsSupportDowngrade,
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

test('shared and private markers remain untrusted without migration proof', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-state-roots-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  const marker = await advanceStateCompatibility(privateRoot, 1, sharedRoot);
  assert.equal(marker.trusted, false);
  assert.equal(
    (await advanceStateCompatibility(privateRoot, 1, sharedRoot)).trusted,
    false,
  );
  await fs.rm(path.join(sharedRoot, 'state-compatibility.json'));
  await assert.rejects(
    advanceStateCompatibility(privateRoot, 1, sharedRoot),
    /marker is missing/,
  );
});

test('existing unmarked data stays untrusted', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-existing-state-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  await fs.writeFile(path.join(root, 'state.json'), '{}');
  assert.equal((await advanceStateCompatibility(root, 1)).trusted, false);
});

test('marker advancement cannot write while another participant holds the shared lease', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-state-lease-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  const lease = await acquireSharedStateLease(sharedRoot);
  await assert.rejects(
    advanceStateCompatibility(privateRoot, 1, sharedRoot),
    /already in use/,
  );
  await assert.rejects(
    fs.access(path.join(privateRoot, 'state-compatibility.json')),
  );
  await lease.release();
  assert.equal(
    (await advanceStateCompatibility(privateRoot, 1, sharedRoot)).trusted,
    false,
  );
});

test('downgrade authorization re-reads both roots and fails closed on missing or changed state', async (t) => {
  const root = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-state-authorization-'),
  );
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  await advanceStateCompatibility(privateRoot, 1, sharedRoot);
  for (const directory of [privateRoot, sharedRoot])
    await fs.writeFile(
      path.join(directory, 'state-compatibility.json'),
      JSON.stringify({ version: 1, highestEpoch: 1, trusted: true }),
    );
  assert.equal(
    await stateRootsSupportDowngrade([privateRoot, sharedRoot], 1),
    true,
  );
  assert.equal(
    await stateRootsSupportDowngrade([privateRoot, sharedRoot], undefined),
    false,
  );
  await advanceStateCompatibility(privateRoot, 2, sharedRoot);
  assert.equal(
    await stateRootsSupportDowngrade([privateRoot, sharedRoot], 1),
    false,
  );
  await fs.rm(path.join(sharedRoot, 'state-compatibility.json'));
  assert.equal(
    await stateRootsSupportDowngrade([privateRoot, sharedRoot], 2),
    false,
  );
});

test('an interrupted two-root update recovers without granting trust', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-state-recovery-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  await fs.mkdir(privateRoot);
  await fs.mkdir(sharedRoot);
  const marker = { version: 1, highestEpoch: 1, trusted: true };
  await fs.writeFile(
    path.join(privateRoot, 'state-compatibility.json'),
    JSON.stringify(marker),
  );
  await fs.writeFile(
    path.join(sharedRoot, 'state-compatibility.pending.json'),
    JSON.stringify({ roots: [privateRoot, sharedRoot], marker }),
  );
  assert.equal(
    await stateRootsSupportDowngrade([privateRoot, sharedRoot], 1),
    false,
  );
  const recovered = await advanceStateCompatibility(privateRoot, 1, sharedRoot);
  assert.equal(recovered.trusted, false);
  assert.equal(
    await stateRootsSupportDowngrade([privateRoot, sharedRoot], 1),
    false,
  );
  await assert.rejects(
    fs.access(path.join(sharedRoot, 'state-compatibility.pending.json')),
  );
});

test('pending transaction cannot overwrite a newer marker', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-state-recovery-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  await fs.mkdir(privateRoot);
  await fs.mkdir(sharedRoot);
  const newer = { version: 1, highestEpoch: 2, trusted: true };
  await fs.writeFile(
    path.join(privateRoot, 'state-compatibility.json'),
    JSON.stringify(newer),
  );
  await fs.writeFile(
    path.join(sharedRoot, 'state-compatibility.pending.json'),
    JSON.stringify({
      roots: [privateRoot, sharedRoot],
      marker: { ...newer, highestEpoch: 1 },
    }),
  );
  await assert.rejects(
    advanceStateCompatibility(privateRoot, 2, sharedRoot),
    /conflicts/,
  );
  assert.deepEqual(
    JSON.parse(
      await fs.readFile(
        path.join(privateRoot, 'state-compatibility.json'),
        'utf8',
      ),
    ),
    newer,
  );
});

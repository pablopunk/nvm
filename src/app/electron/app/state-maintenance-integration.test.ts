import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createExtensionJsonStore } from '../extension-host/extension-json-store';
import { JobRegistry } from '../platform/jobs';
import { acquireSharedStateLease } from '../platform/shared-state-lease';
import { LocalLearningStore } from '../state/learning-store';
import {
  backupAndTrustState,
  stateRootsSupportDowngrade,
} from '../state/state-compatibility';
import { isStateSafeForInstall } from './state-update-safety';

test('drained timed-out job and extension storage are included in a verified backup', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-maintenance-'));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  const backupParent = path.join(root, 'backup');
  await Promise.all([
    fs.mkdir(privateRoot),
    fs.mkdir(sharedRoot),
    fs.mkdir(backupParent),
  ]);
  const lease = await acquireSharedStateLease(sharedRoot);
  t.after(async () => {
    await lease.release();
    await fs.rm(root, { recursive: true, force: true });
  });
  const storage = createExtensionJsonStore();
  const jobs = new JobRegistry();
  const learning = new LocalLearningStore({
    tracesPath: path.join(privateRoot, 'traces.json'),
    learningsPath: path.join(privateRoot, 'learnings.md'),
  });
  await learning.load();
  learning.appendMessage('conversation', 'user', 'Remember this');
  let proceed!: () => void;
  const blocked = new Promise<void>((resolve) => {
    proceed = resolve;
  });
  jobs.register({
    id: 'writer',
    title: 'Writer',
    timeoutMs: 10,
    run: async () => {
      await blocked;
      await storage.replace(path.join(privateRoot, 'extension.json'), {
        saved: true,
      });
    },
  });
  const run = jobs.run('writer');
  const keepAlive = setTimeout(() => {}, 1000);
  await assert.rejects(run, /timed out/);
  clearTimeout(keepAlive);
  const draining = jobs.quiesce();
  proceed();
  await draining;
  await storage.quiesce();
  await learning.flushPendingSave();
  const { backupPath: backup } = await backupAndTrustState({
    privateRoot,
    sharedRoot,
    backupParent,
    epoch: 1,
    lease,
    legacyInstancesClosed: true,
    // This isolated fixture has no other writers; production must independently establish that condition.
    verifyNoUncoordinatedWriters: async () => true,
  });
  assert.deepEqual(
    JSON.parse(
      await fs.readFile(path.join(backup, 'private', 'extension.json'), 'utf8'),
    ),
    { saved: true },
  );
  const traces = JSON.parse(
    await fs.readFile(path.join(backup, 'private', 'traces.json'), 'utf8'),
  );
  assert.equal(traces.traces[0].messages[0].content, 'Remember this');
  await assert.rejects(
    storage.replace(path.join(privateRoot, 'extension.json'), {}),
    /paused/,
  );
  storage.resume();
  jobs.resume();
  jobs.clear();
  await lease.release();
  assert.equal(
    await stateRootsSupportDowngrade([privateRoot, sharedRoot], 1),
    true,
  );
  assert.equal(
    await stateRootsSupportDowngrade([privateRoot, sharedRoot], 2),
    false,
  );
  assert.equal(
    await isStateSafeForInstall(
      '2026.1030.2',
      { version: '2026.1029.1', nevermindStateEpoch: 1 },
      [privateRoot, sharedRoot],
    ),
    true,
  );
  assert.equal(
    await isStateSafeForInstall(
      '2026.1030.2',
      { version: '2026.1029.1', nevermindStateEpoch: 2 },
      [privateRoot, sharedRoot],
    ),
    false,
  );
});

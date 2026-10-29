import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createExtensionJsonStore } from '../extension-host/extension-json-store';
import { JobRegistry } from '../platform/jobs';
import { acquireSharedStateLease } from '../platform/shared-state-lease';
import { createVerifiedStateBackup } from './state-migration-backup';

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
  const backup = await createVerifiedStateBackup({
    roots: [
      { name: 'private', path: privateRoot },
      { name: 'shared', path: sharedRoot },
    ],
    backupParent,
    lease,
    // This isolated fixture has no other writers; production must independently establish that condition.
    verifyNoUncoordinatedWriters: async () => true,
  });
  assert.deepEqual(
    JSON.parse(
      await fs.readFile(path.join(backup, 'private', 'extension.json'), 'utf8'),
    ),
    { saved: true },
  );
  await assert.rejects(
    storage.replace(path.join(privateRoot, 'extension.json'), {}),
    /paused/,
  );
  storage.resume();
  jobs.resume();
  jobs.clear();
});

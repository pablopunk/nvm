import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test, { type TestContext } from 'node:test';
import { acquireSharedStateLease } from '../platform/shared-state-lease';
import { createVerifiedStateBackup } from './state-migration-backup';

async function fixture(t: TestContext) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-state-backup-'));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  const backupParent = path.join(root, 'backups');
  await Promise.all([
    fs.mkdir(privateRoot),
    fs.mkdir(sharedRoot),
    fs.mkdir(backupParent),
  ]);
  await fs.writeFile(path.join(privateRoot, 'state.json'), '{"aliases":{}}');
  await fs.writeFile(path.join(sharedRoot, 'config.json'), '{"version":1}');
  const lease = await acquireSharedStateLease(sharedRoot);
  t.after(async () => {
    await lease.release();
    await fs.rm(root, { recursive: true, force: true });
  });
  return {
    root,
    privateRoot,
    sharedRoot,
    backupParent,
    lease,
    roots: [
      { name: 'private' as const, path: privateRoot },
      { name: 'shared' as const, path: sharedRoot },
    ],
  };
}

test('verified backup contains both roots and a digest manifest', async (t) => {
  const f = await fixture(t);
  const destination = await createVerifiedStateBackup({
    ...f,
    verifyNoUncoordinatedWriters: async () => true,
  });
  assert.equal(
    await fs.readFile(path.join(destination, 'private', 'state.json'), 'utf8'),
    '{"aliases":{}}',
  );
  assert.equal(
    await fs.readFile(path.join(destination, 'shared', 'config.json'), 'utf8'),
    '{"version":1}',
  );
  const manifest = JSON.parse(
    await fs.readFile(path.join(destination, 'manifest.json'), 'utf8'),
  );
  assert.match(manifest.roots.private[0].sha256, /^[a-f0-9]{64}$/);
});

test('unknown writers and a changing source cannot create a completed backup', async (t) => {
  const f = await fixture(t);
  await assert.rejects(
    createVerifiedStateBackup({
      ...f,
      verifyNoUncoordinatedWriters: async () => false,
    }),
    /Cannot rule out/,
  );
  let probes = 0;
  await assert.rejects(
    createVerifiedStateBackup({
      ...f,
      verifyNoUncoordinatedWriters: async () => {
        probes += 1;
        if (probes === 2)
          await fs.writeFile(
            path.join(f.privateRoot, 'state.json'),
            '{"changed":true}',
          );
        return true;
      },
    }),
    /State changed/,
  );
  assert.deepEqual(await fs.readdir(f.backupParent), []);
});

test('nested symbolic links fail closed instead of silently omitting their targets', async (t) => {
  if (process.platform === 'win32') {
    t.skip('Creating symlinks requires platform privileges');
    return;
  }
  const f = await fixture(t);
  await fs.symlink(
    path.join(f.privateRoot, 'state.json'),
    path.join(f.sharedRoot, 'link'),
  );
  await assert.rejects(
    createVerifiedStateBackup({
      ...f,
      verifyNoUncoordinatedWriters: async () => true,
    }),
    /symbolic link/,
  );
  assert.deepEqual(await fs.readdir(f.backupParent), []);
});

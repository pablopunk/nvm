import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { acquireSharedStateLease } from './shared-state-lease';

function attemptMkdirInChild(lockDirectory: string): Promise<number | null> {
  return new Promise((resolve, reject) => {
    const child = spawn(
      process.execPath,
      [
        '-e',
        'require("node:fs").mkdir(process.argv[1], e => process.exit(e?.code === "EEXIST" ? 0 : 1))',
        lockDirectory,
      ],
      { stdio: 'ignore' },
    );
    child.once('error', reject);
    child.once('exit', resolve);
  });
}

test('separate process cannot acquire a shared lease until it is released', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-shared-lease-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const shared = path.join(root, 'shared');
  const lease = await acquireSharedStateLease(shared);
  assert.equal(await attemptMkdirInChild(`${shared}.writer-lock`), 0);
  await assert.rejects(acquireSharedStateLease(shared), /already in use/);
  await lease.assertHeld();
  await lease.release();
  const next = await acquireSharedStateLease(shared);
  await next.release();
});

test('a missing or changed owner fails closed rather than removing another lease', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-shared-lease-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const shared = path.join(root, 'shared');
  const lease = await acquireSharedStateLease(shared);
  await fs.writeFile(
    path.join(`${shared}.writer-lock`, 'owner.json'),
    JSON.stringify({ token: 'other' }),
  );
  await assert.rejects(lease.release(), /ownership changed/);
  assert.equal((await fs.stat(`${shared}.writer-lock`)).isDirectory(), true);
});

test('aliases to the same state root share a lease', async (t) => {
  if (process.platform === 'win32') {
    t.skip('Creating directory symlinks requires platform privileges');
    return;
  }
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-shared-lease-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const shared = path.join(root, 'shared');
  const alias = path.join(root, 'alias');
  await fs.mkdir(shared);
  await fs.symlink(shared, alias, 'dir');
  const lease = await acquireSharedStateLease(shared);
  await assert.rejects(acquireSharedStateLease(alias), /already in use/);
  await lease.release();
});

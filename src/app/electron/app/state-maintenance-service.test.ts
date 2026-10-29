import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { acquireSharedStateLease } from '../platform/shared-state-lease';
import { stateRootsSupportDowngrade } from '../state/state-compatibility';
import {
  createStateMaintenanceService,
  type StateMaintenanceDeps,
} from './state-maintenance-service';

async function fixture(t: test.TestContext) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-maintenance-svc-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const roots = {
    privateRoot: path.join(root, 'private'),
    sharedRoot: path.join(root, 'shared'),
    backupParent: path.join(root, 'backups'),
  };
  await Promise.all([fs.mkdir(roots.privateRoot), fs.mkdir(roots.sharedRoot)]);
  await fs.writeFile(path.join(roots.privateRoot, 'state.json'), '{"a":1}');
  const events: string[] = [];
  const writer = (name: string, quiesce = async () => {}) => ({
    quiesce: async () => {
      events.push(`pause:${name}`);
      await quiesce();
    },
    resume: () => void events.push(`resume:${name}`),
  });
  function create(overrides: Partial<StateMaintenanceDeps> = {}) {
    return createStateMaintenanceService({
      writers: [writer('activity'), writer('jobs')],
      flushPendingState: async () => void events.push('flush'),
      roots: () => roots,
      epoch: 1,
      ownsSingleInstance: () => true,
      isUpdateInstalling: () => false,
      ...overrides,
    });
  }
  return { roots, events, writer, create };
}

test('pauses writers, flushes, backs up, trusts both roots, then resumes in reverse', async (t) => {
  const { roots, events, create } = await fixture(t);
  const service = create();
  const result = await service.verifyAndTrust();
  assert.equal(result.phase, 'complete');
  assert.deepEqual(events, [
    'pause:activity',
    'pause:jobs',
    'flush',
    'resume:jobs',
    'resume:activity',
  ]);
  const backupPath = (result as { backupPath: string }).backupPath;
  assert.equal(
    await fs.readFile(path.join(backupPath, 'private', 'state.json'), 'utf8'),
    '{"a":1}',
  );
  assert.equal(
    await stateRootsSupportDowngrade([roots.privateRoot, roots.sharedRoot], 1),
    true,
  );
});

test('refuses to trust state when this process does not own the single-instance lock', async (t) => {
  const { roots, events, create } = await fixture(t);
  const service = create({ ownsSingleInstance: () => false });
  const result = await service.verifyAndTrust();
  assert.equal(result.phase, 'failed');
  assert.match((result as { message: string }).message, /other state writers/);
  assert.deepEqual(events.slice(-2), ['resume:jobs', 'resume:activity']);
  assert.equal(
    await stateRootsSupportDowngrade([roots.privateRoot, roots.sharedRoot], 1),
    false,
  );
});

test('refuses to trust state while an update install is in flight', async (t) => {
  const { create } = await fixture(t);
  const result = await create({
    isUpdateInstalling: () => true,
  }).verifyAndTrust();
  assert.equal(result.phase, 'failed');
});

test('resumes every writer that paused when a later writer fails to quiesce', async (t) => {
  const { events, writer, create } = await fixture(t);
  const service = create({
    writers: [
      writer('activity'),
      writer('jobs', async () => {
        throw new Error('drain failed');
      }),
    ],
  });
  const result = await service.verifyAndTrust();
  assert.equal(result.phase, 'failed');
  assert.deepEqual(events, [
    'pause:activity',
    'pause:jobs',
    'resume:jobs',
    'resume:activity',
  ]);
});

test('a flush failure leaves state untrusted and resumes writers', async (t) => {
  const { roots, events, create } = await fixture(t);
  const result = await create({
    flushPendingState: async () => {
      throw new Error('disk full');
    },
  }).verifyAndTrust();
  assert.equal(result.phase, 'failed');
  assert.deepEqual(events.slice(-2), ['resume:jobs', 'resume:activity']);
  assert.equal(
    await stateRootsSupportDowngrade([roots.privateRoot, roots.sharedRoot], 1),
    false,
  );
});

test('concurrent requests share one transaction', async (t) => {
  const { events, create } = await fixture(t);
  const service = create();
  const [first, second] = [service.verifyAndTrust(), service.verifyAndTrust()];
  assert.equal(first, second);
  await first;
  assert.equal(events.filter((event) => event === 'pause:activity').length, 1);
});

test('reports unsupported when no state roots are available', async (t) => {
  const { create } = await fixture(t);
  const service = create({ roots: () => null });
  assert.deepEqual(service.status(), { phase: 'unsupported' });
  assert.deepEqual(await service.verifyAndTrust(), { phase: 'unsupported' });
});

test('refresh reports whether the roots are already trusted', async (t) => {
  const { create } = await fixture(t);
  const service = create();
  await service.refresh();
  assert.deepEqual(service.status(), { phase: 'idle', trusted: false });
});

test('a transaction that has not taken the lease refuses to start once quitting', async (t) => {
  const { create } = await fixture(t);
  let proceed!: () => void;
  const blocked = new Promise<void>((resolve) => {
    proceed = resolve;
  });
  const service = create({
    flushPendingState: async () => {
      await blocked;
    },
  });
  const running = service.verifyAndTrust();
  let quitReady = false;
  const quit = service.prepareForQuit().then(() => {
    quitReady = true;
  });
  proceed();
  const result = await running;
  await quit;
  assert.equal(quitReady, true);
  assert.equal(result.phase, 'failed');
  assert.match((result as { message: string }).message, /quitting/);
  assert.equal((await service.verifyAndTrust()).phase, 'failed');
});

test('notifies listeners of status changes until they unsubscribe', async (t) => {
  const { create } = await fixture(t);
  const service = create();
  const phases: string[] = [];
  const unsubscribe = service.onChange(() =>
    phases.push(service.status().phase),
  );
  await service.verifyAndTrust();
  unsubscribe();
  await service.refresh();
  assert.deepEqual(phases, ['running', 'complete']);
});

test('quitting waits until a transaction holding the lease releases it', async (t) => {
  const { create } = await fixture(t);
  let proceed!: () => void;
  const gate = new Promise<void>((resolve) => {
    proceed = resolve;
  });
  let held!: () => void;
  const lockTaken = new Promise<void>((resolve) => {
    held = resolve;
  });
  const service = create({
    acquireLease: async (sharedRoot) => {
      const lease = await acquireSharedStateLease(sharedRoot);
      return {
        release: lease.release,
        assertHeld: async () => {
          held();
          await gate;
          await lease.assertHeld();
        },
      };
    },
  });
  const running = service.verifyAndTrust();
  await lockTaken;
  let quitReady = false;
  const quit = service.prepareForQuit().then(() => {
    quitReady = true;
  });
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(quitReady, false);
  proceed();
  assert.equal((await running).phase, 'complete');
  await quit;
  assert.equal(quitReady, true);
});

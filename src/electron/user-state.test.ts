import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  corruptStateBackupPath,
  createUserStateSaveScheduler,
  readUserStateFile,
  writeUserStateFile,
} from './user-state';

const JSON_ERROR_PATTERN = /JSON/;
const ATOMIC_REPLACEMENT_FAILURE_PATTERN =
  /injected atomic replacement failure/;
const SAVE_FAILURE_PATTERN = /injected save failure/;

async function withTemporaryDirectory(
  callback: (directory: string) => Promise<void>,
) {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-user-state-'));
  try {
    await callback(directory);
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
}

function fakeTimers() {
  let nextId = 1;
  const timers = new Map<number, () => void>();
  return {
    setTimer: ((callback: () => void) => {
      const id = nextId++;
      timers.set(id, callback);
      return id;
    }) as unknown as typeof setTimeout,
    clearTimer: ((id: number) => {
      timers.delete(id);
    }) as unknown as typeof clearTimeout,
    runNext() {
      const entry = timers.entries().next().value as
        | [number, () => void]
        | undefined;
      if (!entry) {
        throw new Error('Expected a pending timer');
      }
      timers.delete(entry[0]);
      entry[1]();
    },
    size() {
      return timers.size;
    },
  };
}

test('returns null without logging for a missing state file', async () => {
  let readErrors = 0;
  const state = await readUserStateFile('/missing/state.json', {
    readFile: (() =>
      Promise.reject(
        Object.assign(new Error('missing'), { code: 'ENOENT' }),
      )) as typeof fs.readFile,
    onReadError: () => {
      readErrors += 1;
    },
  });

  assert.equal(state, null);
  assert.equal(readErrors, 0);
});

test('loads valid user state', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'state.json');
    await fs.writeFile(filePath, JSON.stringify({ settings: { sound: true } }));

    assert.deepEqual(await readUserStateFile(filePath), {
      settings: { sound: true },
    });
  });
});

test('backs up malformed state and reports corruption', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'state.json');
    const now = new Date('2026-07-19T10:11:12.345Z');
    const expectedBackup = corruptStateBackupPath(filePath, now);
    const corruptions: Array<{ error: unknown; backupPath: string }> = [];
    await fs.writeFile(filePath, '{not json');

    const state = await readUserStateFile(filePath, {
      now: () => now,
      onCorrupt: (error, backupPath) => corruptions.push({ error, backupPath }),
    });

    assert.equal(state, null);
    assert.equal(corruptions.length, 1);
    assert.equal(corruptions[0]?.backupPath, expectedBackup);
    assert.match(String(corruptions[0]?.error), JSON_ERROR_PATTERN);
    await assert.rejects(fs.access(filePath));
    assert.equal(await fs.readFile(expectedBackup, 'utf8'), '{not json');
  });
});

test('reports non-missing read errors without treating them as corruption', async () => {
  const injectedError = Object.assign(new Error('permission denied'), {
    code: 'EACCES',
  });
  const errors: unknown[] = [];
  const state = await readUserStateFile('/denied/state.json', {
    readFile: (() => Promise.reject(injectedError)) as typeof fs.readFile,
    onReadError: (error) => errors.push(error),
  });

  assert.equal(state, null);
  assert.deepEqual(errors, [injectedError]);
});

test('reports corruption even when preserving the malformed file fails', async () => {
  const corruptions: unknown[] = [];
  const backupErrors: unknown[] = [];
  const state = await readUserStateFile('/denied/state.json', {
    readFile: (() =>
      Promise.resolve('{not json')) as unknown as typeof fs.readFile,
    rename: (() =>
      Promise.reject(new Error('injected backup failure'))) as typeof fs.rename,
    onCorrupt: (error) => corruptions.push(error),
    onCorruptBackupError: (error) => backupErrors.push(error),
  });

  assert.equal(state, null);
  assert.equal(corruptions.length, 1);
  assert.equal(backupErrors.length, 1);
});

test('atomic write failure preserves the previous state', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'state.json');
    await fs.writeFile(filePath, JSON.stringify({ previous: true }));

    await assert.rejects(
      writeUserStateFile(
        filePath,
        { next: true },
        {
          replaceFile: () =>
            Promise.reject(new Error('injected atomic replacement failure')),
        },
      ),
      ATOMIC_REPLACEMENT_FAILURE_PATTERN,
    );
    assert.deepEqual(JSON.parse(await fs.readFile(filePath, 'utf8')), {
      previous: true,
    });
  });
});

test('successfully replaces state through the shared atomic writer', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'state.json');
    await fs.writeFile(filePath, JSON.stringify({ previous: true }));

    await writeUserStateFile(filePath, { next: true });

    assert.deepEqual(JSON.parse(await fs.readFile(filePath, 'utf8')), {
      next: true,
    });
    assert.deepEqual(await fs.readdir(directory), ['state.json']);
  });
});

test('immediate flush persists a mutation still inside the debounce window', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'state.json');
    const timers = fakeTimers();
    const state = { setting: 'before' };
    const scheduler = createUserStateSaveScheduler({
      save: () => writeUserStateFile(filePath, state),
      setTimer: timers.setTimer,
      clearTimer: timers.clearTimer,
    });

    state.setting = 'after';
    scheduler.schedule();
    await scheduler.flushPendingSave();

    assert.deepEqual(JSON.parse(await fs.readFile(filePath, 'utf8')), {
      setting: 'after',
    });
    assert.equal(timers.size(), 0);
  });
});

test('coalesces scheduled saves and flushes before the debounce delay', async () => {
  const timers = fakeTimers();
  let saves = 0;
  const scheduler = createUserStateSaveScheduler({
    save: () => {
      saves += 1;
      return Promise.resolve();
    },
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  });

  scheduler.schedule();
  scheduler.schedule();
  assert.equal(timers.size(), 1);
  await scheduler.flushPendingSave();

  assert.equal(saves, 1);
  assert.equal(timers.size(), 0);
  assert.equal(scheduler.hasPendingSave(), false);
});

test('multiple flush callers share an in-flight save without duplicating it', async () => {
  const timers = fakeTimers();
  let releaseSave!: () => void;
  const saveMayFinish = new Promise<void>((resolve) => {
    releaseSave = resolve;
  });
  let saves = 0;
  const scheduler = createUserStateSaveScheduler({
    save: async () => {
      saves += 1;
      await saveMayFinish;
    },
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  });

  scheduler.schedule();
  timers.runNext();
  await new Promise((resolve) => setImmediate(resolve));
  const firstFlush = scheduler.flushPendingSave();
  const secondFlush = scheduler.flushPendingSave();
  assert.equal(firstFlush, secondFlush);
  assert.equal(saves, 1);

  releaseSave();
  await firstFlush;
  assert.equal(saves, 1);
});

test('a change during an in-flight write is saved once more by flush', async () => {
  const timers = fakeTimers();
  let releaseFirstSave!: () => void;
  const firstSaveMayFinish = new Promise<void>((resolve) => {
    releaseFirstSave = resolve;
  });
  let saves = 0;
  const scheduler = createUserStateSaveScheduler({
    save: async () => {
      saves += 1;
      if (saves === 1) {
        await firstSaveMayFinish;
      }
    },
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  });

  scheduler.schedule();
  timers.runNext();
  await new Promise((resolve) => setImmediate(resolve));
  scheduler.schedule();
  const flush = scheduler.flushPendingSave();
  releaseFirstSave();
  await flush;

  assert.equal(saves, 2);
  assert.equal(timers.size(), 0);
});

test('save failures are reported and a later save can recover', async () => {
  const errors: unknown[] = [];
  let saves = 0;
  const scheduler = createUserStateSaveScheduler({
    save: () => {
      saves += 1;
      if (saves === 1) {
        return Promise.reject(new Error('injected save failure'));
      }
      return Promise.resolve();
    },
    onSaveError: (error) => errors.push(error),
  });

  scheduler.schedule();
  await assert.rejects(scheduler.flushPendingSave(), SAVE_FAILURE_PATTERN);
  scheduler.schedule();
  await scheduler.flushPendingSave();

  assert.equal(saves, 2);
  assert.equal(errors.length, 1);
  assert.match(String(errors[0]), SAVE_FAILURE_PATTERN);
});

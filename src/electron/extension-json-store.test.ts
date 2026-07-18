import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createExtensionJsonStore } from './extension-json-store';

const INJECTED_UPDATE_FAILURE_PATTERN = /injected update failure/;
const PRIVATE_FILE_MODE = 0o600;
const PERMISSION_BITS_MODULUS = 0o1000;

async function withTemporaryDirectory(
  callback: (directory: string) => Promise<void>,
) {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-extension-json-store-'),
  );
  try {
    await callback(directory);
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
}

test('serializes concurrent mutations to the same file', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'storage.json');
    const store = createExtensionJsonStore();

    await Promise.all(
      Array.from({ length: 20 }, (_, index) =>
        store.mutate(filePath, (current) => ({
          ...current,
          [`key-${index}`]: index,
        })),
      ),
    );

    assert.deepEqual(await store.read(filePath), {
      ...Object.fromEntries(
        Array.from({ length: 20 }, (_, index) => [`key-${index}`, index]),
      ),
    });
    assert.equal(store.pendingOperationCount(), 0);
  });
});

test('allows different files to mutate independently', async () => {
  await withTemporaryDirectory(async (directory) => {
    const store = createExtensionJsonStore();
    const firstPath = path.join(directory, 'first.json');
    const secondPath = path.join(directory, 'second.json');
    let releaseFirstMutation!: () => void;
    const firstMutationMayFinish = new Promise<void>((resolve) => {
      releaseFirstMutation = resolve;
    });
    let firstMutationStarted!: () => void;
    const firstMutationDidStart = new Promise<void>((resolve) => {
      firstMutationStarted = resolve;
    });

    const firstMutation = store.mutate(firstPath, async (current) => {
      firstMutationStarted();
      await firstMutationMayFinish;
      return { ...current, first: true };
    });
    await firstMutationDidStart;

    await store.mutate(secondPath, (current) => ({
      ...current,
      second: true,
    }));
    releaseFirstMutation();
    await firstMutation;

    assert.deepEqual(await store.read(firstPath), { first: true });
    assert.deepEqual(await store.read(secondPath), { second: true });
  });
});

test('preserves invocation order while the canonical path is resolving', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'storage.json');
    let releaseFirstRealpath!: () => void;
    const firstRealpathMayFinish = new Promise<void>((resolve) => {
      releaseFirstRealpath = resolve;
    });
    let realpathCalls = 0;
    const store = createExtensionJsonStore({
      realpath: (async (candidate) => {
        realpathCalls += 1;
        if (realpathCalls === 1) {
          await firstRealpathMayFinish;
        }
        return String(candidate);
      }) as typeof fs.realpath,
    });

    const mutation = store.mutate(filePath, (current) => ({
      ...current,
      beforeReplace: true,
    }));
    const replacement = store.replace(filePath, {});
    await new Promise((resolve) => setImmediate(resolve));
    releaseFirstRealpath();
    await Promise.all([mutation, replacement]);

    assert.deepEqual(await store.read(filePath), {});
  });
});

test('serializes canonical path aliases', {
  skip: process.platform === 'win32',
}, async () => {
  await withTemporaryDirectory(async (directory) => {
    const realDirectory = path.join(directory, 'real');
    const aliasDirectory = path.join(directory, 'alias');
    await fs.mkdir(realDirectory);
    await fs.symlink(realDirectory, aliasDirectory, 'dir');
    const realPath = path.join(realDirectory, 'storage.json');
    const aliasPath = path.join(aliasDirectory, 'storage.json');
    const store = createExtensionJsonStore();
    let releaseFirstMutation!: () => void;
    const firstMutationMayFinish = new Promise<void>((resolve) => {
      releaseFirstMutation = resolve;
    });
    let firstMutationStarted!: () => void;
    const firstMutationDidStart = new Promise<void>((resolve) => {
      firstMutationStarted = resolve;
    });
    let aliasMutationStarted = false;

    const firstMutation = store.mutate(realPath, async (current) => {
      firstMutationStarted();
      await firstMutationMayFinish;
      return { ...current, first: true };
    });
    await firstMutationDidStart;
    const aliasMutation = store.mutate(aliasPath, (current) => {
      aliasMutationStarted = true;
      return { ...current, alias: true };
    });
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(aliasMutationStarted, false);

    releaseFirstMutation();
    await Promise.all([firstMutation, aliasMutation]);
    assert.deepEqual(await store.read(realPath), {
      first: true,
      alias: true,
    });
  });
});

test('returns an empty object for missing files and surfaces malformed JSON', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'storage.json');
    const store = createExtensionJsonStore();

    assert.deepEqual(await store.read(filePath), {});
    await fs.writeFile(filePath, '{not json');
    await assert.rejects(store.read(filePath), SyntaxError);
    assert.equal(store.pendingOperationCount(), 0);
  });
});

test('keeps the previous file and recovers the queue after a failed temporary write', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'storage.json');
    await fs.writeFile(filePath, JSON.stringify({ previous: true }));
    let failWrite = true;
    const store = createExtensionJsonStore({
      writeTemporaryFile: async (file, data) => {
        if (failWrite) {
          failWrite = false;
          throw new Error('injected write failure');
        }
        await file.writeFile(data);
      },
    });

    await assert.rejects(store.replace(filePath, { replacement: true }));
    assert.deepEqual(JSON.parse(await fs.readFile(filePath, 'utf8')), {
      previous: true,
    });
    assert.deepEqual(await fs.readdir(directory), ['storage.json']);

    await store.replace(filePath, { replacement: true });
    assert.deepEqual(await store.read(filePath), { replacement: true });
    assert.equal(store.pendingOperationCount(), 0);
  });
});

test('recovers and releases the queue after a rejected mutation callback', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'storage.json');
    const store = createExtensionJsonStore();

    await assert.rejects(
      store.mutate(filePath, () => {
        throw new Error('injected update failure');
      }),
      INJECTED_UPDATE_FAILURE_PATTERN,
    );
    assert.equal(store.pendingOperationCount(), 0);

    await store.mutate(filePath, (current) => ({
      ...current,
      recovered: true,
    }));
    assert.deepEqual(await store.read(filePath), { recovered: true });
    assert.equal(store.pendingOperationCount(), 0);
  });
});

test('keeps the previous file and removes temporary files after a failed rename', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'storage.json');
    await fs.writeFile(filePath, JSON.stringify({ previous: true }));
    let failRename = true;
    const store = createExtensionJsonStore({
      rename: async (from, to) => {
        if (failRename) {
          failRename = false;
          throw new Error('injected rename failure');
        }
        await fs.rename(from, to);
      },
    });

    await assert.rejects(store.replace(filePath, { replacement: true }));
    assert.deepEqual(JSON.parse(await fs.readFile(filePath, 'utf8')), {
      previous: true,
    });
    assert.deepEqual(await fs.readdir(directory), ['storage.json']);

    await store.replace(filePath, { replacement: true });
    assert.deepEqual(await store.read(filePath), { replacement: true });
    assert.equal(store.pendingOperationCount(), 0);
  });
});

test('uses unique temporary files with restrictive modes', async () => {
  await withTemporaryDirectory(async (directory) => {
    const filePath = path.join(directory, 'storage.json');
    const temporaryPaths: string[] = [];
    const modes: unknown[] = [];
    const store = createExtensionJsonStore({
      open: ((temporaryPath, flags, mode) => {
        temporaryPaths.push(String(temporaryPath));
        modes.push(mode);
        return fs.open(temporaryPath, flags, mode);
      }) as typeof fs.open,
    });

    await store.replace(filePath, { first: true });
    await store.replace(filePath, { second: true });

    assert.equal(temporaryPaths.length, 2);
    assert.notEqual(temporaryPaths[0], temporaryPaths[1]);
    assert.deepEqual(modes, [PRIVATE_FILE_MODE, PRIVATE_FILE_MODE]);
    if (process.platform !== 'win32') {
      const stat = await fs.stat(filePath);
      assert.equal(stat.mode % PERMISSION_BITS_MODULUS, PRIVATE_FILE_MODE);
    }
  });
});

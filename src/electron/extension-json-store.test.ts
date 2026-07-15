import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createExtensionJsonStore } from './extension-json-store';

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
  });
});

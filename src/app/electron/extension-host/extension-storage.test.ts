import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createExtensionJsonStore } from './extension-json-store';
import { createExtensionStorage } from './extension-storage';

const MEMO_TTL_MS = 60_000;
const LOADER_FAILED_PATTERN = /loader failed/;

async function withExtensionStorage(
  callback: (context: {
    cachePath: string;
    storage: ReturnType<typeof createExtensionStorage>;
    storagePath: string;
    store: ReturnType<typeof createExtensionJsonStore>;
  }) => Promise<void>,
) {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-extension-storage-'),
  );
  const storagePath = path.join(directory, 'storage.json');
  const cachePath = path.join(directory, 'cache.json');
  const store = createExtensionJsonStore();
  const storage = createExtensionStorage({ storagePath, cachePath, store });
  try {
    await callback({ cachePath, storage, storagePath, store });
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
}

function storedCacheValue(
  cache: Record<string, unknown>,
  key: string,
): { value: unknown; updatedAt?: unknown } {
  const value = cache[key];
  if (!(value && typeof value === 'object' && 'value' in value)) {
    throw new TypeError(`Missing cache value for ${key}`);
  }
  return value as { value: unknown; updatedAt?: unknown };
}

test('concurrent set and delete retain unrelated storage values', async () => {
  await withExtensionStorage(async ({ storage, storagePath, store }) => {
    await store.replace(storagePath, { keep: true, remove: true });

    await Promise.all([storage.set('added', 1), storage.delete('remove')]);

    assert.deepEqual(await store.read(storagePath), { keep: true, added: 1 });
  });
});

test('memo loaders run outside the queue and distinct memo keys survive a storage set', async () => {
  await withExtensionStorage(
    async ({ cachePath, storage, storagePath, store }) => {
      let releaseFirstLoader!: () => void;
      const firstLoaderMayFinish = new Promise<void>((resolve) => {
        releaseFirstLoader = resolve;
      });
      let firstLoaderStarted!: () => void;
      const firstLoaderDidStart = new Promise<void>((resolve) => {
        firstLoaderStarted = resolve;
      });

      const firstMemo = storage.memo('first', MEMO_TTL_MS, async () => {
        firstLoaderStarted();
        await firstLoaderMayFinish;
        return 'first-value';
      });
      await firstLoaderDidStart;

      assert.equal(
        await storage.memo('second', MEMO_TTL_MS, () => 'second-value'),
        'second-value',
      );
      await storage.set('setting', 'preserved');
      releaseFirstLoader();
      assert.equal(await firstMemo, 'first-value');

      assert.deepEqual(await store.read(storagePath), {
        setting: 'preserved',
      });
      const cache = await store.read(cachePath);
      assert.equal(storedCacheValue(cache, 'first').value, 'first-value');
      assert.equal(storedCacheValue(cache, 'second').value, 'second-value');
    },
  );
});

test('clear is ordered between earlier and later storage mutations', async () => {
  await withExtensionStorage(async ({ storage, storagePath, store }) => {
    await Promise.all([
      storage.set('before-clear', true),
      storage.clear(),
      storage.set('after-clear', true),
    ]);

    assert.deepEqual(await store.read(storagePath), { 'after-clear': true });
  });
});

test('memo loader rejection does not write or poison later cache mutations', async () => {
  await withExtensionStorage(async ({ cachePath, storage, store }) => {
    await assert.rejects(
      storage.memo('failed', MEMO_TTL_MS, () => {
        throw new Error('loader failed');
      }),
      LOADER_FAILED_PATTERN,
    );

    assert.equal(
      await storage.memo('recovered', MEMO_TTL_MS, () => 'value'),
      'value',
    );
    const cache = await store.read(cachePath);
    assert.equal(cache.failed, undefined);
    const recovered = storedCacheValue(cache, 'recovered');
    assert.equal(recovered.value, 'value');
    assert.equal(typeof recovered.updatedAt, 'number');
  });
});

test('memoStale deduplicates concurrent refreshes for the same cache key', async () => {
  await withExtensionStorage(async ({ cachePath, storage, store }) => {
    let loaderCalls = 0;
    let releaseLoader!: () => void;
    const loaderMayFinish = new Promise<void>((resolve) => {
      releaseLoader = resolve;
    });
    let loaderStarted!: () => void;
    const loaderDidStart = new Promise<void>((resolve) => {
      loaderStarted = resolve;
    });
    async function loader() {
      loaderCalls += 1;
      loaderStarted();
      await loaderMayFinish;
      return 'fresh';
    }

    const first = storage.memoStale('shared', 0, 0, loader);
    await loaderDidStart;
    const second = storage.memoStale('shared', 0, 0, loader);
    releaseLoader();

    assert.deepEqual(await Promise.all([first, second]), ['fresh', 'fresh']);
    assert.equal(loaderCalls, 1);
    assert.equal(
      storedCacheValue(await store.read(cachePath), 'shared').value,
      'fresh',
    );
  });
});

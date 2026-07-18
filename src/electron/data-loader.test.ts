import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createDataLoaderHandle,
  createStaleWhileRevalidateHandle,
  createViewLoaderRegistry,
  isLoaderHandle,
  isStaleWhileRevalidateHandle,
  normalizeLoaderItems,
  resolveLoaderEmptyView,
} from './data-loader';

const CUSTOM_TTL_MS = 30_000;
const CUSTOM_STALE_TTL_MS = 120_000;
const DEFAULT_TTL_MS = 60_000;
const DEFAULT_STALE_TTL_MS = 300_000;
const FRESH_CACHE_AGE_MS = 10_000;
const STALE_CACHE_AGE_MS = 90_000;

type TestCache = Record<string, unknown>;

function testCachedValue(cache: TestCache, key: string) {
  const value = cache[key];
  if (!(value && typeof value === 'object' && 'value' in value)) {
    throw new TypeError(`Missing cache value for ${key}`);
  }
  return value;
}

test('ctx.data.loader returns an opaque handle and normalizeLoaderItems strips it', () => {
  const loader = createDataLoaderHandle(async () => [{ id: 'a' }], {
    retry: true,
  });

  assert.equal(loader._loader, true);
  assert.equal(isLoaderHandle(loader), true);
  assert.equal(typeof loader._fn, 'function');
  assert.equal(loader._retry, true);
  assert.deepEqual(normalizeLoaderItems(loader), []);

  // isLoaderHandle rejects objects missing _fn
  assert.equal(isLoaderHandle({ _loader: true }), false);
  assert.equal(isLoaderHandle({ _loader: true, _fn: 'not-a-function' }), false);
  assert.equal(isLoaderHandle(null), false);
  assert.equal(isLoaderHandle('string'), false);
});

test('loader registry hydrates normalized items', async () => {
  const payloads: Array<{ viewId: string; payload: Record<string, unknown> }> =
    [];
  const registry = createViewLoaderRegistry({
    sendHydrate: (viewId, payload) => payloads.push({ viewId, payload }),
    normalizeItems: (items) =>
      items.map((item) => ({ ...item, normalized: true })),
  });

  registry.register(
    'view:1',
    createDataLoaderHandle(async () => [{ id: 'a', title: 'A' }]),
    { extension: { id: 'test' } },
  );
  assert.equal(registry.has('view:1'), true);

  await registry.spawn('view:1');

  assert.equal(registry.has('view:1'), false);
  assert.deepEqual(payloads, [
    {
      viewId: 'view:1',
      payload: {
        items: [{ id: 'a', title: 'A', normalized: true }],
        isLoading: false,
      },
    },
  ]);
});

test('views with loaders get a default empty state when one is not provided', () => {
  const loader = createDataLoaderHandle(async () => []);

  assert.deepEqual(resolveLoaderEmptyView(undefined, loader), {
    title: 'No items',
    subtitle: '',
  });
  assert.deepEqual(
    resolveLoaderEmptyView({ title: 'Custom', subtitle: 'Empty' }, loader),
    { title: 'Custom', subtitle: 'Empty' },
  );
  assert.equal(resolveLoaderEmptyView(undefined), undefined);
});

test('loader errors preserve entry for retry and retry re-runs after re-registration', async () => {
  const payloads: Record<string, unknown>[] = [];
  const warnings: Array<{ viewId: string; message: string }> = [];
  const registry = createViewLoaderRegistry({
    sendHydrate: (_viewId, payload) => payloads.push(payload),
    normalizeItems: (items) => items,
    warn: (viewId, message) => warnings.push({ viewId, message }),
  });

  // Non-retry: entry is cleaned up after error
  registry.register(
    'view:no-retry',
    createDataLoaderHandle(() => {
      throw new Error('boom');
    }),
    null,
  );
  await registry.spawn('view:no-retry');
  assert.equal(registry.has('view:no-retry'), false);

  // Retry-enabled: entry is preserved after error so retry can re-run
  registry.register(
    'view:retry',
    createDataLoaderHandle(
      () => {
        throw new Error('retry me');
      },
      { retry: true },
    ),
    null,
  );
  await registry.spawn('view:retry');
  assert.equal(registry.has('view:retry'), true);

  // Re-register with a passing loader to simulate a successful retry
  registry.register(
    'view:retry',
    createDataLoaderHandle(async () => [{ id: 'retried' }], { retry: true }),
    null,
  );
  await registry.retry('view:retry');
  assert.equal(registry.has('view:retry'), false);

  assert.deepEqual(payloads, [
    { error: { message: 'boom' }, retry: false },
    { error: { message: 'retry me' }, retry: true },
    { items: [{ id: 'retried' }], isLoading: false },
  ]);
  assert.deepEqual(warnings, [
    { viewId: 'view:no-retry', message: 'boom' },
    { viewId: 'view:retry', message: 'retry me' },
  ]);
});

test('stale in-flight completions do not overwrite newer registrations', async () => {
  const payloads: Record<string, unknown>[] = [];
  const registry = createViewLoaderRegistry({
    sendHydrate: (_viewId, payload) => payloads.push(payload),
    normalizeItems: (items) => items,
  });

  // Register and spawn a slow loader
  let slowResolve!: (items: Record<string, unknown>[]) => void;
  const slowLoader = createDataLoaderHandle(
    () =>
      new Promise<Record<string, unknown>[]>((resolve) => {
        slowResolve = resolve;
      }),
  );
  registry.register('view:1', slowLoader, null);
  const spawnPromise = registry.spawn('view:1');

  // While the first spawn is in-flight, re-register with a new handle
  const fastLoader = createDataLoaderHandle(async () => [{ id: 'fast' }]);
  registry.register('view:1', fastLoader, null);

  // Resolve the original (now-stale) loader
  slowResolve([{ id: 'stale' }]);
  await spawnPromise;

  // The stale completion must not have hydrated or deleted the new entry
  assert.equal(registry.has('view:1'), true);
  assert.deepEqual(payloads, []);

  // The new registration should still work
  await registry.spawn('view:1');
  assert.equal(registry.has('view:1'), false);
  assert.deepEqual(payloads, [{ items: [{ id: 'fast' }], isLoading: false }]);
});

test('normalizeLoaderItems returns _initialItems when handle has them', () => {
  const handle = createDataLoaderHandle(async () => []);
  assert.deepEqual(normalizeLoaderItems(handle), []);

  handle._initialItems = [{ id: 'a' }, { id: 'b' }];
  assert.deepEqual(normalizeLoaderItems(handle), [{ id: 'a' }, { id: 'b' }]);

  // Non-handle values pass through
  assert.deepEqual(normalizeLoaderItems([{ id: 'c' }]), [{ id: 'c' }]);
  assert.equal(normalizeLoaderItems(null), null);
});

test('createStaleWhileRevalidateHandle sets correct shape', () => {
  const loader = async () => [{ id: 'a' }];
  const handle = createStaleWhileRevalidateHandle({
    cacheKey: 'my-key',
    ttlMs: CUSTOM_TTL_MS,
    staleTtlMs: CUSTOM_STALE_TTL_MS,
    loader,
    retry: true,
  });

  assert.equal(isLoaderHandle(handle), true);
  assert.equal(isStaleWhileRevalidateHandle(handle), true);
  assert.equal(handle._kind, 'stale-while-revalidate');
  assert.equal(handle._cacheKey, 'my-key');
  assert.equal(handle._ttlMs, CUSTOM_TTL_MS);
  assert.equal(handle._staleTtlMs, CUSTOM_STALE_TTL_MS);
  assert.equal(handle._retry, true);

  // Default TTLs
  const defaultHandle = createStaleWhileRevalidateHandle({
    cacheKey: 'k',
    loader,
  });
  assert.equal(defaultHandle._ttlMs, DEFAULT_TTL_MS);
  assert.equal(defaultHandle._staleTtlMs, DEFAULT_STALE_TTL_MS);
  assert.equal(defaultHandle._retry, false);
});

test('stale-while-revalidate registry hydrates stale cached items before loader', async () => {
  const cache: TestCache = {
    'swr-key': {
      value: [{ id: 'cached-a' }, { id: 'cached-b' }],
      updatedAt: Date.now() - STALE_CACHE_AGE_MS,
    },
  };

  const payloads: Array<{ viewId: string; payload: Record<string, unknown> }> =
    [];
  const registry = createViewLoaderRegistry({
    sendHydrate: (viewId, payload) => payloads.push({ viewId, payload }),
    normalizeItems: (items) => items,
    readCache: async () => cache,
    mutateCache: (_ext, update) => {
      Object.assign(cache, update(cache));
      return Promise.resolve();
    },
  });

  registry.register(
    'view:swr',
    createStaleWhileRevalidateHandle({
      cacheKey: 'swr-key',
      ttlMs: DEFAULT_TTL_MS,
      staleTtlMs: DEFAULT_STALE_TTL_MS,
      loader: async () => [{ id: 'fresh' }],
    }),
    { extension: { id: 'test' } },
  );

  await registry.spawn('view:swr');

  // Phase 1: stale items hydrated with isLoading: true
  // Phase 2: fresh items hydrated with isLoading: false
  assert.equal(payloads.length, 2);
  assert.deepEqual(payloads[0], {
    viewId: 'view:swr',
    payload: {
      items: [{ id: 'cached-a' }, { id: 'cached-b' }],
      isLoading: true,
    },
  });
  assert.deepEqual(payloads[1], {
    viewId: 'view:swr',
    payload: {
      items: [{ id: 'fresh' }],
      isLoading: false,
    },
  });

  // Cache should be updated with fresh items
  assert.deepEqual(testCachedValue(cache, 'swr-key').value, [{ id: 'fresh' }]);
});

test('concurrent stale-while-revalidate hydrations retain every cache key', async () => {
  let cache: TestCache = {};
  const cacheMutations: Promise<void>[] = [];
  let mutationTail = Promise.resolve();
  function mutateCache(
    _extension: unknown,
    update: (current: TestCache) => TestCache,
  ) {
    const mutation = mutationTail.then(() => {
      cache = update(cache);
    });
    mutationTail = mutation;
    cacheMutations.push(mutation);
    return mutation;
  }
  const registry = createViewLoaderRegistry({
    sendHydrate: () => undefined,
    normalizeItems: (items) => items,
    readCache: async () => cache,
    mutateCache,
  });

  registry.register(
    'view:first',
    createStaleWhileRevalidateHandle({
      cacheKey: 'first',
      loader: async () => [{ id: 'first' }],
    }),
    { extension: { id: 'test' } },
  );
  registry.register(
    'view:second',
    createStaleWhileRevalidateHandle({
      cacheKey: 'second',
      loader: async () => [{ id: 'second' }],
    }),
    { extension: { id: 'test' } },
  );

  const unrelatedMutation = mutateCache(null, (current) => ({
    ...current,
    unrelated: { preserved: true },
  }));
  await Promise.all([
    registry.spawn('view:first'),
    registry.spawn('view:second'),
    unrelatedMutation,
  ]);
  await Promise.all(cacheMutations);

  assert.deepEqual(testCachedValue(cache, 'first').value, [{ id: 'first' }]);
  assert.deepEqual(testCachedValue(cache, 'second').value, [{ id: 'second' }]);
  assert.deepEqual(cache.unrelated, { preserved: true });
});

test('fresh hydration does not wait for persistent cache mutation', async () => {
  const payloads: Record<string, unknown>[] = [];
  let releaseMutation!: () => void;
  const mutationMayFinish = new Promise<void>((resolve) => {
    releaseMutation = resolve;
  });
  let cacheMutation: Promise<void> | undefined;
  const registry = createViewLoaderRegistry({
    sendHydrate: (_viewId, payload) => payloads.push(payload),
    normalizeItems: (items) => items,
    readCache: async () => ({}),
    mutateCache: () => {
      cacheMutation = mutationMayFinish;
      return cacheMutation;
    },
  });
  registry.register(
    'view:non-blocking-cache',
    createStaleWhileRevalidateHandle({
      cacheKey: 'non-blocking',
      loader: async () => [{ id: 'fresh' }],
    }),
    { extension: { id: 'test' } },
  );

  await registry.spawn('view:non-blocking-cache');

  assert.deepEqual(payloads, [{ items: [{ id: 'fresh' }], isLoading: false }]);
  assert.ok(cacheMutation);
  let mutationSettled = false;
  cacheMutation.then(() => {
    mutationSettled = true;
  });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(mutationSettled, false);
  releaseMutation();
  await cacheMutation;
});

test('stale-while-revalidate fresh cache skips loader entirely', async () => {
  const cache: TestCache = {
    'swr-fresh': {
      value: [{ id: 'recent' }],
      updatedAt: Date.now() - FRESH_CACHE_AGE_MS,
    },
  };

  const payloads: Array<{ viewId: string; payload: Record<string, unknown> }> =
    [];
  let loaderRan = false;
  const registry = createViewLoaderRegistry({
    sendHydrate: (viewId, payload) => payloads.push({ viewId, payload }),
    normalizeItems: (items) => items,
    readCache: async () => cache,
  });

  registry.register(
    'view:fresh',
    createStaleWhileRevalidateHandle({
      cacheKey: 'swr-fresh',
      ttlMs: DEFAULT_TTL_MS,
      staleTtlMs: DEFAULT_STALE_TTL_MS,
      loader: () => {
        loaderRan = true;
        return Promise.resolve([{ id: 'new' }]);
      },
    }),
    { extension: { id: 'test' } },
  );

  await registry.spawn('view:fresh');

  // Only one hydrate: fresh items with isLoading: false
  assert.equal(payloads.length, 1);
  assert.deepEqual(payloads[0], {
    viewId: 'view:fresh',
    payload: {
      items: [{ id: 'recent' }],
      isLoading: false,
    },
  });
  assert.equal(loaderRan, false);
  assert.equal(registry.has('view:fresh'), false);
});

test('stale-while-revalidate loader failure falls back to stale cache', async () => {
  const cache: TestCache = {
    'swr-fallback': {
      value: [{ id: 'stale' }],
      updatedAt: Date.now() - STALE_CACHE_AGE_MS,
    },
  };

  const payloads: Record<string, unknown>[] = [];
  const warnings: Array<{ viewId: string; message: string }> = [];
  const registry = createViewLoaderRegistry({
    sendHydrate: (_viewId, payload) => payloads.push(payload),
    normalizeItems: (items) => items,
    warn: (viewId, message) => warnings.push({ viewId, message }),
    readCache: async () => cache,
  });

  registry.register(
    'view:fail',
    createStaleWhileRevalidateHandle({
      cacheKey: 'swr-fallback',
      ttlMs: DEFAULT_TTL_MS,
      staleTtlMs: DEFAULT_STALE_TTL_MS,
      loader: () => Promise.reject(new Error('network error')),
    }),
    { extension: { id: 'test' } },
  );

  await registry.spawn('view:fail');

  // Phase 1: stale items with isLoading: true
  // Phase 2 fallback: stale items with isLoading: false (graceful fallback, not error)
  assert.equal(payloads.length, 2);
  assert.deepEqual(payloads[0], {
    items: [{ id: 'stale' }],
    isLoading: true,
  });
  assert.deepEqual(payloads[1], {
    items: [{ id: 'stale' }],
    isLoading: false,
  });
  assert.equal(registry.has('view:fail'), false);

  // No error payload — stale fallback replaced it
  const errorPayloads = payloads.filter((p) => 'error' in p);
  assert.equal(errorPayloads.length, 0);
});

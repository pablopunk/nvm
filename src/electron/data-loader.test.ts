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

test('ctx.data.loader returns an opaque handle and normalizeLoaderItems strips it', async () => {
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

test('views with loaders get a default empty state when one is not provided', async () => {
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
  const payloads: Array<Record<string, unknown>> = [];
  const warnings: Array<{ viewId: string; message: string }> = [];
  const registry = createViewLoaderRegistry({
    sendHydrate: (_viewId, payload) => payloads.push(payload),
    normalizeItems: (items) => items,
    warn: (viewId, message) => warnings.push({ viewId, message }),
  });

  // Non-retry: entry is cleaned up after error
  registry.register(
    'view:no-retry',
    createDataLoaderHandle(async () => {
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
      async () => {
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
  const payloads: Array<Record<string, unknown>> = [];
  const registry = createViewLoaderRegistry({
    sendHydrate: (_viewId, payload) => payloads.push(payload),
    normalizeItems: (items) => items,
  });

  // Register and spawn a slow loader
  let slowResolve!: (items: any[]) => void;
  const slowLoader = createDataLoaderHandle(
    () =>
      new Promise<any[]>((resolve) => {
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
    ttlMs: 30_000,
    staleTtlMs: 120_000,
    loader,
    retry: true,
  });

  assert.equal(isLoaderHandle(handle), true);
  assert.equal(isStaleWhileRevalidateHandle(handle), true);
  assert.equal(handle._kind, 'stale-while-revalidate');
  assert.equal(handle._cacheKey, 'my-key');
  assert.equal(handle._ttlMs, 30_000);
  assert.equal(handle._staleTtlMs, 120_000);
  assert.equal(handle._retry, true);

  // Default TTLs
  const defaultHandle = createStaleWhileRevalidateHandle({
    cacheKey: 'k',
    loader,
  });
  assert.equal(defaultHandle._ttlMs, 60_000);
  assert.equal(defaultHandle._staleTtlMs, 300_000);
  assert.equal(defaultHandle._retry, false);
});

test('stale-while-revalidate registry hydrates stale cached items before loader', async () => {
  const cache: Record<string, any> = {
    'swr-key': {
      value: [{ id: 'cached-a' }, { id: 'cached-b' }],
      updatedAt: Date.now() - 90_000, // stale: > 60s ttl, < 300s stale
    },
  };

  const payloads: Array<{ viewId: string; payload: Record<string, unknown> }> =
    [];
  const registry = createViewLoaderRegistry({
    sendHydrate: (viewId, payload) => payloads.push({ viewId, payload }),
    normalizeItems: (items) => items,
    readCache: async () => cache,
    mutateCache: async (_ext, update) => {
      Object.assign(cache, update(cache));
    },
  });

  registry.register(
    'view:swr',
    createStaleWhileRevalidateHandle({
      cacheKey: 'swr-key',
      ttlMs: 60_000,
      staleTtlMs: 300_000,
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
  assert.ok(cache['swr-key']);
  assert.deepEqual(cache['swr-key'].value, [{ id: 'fresh' }]);
});

test('concurrent stale-while-revalidate hydrations retain every cache key', async () => {
  const cache: Record<string, any> = {};
  const cacheMutations: Promise<void>[] = [];
  let mutationTail = Promise.resolve();
  const registry = createViewLoaderRegistry({
    sendHydrate: () => {},
    normalizeItems: (items) => items,
    readCache: async () => cache,
    mutateCache: (_extension, update) => {
      const mutation = mutationTail.then(() => {
        Object.assign(cache, update(cache));
      });
      mutationTail = mutation;
      cacheMutations.push(mutation);
      return mutation;
    },
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

  await Promise.all([
    registry.spawn('view:first'),
    registry.spawn('view:second'),
  ]);
  await Promise.all(cacheMutations);

  assert.deepEqual(cache.first.value, [{ id: 'first' }]);
  assert.deepEqual(cache.second.value, [{ id: 'second' }]);
});

test('stale-while-revalidate fresh cache skips loader entirely', async () => {
  const cache: Record<string, any> = {
    'swr-fresh': {
      value: [{ id: 'recent' }],
      updatedAt: Date.now() - 10_000, // fresh: < 60s ttl
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
      ttlMs: 60_000,
      staleTtlMs: 300_000,
      loader: async () => {
        loaderRan = true;
        return [{ id: 'new' }];
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
  const cache: Record<string, any> = {
    'swr-fallback': {
      value: [{ id: 'stale' }],
      updatedAt: Date.now() - 90_000,
    },
  };

  const payloads: Array<Record<string, unknown>> = [];
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
      ttlMs: 60_000,
      staleTtlMs: 300_000,
      loader: async () => {
        throw new Error('network error');
      },
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

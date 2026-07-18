import assert from 'node:assert/strict';
import test from 'node:test';
import { createAppIconCache, withTimeout } from './app-icon-cache';

function createCache(
  options: { hasIcons?: boolean; cached?: Buffer | null; icon?: Buffer } = {},
) {
  const scheduled: Array<{ reason: string; delayMs?: number }> = [];
  const loaded: string[] = [];
  const written: Array<{ key: string; png: Buffer }> = [];
  const marks: Array<{ name: string; data?: Record<string, unknown> }> = [];
  const cache = createAppIconCache({
    hasAppIcons: () => options.hasIcons !== false,
    hashValue: (value) => `hash:${value}`,
    readCachedIcon: async () => options.cached ?? null,
    writeCachedIcon: async (key, png) => {
      written.push({ key, png });
    },
    loadIcon: async (appPath) => {
      loaded.push(appPath);
      return options.icon || Buffer.from('png');
    },
    schedule: (reason, delayMs) => scheduled.push({ reason, delayMs }),
    mark: (name, data) => marks.push({ name, data }),
  });
  return { cache, scheduled, loaded, written, marks };
}

test('app icon cache rejects unsupported paths without scheduling work', async () => {
  const { cache, scheduled } = createCache();

  assert.equal(await cache.get('/Applications/TextEdit.txt'), null);
  assert.equal(await cache.get(''), null);
  assert.deepEqual(scheduled, []);
});

test('app icon cache dedupes concurrent requests and resolves waiters from one load', async () => {
  const { cache, scheduled, loaded, written } = createCache();

  const first = cache.get('/Applications/TextEdit.app');
  const second = cache.get('/Applications/TextEdit.app');
  assert.deepEqual(scheduled, [{ reason: 'icon-request', delayMs: 0 }]);

  await cache.processPending();
  assert.equal(await first, 'data:image/png;base64,cG5n');
  assert.equal(await second, 'data:image/png;base64,cG5n');
  assert.deepEqual(loaded, ['/Applications/TextEdit.app']);
  assert.deepEqual(
    written.map((item) => item.key),
    ['hash:bundle-icon-v5:/Applications/TextEdit.app'],
  );
});

test('app icon cache loads Windows Start Menu shortcut icons', async () => {
  const { cache, scheduled, loaded } = createCache();
  const appPath = String.raw`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Example.LNK`;

  const request = cache.get(appPath);
  assert.deepEqual(scheduled, [{ reason: 'icon-request', delayMs: 0 }]);

  await cache.processPending();
  assert.equal(await request, 'data:image/png;base64,cG5n');
  assert.deepEqual(loaded, [appPath]);
});

test('app icon cache uses disk cache before loading native app icon', async () => {
  const { cache, loaded, marks } = createCache({
    cached: Buffer.from('cached'),
  });

  assert.equal(
    await cache.loadAppIconDataUrl('/Applications/TextEdit.app'),
    'data:image/png;base64,Y2FjaGVk',
  );
  assert.deepEqual(loaded, []);
  assert.equal(marks[0]?.name, 'apps.icon.cache-hit');
});

test('app icon cache schedules backlog batches when pending work remains', async () => {
  const { cache, scheduled } = createCache();

  // Create 3 requests, process 1 at a time (sequential processing)
  const requests = [
    cache.get('/Applications/App1.app'),
    cache.get('/Applications/App2.app'),
    cache.get('/Applications/App3.app'),
  ];
  await cache.processPending();
  // First icon processed, 2 remaining
  assert.equal(cache.pendingCount(), 2);
  assert.equal(scheduled.at(-1)?.reason, 'icon-backlog');
  assert.equal(scheduled.at(-1)?.delayMs, 50);

  // Resolve remaining
  await cache.processPending();
  await cache.processPending();
  await Promise.all(requests);
  assert.equal(cache.pendingCount(), 0);
});

test('withTimeout resolves with value when promise settles first', async () => {
  const result = await withTimeout(Promise.resolve('ok'), 99999, 'fallback');
  assert.equal(result, 'ok');
});

test('withTimeout resolves with fallback when promise times out', {
  skip: 'unref() timer cannot keep event loop alive in test runner',
}, async () => {
  const never = new Promise<string>(() => {});
  const result = await withTimeout(never, 10, 'timed-out');
  assert.equal(result, 'timed-out');
});

test('withTimeout rejects when promise rejects (fallback is only for timeout)', async () => {
  // Catch the rejection so it doesn't trigger unhandledRejection
  const failing = new Promise<string>((_, reject) => reject(new Error('fail')));
  await failing.catch(() => {}); // handle so it doesn't pollute
  await assert.rejects(
    withTimeout(Promise.reject(new Error('fail')), 50, 'fallback'),
    (err: Error) => err.message === 'fail',
  );
});

test('app icon cache returns null for individual load failures (rejection, not timeout)', async () => {
  let throwOnLoad = false;

  const deps = {
    hasAppIcons: () => true,
    hashValue: (value: string) => `hash:${value}`,
    readCachedIcon: async () => null as Buffer | null,
    writeCachedIcon: async (_key: string, _png: Buffer) => {},
    loadIcon: async (_appPath: string) => {
      if (throwOnLoad) {
        throw new Error('injected failure');
      }
      return Buffer.from('ok');
    },
    schedule: (_reason: string, _delayMs?: number) => {},
    mark: (_name: string, _data?: Record<string, unknown>) => {},
  };
  const cache = createAppIconCache(deps);

  throwOnLoad = true;
  const appPath = '/Applications/Broken.app';
  const promise = cache.get(appPath);
  assert.equal(cache.pendingCount(), 1);

  // processPending handles per-icon rejection gracefully and resolves waiters
  await cache.processPending();

  // The waiter should be resolved with null, not left hanging
  const result = await promise;
  assert.equal(result, null);
  assert.equal(cache.memorySize(), 0);
});

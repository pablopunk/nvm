import assert from 'node:assert/strict';
import test from 'node:test';
import { createAppIconCache } from './app-icon-cache';

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
    ['hash:/Applications/TextEdit.app'],
  );
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

  const requests = Array.from({ length: 21 }, (_, index) =>
    cache.get(`/Applications/App${index}.app`),
  );
  await cache.processPending();
  await Promise.all(requests.slice(0, 20));

  assert.equal(cache.pendingCount(), 1);
  assert.equal(scheduled.at(-1)?.reason, 'icon-backlog');
  assert.equal(scheduled.at(-1)?.delayMs, 50);
});

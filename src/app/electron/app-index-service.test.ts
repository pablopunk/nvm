import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createAppIndexService,
  dedupeAndSortApps,
  type IndexedApp,
} from './app-index-service';

const normalize = (value: string) => value.trim().toLowerCase();
const INITIAL_DATE_ADDED = 100;
const ENRICHED_DATE_ADDED = 200;

function noop() {
  return;
}

test('dedupeAndSortApps keeps one app per normalized name and sorts by display name', () => {
  const apps = dedupeAndSortApps(
    [
      { name: 'Safari', path: '/A/Safari.app' },
      { name: 'Notes', path: '/A/Notes.app' },
      { name: 'safari', path: '/B/Safari.app' },
    ],
    normalize,
  );

  assert.deepEqual(
    apps.map((item) => `${item.name}:${item.path}`),
    ['Notes:/A/Notes.app', 'safari:/B/Safari.app'],
  );
});

test('app index service indexes apps and notifies dependent running-app status', async () => {
  const notifications: string[] = [];
  const service = createAppIndexService({
    scanApps: async () => [
      { name: 'B' },
      { name: 'A' },
      { name: 'a', path: '/latest/A.app' },
    ],
    watchApps: () => [],
    normalize,
    emitChanged: () => notifications.push('changed'),
    invalidateRunningStatus: () => notifications.push('invalidate-running'),
    scheduleRunningStatusRefresh: (reason) =>
      notifications.push(`refresh:${reason}`),
    notifyIndexed: (count) => notifications.push(`indexed:${count}`),
    mark: (name, data) => notifications.push(`${name}:${data?.indexedCount}`),
  });

  await service.indexApplications();

  assert.deepEqual(
    service.get().map((item) => item.name),
    ['a', 'B'],
  );
  assert.deepEqual(notifications, [
    'invalidate-running',
    'apps.index.result:2',
    'indexed:2',
    'refresh:apps-indexed',
  ]);
});

test('app index service publishes cheap apps before date-added enrichment', async () => {
  let finishEnrichment: ((apps: IndexedApp[]) => void) | undefined;
  let primaryIndexReady: (() => void) | undefined;
  const primaryIndexed = new Promise<void>((resolve) => {
    primaryIndexReady = resolve;
  });
  const service = createAppIndexService({
    scanApps: async () => [
      {
        name: 'New App',
        path: '/Applications/New App.app',
        dateAddedMs: INITIAL_DATE_ADDED,
      },
    ],
    enrichApps: () =>
      new Promise((resolve) => {
        finishEnrichment = resolve;
      }),
    watchApps: () => [],
    normalize,
    emitChanged: noop,
    invalidateRunningStatus: noop,
    scheduleRunningStatusRefresh: noop,
    notifyIndexed: () => primaryIndexReady?.(),
  });

  const indexing = service.indexApplications();
  await primaryIndexed;
  assert.equal(service.get()[0]?.dateAddedMs, INITIAL_DATE_ADDED);

  finishEnrichment?.([
    {
      name: 'New App',
      path: '/Applications/New App.app',
      dateAddedMs: ENRICHED_DATE_ADDED,
    },
  ]);
  await indexing;
  assert.equal(service.get()[0]?.dateAddedMs, ENRICHED_DATE_ADDED);
});

test('app index service replaces watchers and emits debounced host change events', async () => {
  const closed: string[] = [];
  const emitted: string[] = [];
  const callbacks: Array<() => void> = [];
  const service = createAppIndexService({
    scanApps: async () => [],
    watchApps: () => {
      const id = `watcher-${callbacks.length}`;
      callbacks.push(() => emitted.push('changed'));
      return [{ close: () => closed.push(id) }];
    },
    normalize,
    emitChanged: () => emitted.push('changed'),
    invalidateRunningStatus: noop,
    scheduleRunningStatusRefresh: noop,
    notifyIndexed: noop,
  });

  await service.startWatcher();
  await service.startWatcher();
  service.scheduleIndex();
  service.closeWatchers();

  assert.deepEqual(closed, ['watcher-0', 'watcher-1']);
  assert.deepEqual(emitted, ['changed']);
});

test('app index service keeps previous index when scanning fails', async () => {
  const errors: Array<{ message: string; error: unknown }> = [];
  let apps: IndexedApp[] | Error = [{ name: 'Notes' }];
  const service = createAppIndexService({
    scanApps: () =>
      apps instanceof Error ? Promise.reject(apps) : Promise.resolve(apps),
    watchApps: () => [],
    normalize,
    emitChanged: noop,
    invalidateRunningStatus: noop,
    scheduleRunningStatusRefresh: noop,
    notifyIndexed: noop,
    error: (message, error) => errors.push({ message, error }),
  });

  await service.indexApplications();
  apps = new Error('boom');
  await service.indexApplications();

  assert.deepEqual(
    service.get().map((item) => item.name),
    ['Notes'],
  );
  assert.equal(errors[0]?.message, 'applications.index.failed');
});

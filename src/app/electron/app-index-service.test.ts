import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createAppIndexService,
  dedupeAndSortApps,
  type IndexedApp,
  trackFirstSeenApps,
} from './app-index-service';

const normalize = (value: string) => value.trim().toLowerCase();
const FIRST_SCAN_TIME = 100;
const NEW_APP_TIME = 200;
const UPDATE_TIME = 300;

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

test('first-seen tracking keeps a neutral baseline and dates only new app paths', () => {
  const baseline = trackFirstSeenApps(
    [{ name: 'Existing', path: '/Applications/Existing.app' }],
    {
      firstSeenAtById: {},
      initialized: false,
      now: FIRST_SCAN_TIME,
      normalize,
    },
  );
  assert.equal(baseline.apps[0]?.firstSeenAt, 0);
  assert.equal(baseline.initialized, true);

  const nextScan = trackFirstSeenApps(
    [
      { name: 'Existing', path: '/Applications/Existing.app' },
      { name: 'New', path: '/Applications/New.app' },
    ],
    {
      firstSeenAtById: baseline.firstSeenAtById,
      initialized: baseline.initialized,
      now: NEW_APP_TIME,
      normalize,
    },
  );
  assert.deepEqual(
    nextScan.apps.map((app) => app.firstSeenAt),
    [0, NEW_APP_TIME],
  );

  const afterUpdate = trackFirstSeenApps(
    [{ name: 'New', path: '/Applications/New.app' }],
    {
      firstSeenAtById: nextScan.firstSeenAtById,
      initialized: nextScan.initialized,
      now: UPDATE_TIME,
      normalize,
    },
  );
  assert.equal(afterUpdate.apps[0]?.firstSeenAt, NEW_APP_TIME);
  assert.equal(afterUpdate.changed, false);
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

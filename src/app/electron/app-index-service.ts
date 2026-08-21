export interface IndexedApp {
  id?: string;
  name: string;
  path?: string;
  firstSeenAt?: number;
  [key: string]: unknown;
}

export interface AppIndexServiceDeps {
  scanApps: () => Promise<IndexedApp[]>;
  trackFirstSeen?: (apps: IndexedApp[]) => IndexedApp[];
  watchApps: (onChanged: () => void) => Array<{ close: () => unknown }>;
  normalize: (value: string) => string;
  emitChanged: () => void;
  invalidateRunningStatus: () => void;
  scheduleRunningStatusRefresh: (reason: string) => void;
  notifyIndexed: (count: number) => void;
  measure?: <T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => Promise<T>,
  ) => Promise<T>;
  mark?: (name: string, data?: Record<string, unknown>) => void;
  error?: (message: string, error: unknown) => void;
}

export function trackFirstSeenApps(
  apps: IndexedApp[],
  options: {
    firstSeenAtById: Record<string, number>;
    initialized: boolean;
    now: number;
    normalize: (value: string) => string;
  },
) {
  const { firstSeenAtById, initialized, now, normalize } = options;
  const nextFirstSeenAtById = { ...firstSeenAtById };
  let changed = false;
  const trackedApps = apps.map((app) => {
    const id = normalize(String(app.path || app.id || app.name));
    if (!Object.hasOwn(nextFirstSeenAtById, id)) {
      nextFirstSeenAtById[id] = initialized ? now : 0;
      changed = true;
    }
    return { ...app, firstSeenAt: nextFirstSeenAtById[id] };
  });
  const nextInitialized = initialized || apps.length > 0;
  return {
    apps: trackedApps,
    firstSeenAtById: nextFirstSeenAtById,
    initialized: nextInitialized,
    changed: changed || nextInitialized !== initialized,
  };
}

export function dedupeAndSortApps(
  apps: IndexedApp[],
  normalize: (value: string) => string,
) {
  const deduped = new Map<string, IndexedApp>();
  for (const item of apps) {
    deduped.set(normalize(item.name), item);
  }
  return Array.from(deduped.values()).sort((a, b) =>
    a.name.localeCompare(b.name),
  );
}

export function createAppIndexService(deps: AppIndexServiceDeps) {
  let index: IndexedApp[] = [];
  let indexGeneration = 0;
  let watchers: Array<{ close: () => unknown }> = [];

  function measure<T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => Promise<T>,
  ) {
    return deps.measure ? deps.measure(name, data, fn) : fn();
  }

  function get() {
    return index;
  }

  function scheduleIndex() {
    deps.emitChanged();
  }

  function startWatcher() {
    for (const watcher of watchers) {
      watcher.close();
    }
    watchers = deps.watchApps(scheduleIndex);
  }

  async function indexApplications() {
    const generation = ++indexGeneration;
    await measure('apps.index', { alwaysLog: true }, async () => {
      try {
        const scannedApps = await deps.scanApps();
        if (generation !== indexGeneration) {
          return;
        }
        const apps = deps.trackFirstSeen?.(scannedApps) || scannedApps;
        index = dedupeAndSortApps(apps, deps.normalize);
        deps.invalidateRunningStatus();
        deps.mark?.('apps.index.result', {
          scannedCount: apps.length,
          indexedCount: index.length,
        });
        deps.notifyIndexed(index.length);
        deps.scheduleRunningStatusRefresh('apps-indexed');
      } catch (error) {
        deps.error?.('applications.index.failed', error);
      }
    });
  }

  function closeWatchers() {
    for (const watcher of watchers) {
      watcher.close();
    }
    watchers = [];
  }

  return { get, scheduleIndex, startWatcher, indexApplications, closeWatchers };
}

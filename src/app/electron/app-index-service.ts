function measureAppIndex<T>(
  deps: AppIndexServiceDeps,
  name: string,
  data: Record<string, unknown>,
  fn: () => Promise<T>,
) {
  return deps.measure ? deps.measure(name, data, fn) : fn();
}

function indexedAppsFromScan(
  deps: AppIndexServiceDeps,
  scan: AppScanResult,
  initializeBaseline: boolean,
) {
  const apps =
    deps.trackFirstSeen?.(scan.apps, initializeBaseline) || scan.apps;
  return dedupeAndSortApps(apps, deps.normalize);
}

async function confirmedBaselineIndex(options: {
  deps: AppIndexServiceDeps;
  isCurrent: () => boolean;
}) {
  const { deps, isCurrent } = options;
  if (!deps.needsFirstSeenBaseline?.()) {
    return null;
  }
  const scan = await deps.scanApps();
  if (!isCurrent()) {
    return null;
  }
  if (!scan.complete) {
    deps.mark?.('apps.index.baseline-incomplete', {
      scannedCount: scan.apps.length,
    });
    return null;
  }
  return indexedAppsFromScan(deps, scan, true);
}

export interface IndexedApp {
  id?: string;
  name: string;
  path?: string;
  firstSeenAt?: number;
  [key: string]: unknown;
}

export interface AppScanResult {
  apps: IndexedApp[];
  complete: boolean;
}

export interface AppIndexServiceDeps {
  scanApps: () => Promise<AppScanResult>;
  trackFirstSeen?: (
    apps: IndexedApp[],
    initializeBaseline: boolean,
  ) => IndexedApp[];
  needsFirstSeenBaseline?: () => boolean;
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
    initializeBaseline?: boolean;
    now: number;
    identityKey: (value: string) => string;
  },
) {
  const { firstSeenAtById, initialized, initializeBaseline, now, identityKey } =
    options;
  const nextFirstSeenAtById = { ...firstSeenAtById };
  let changed = false;
  const trackedApps = apps.map((app) => {
    const id = identityKey(String(app.path || app.id || app.name));
    if (!Object.hasOwn(nextFirstSeenAtById, id)) {
      nextFirstSeenAtById[id] = initialized ? now : 0;
      changed = true;
    }
    return { ...app, firstSeenAt: nextFirstSeenAtById[id] };
  });
  const nextInitialized = initialized || initializeBaseline === true;
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
    await measureAppIndex(deps, 'apps.index', { alwaysLog: true }, async () => {
      try {
        const scan = await deps.scanApps();
        if (generation !== indexGeneration) {
          return;
        }
        index = indexedAppsFromScan(deps, scan, false);
        deps.invalidateRunningStatus();
        deps.mark?.('apps.index.result', {
          scannedCount: scan.apps.length,
          complete: scan.complete,
          indexedCount: index.length,
        });
        deps.notifyIndexed(index.length);
        deps.scheduleRunningStatusRefresh('apps-indexed');
        const baselineIndex = await confirmedBaselineIndex({
          deps,
          isCurrent: () => generation === indexGeneration,
        });
        if (baselineIndex) {
          index = baselineIndex;
          deps.mark?.('apps.index.baseline', { indexedCount: index.length });
          deps.notifyIndexed(index.length);
        }
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

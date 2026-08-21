async function enrichIndexedApps(
  deps: AppIndexServiceDeps,
  apps: IndexedApp[],
) {
  if (!deps.enrichApps) {
    return apps;
  }
  try {
    return await deps.enrichApps(apps);
  } catch (error) {
    deps.error?.('applications.enrichment.failed', error);
    return apps;
  }
}

export interface IndexedApp {
  id?: string;
  name: string;
  path?: string;
  dateAddedMs?: number;
  [key: string]: unknown;
}

export interface AppIndexServiceDeps {
  scanApps: () => Promise<IndexedApp[]>;
  enrichApps?: (apps: IndexedApp[]) => Promise<IndexedApp[]>;
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
        const apps = await deps.scanApps();
        if (generation !== indexGeneration) {
          return;
        }
        index = dedupeAndSortApps(apps, deps.normalize);
        deps.invalidateRunningStatus();
        deps.mark?.('apps.index.result', {
          scannedCount: apps.length,
          indexedCount: index.length,
        });
        deps.notifyIndexed(index.length);
        deps.scheduleRunningStatusRefresh('apps-indexed');
        const enrichedApps = await enrichIndexedApps(deps, index);
        if (generation !== indexGeneration || enrichedApps === index) {
          return;
        }
        index = dedupeAndSortApps(enrichedApps, deps.normalize);
        deps.mark?.('apps.index.enriched', { indexedCount: index.length });
        deps.notifyIndexed(index.length);
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

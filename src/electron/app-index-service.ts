export type IndexedApp = { id?: string; name: string; path?: string; [key: string]: unknown }

export type AppIndexServiceDeps = {
  scanApps: () => Promise<IndexedApp[]>
  watchApps: (onChanged: () => void) => Array<{ close: () => unknown }>
  normalize: (value: string) => string
  emitChanged: () => void
  invalidateRunningStatus: () => void
  scheduleRunningStatusRefresh: (reason: string) => void
  notifyIndexed: (count: number) => void
  measure?: <T>(name: string, data: Record<string, unknown>, fn: () => Promise<T>) => Promise<T>
  mark?: (name: string, data?: Record<string, unknown>) => void
  error?: (message: string, error: unknown) => void
}

export function dedupeAndSortApps(apps: IndexedApp[], normalize: (value: string) => string) {
  const deduped = new Map<string, IndexedApp>()
  for (const item of apps) deduped.set(normalize(item.name), item)
  return Array.from(deduped.values()).sort((a, b) => a.name.localeCompare(b.name))
}

export function createAppIndexService(deps: AppIndexServiceDeps) {
  let index: IndexedApp[] = []
  let watchers: Array<{ close: () => unknown }> = []

  function measure<T>(name: string, data: Record<string, unknown>, fn: () => Promise<T>) {
    return deps.measure ? deps.measure(name, data, fn) : fn()
  }

  function get() {
    return index
  }

  function scheduleIndex() {
    deps.emitChanged()
  }

  async function startWatcher() {
    for (const watcher of watchers) watcher.close()
    watchers = deps.watchApps(scheduleIndex)
  }

  async function indexApplications() {
    await measure('apps.index', { alwaysLog: true }, async () => {
      try {
        const apps = await deps.scanApps()
        index = dedupeAndSortApps(apps, deps.normalize)
        deps.invalidateRunningStatus()
        deps.mark?.('apps.index.result', { scannedCount: apps.length, indexedCount: index.length })
        deps.notifyIndexed(index.length)
        deps.scheduleRunningStatusRefresh('apps-indexed')
      } catch (error) {
        deps.error?.('applications.index.failed', error)
      }
    })
  }

  function closeWatchers() {
    for (const watcher of watchers) watcher.close()
    watchers = []
  }

  return { get, scheduleIndex, startWatcher, indexApplications, closeWatchers }
}

export type AppIconCacheDeps = {
  hasAppIcons: () => boolean;
  hashValue: (value: string) => string;
  readCachedIcon: (cacheKey: string) => Promise<Buffer | null>;
  writeCachedIcon: (cacheKey: string, png: Buffer) => Promise<void>;
  loadIcon: (appPath: string) => Promise<Buffer>;
  schedule: (reason: string, delayMs?: number) => void;
  mark?: (name: string, data?: Record<string, unknown>) => void;
  measure?: <T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => Promise<T>,
  ) => Promise<T>;
  warn?: (message: string, data?: Record<string, unknown>) => void;
};

export function createAppIconCache(deps: AppIconCacheDeps) {
  const memoryCache = new Map<string, string | null>();
  const loadPromises = new Map<string, Promise<string | null>>();
  const pendingPaths = new Set<string>();
  const waiters = new Map<string, Array<(result: string | null) => void>>();

  function measure<T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => Promise<T>,
  ) {
    return deps.measure ? deps.measure(name, data, fn) : fn();
  }

  function dataUrl(png: Buffer) {
    return `data:image/png;base64,${png.toString('base64')}`;
  }

  async function loadAppIconDataUrl(appPath: string) {
    return measure('apps.icon.load', { appPath }, async () => {
      try {
        const cacheKey = deps.hashValue(appPath);
        const cached = await deps.readCachedIcon(cacheKey);
        if (cached) {
          deps.mark?.('apps.icon.cache-hit', { appPath });
          return dataUrl(cached);
        }

        const png = Buffer.from(await deps.loadIcon(appPath));
        await deps.writeCachedIcon(cacheKey, png).catch(() => {});
        return dataUrl(png);
      } catch (error) {
        deps.warn?.('appIcon.load.failed', { appPath, error });
        return null;
      }
    });
  }

  async function processPending() {
    const paths = Array.from(pendingPaths).slice(0, 20);
    for (const appPath of paths) pendingPaths.delete(appPath);
    await Promise.all(
      paths.map(async (appPath) => {
        const result = await loadAppIconDataUrl(appPath);
        if (result) memoryCache.set(appPath, result);
        for (const resolve of waiters.get(appPath) || []) resolve(result);
        waiters.delete(appPath);
        loadPromises.delete(appPath);
      }),
    );
    if (pendingPaths.size) deps.schedule('icon-backlog', 50);
  }

  async function get(appPath: string) {
    return measure('apps.icon.get', { appPath, alwaysLog: true }, async () => {
      if (!deps.hasAppIcons() || !appPath || !appPath.endsWith('.app'))
        return null;
      if (memoryCache.has(appPath)) {
        deps.mark?.('apps.icon.memory-cache-hit', { appPath });
        return memoryCache.get(appPath) ?? null;
      }
      const inFlight = loadPromises.get(appPath);
      if (inFlight) {
        deps.mark?.('apps.icon.in-flight-hit', { appPath });
        return inFlight;
      }

      pendingPaths.add(appPath);
      const promise = new Promise<string | null>((resolve) => {
        const currentWaiters = waiters.get(appPath) || [];
        currentWaiters.push(resolve);
        waiters.set(appPath, currentWaiters);
      });
      loadPromises.set(appPath, promise);
      deps.schedule('icon-request', 0);
      return promise;
    });
  }

  return {
    get,
    processPending,
    loadAppIconDataUrl,
    pendingCount: () => pendingPaths.size,
    memorySize: () => memoryCache.size,
  };
}

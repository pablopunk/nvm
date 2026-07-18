import { isAppIconPath } from '../app-icons';

interface AppIconCacheDeps {
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
}

const APP_ICON_LOAD_TIMEOUT_MS = 5000;
const APP_ICON_CACHE_VERSION = 'bundle-icon-v5';
const ICON_BACKLOG_DELAY_MS = 50;

function withTimeout<T>(
  promise: Promise<T>,
  timeoutMs: number,
  fallbackValue: T,
): Promise<T> {
  let timer: NodeJS.Timeout | undefined;
  const timeoutPromise = new Promise<T>((resolve) => {
    timer = setTimeout(() => resolve(fallbackValue), timeoutMs);
    timer.unref?.();
  });
  return Promise.race([promise, timeoutPromise]).finally(() => {
    if (timer) {
      clearTimeout(timer);
    }
  });
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: cache state and queue lifecycle share one closure
function createAppIconCache(deps: AppIconCacheDeps) {
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

  function loadAppIconDataUrl(appPath: string) {
    return measure('apps.icon.load', { appPath }, async () => {
      try {
        const cacheKey = deps.hashValue(`${APP_ICON_CACHE_VERSION}:${appPath}`);
        const cached = await deps.readCachedIcon(cacheKey);
        if (cached) {
          deps.mark?.('apps.icon.cache-hit', { appPath });
          return dataUrl(cached);
        }

        const pngBuffer = await withTimeout(
          deps.loadIcon(appPath).then((png) => Buffer.from(png)),
          APP_ICON_LOAD_TIMEOUT_MS,
          null,
        );
        if (!pngBuffer) {
          deps.warn?.('appIcon.load.timedOut', {
            appPath,
            timeoutMs: APP_ICON_LOAD_TIMEOUT_MS,
          });
          return null;
        }
        await deps.writeCachedIcon(cacheKey, pngBuffer).catch(() => undefined);
        return dataUrl(pngBuffer);
      } catch (error) {
        deps.warn?.('appIcon.load.failed', { appPath, error });
        return null;
      }
    });
  }

  function resolveWaitersForPath(appPath: string, result: string | null) {
    for (const resolve of waiters.get(appPath) || []) {
      resolve(result);
    }
    waiters.delete(appPath);
    loadPromises.delete(appPath);
  }

  async function processPending() {
    // Process icons one at a time. app.getFileIcon can block the main thread
    // synchronously on macOS for system apps, network drives, and broken
    // symlinks. Processing sequentially prevents multiple concurrent blocks
    // and gives the event loop a chance to fire timers between each icon.
    const nextPath = pendingPaths.values().next().value;
    if (!nextPath) {
      return;
    }
    pendingPaths.delete(nextPath);
    try {
      const result = await loadAppIconDataUrl(nextPath);
      if (result) {
        memoryCache.set(nextPath, result);
      }
      resolveWaitersForPath(nextPath, result);
    } catch {
      resolveWaitersForPath(nextPath, null);
    }
    if (pendingPaths.size > 0) {
      deps.schedule('icon-backlog', ICON_BACKLOG_DELAY_MS);
    }
  }

  function get(appPath: string) {
    return measure('apps.icon.get', { appPath, alwaysLog: true }, () => {
      if (!(deps.hasAppIcons() && isAppIconPath(appPath))) {
        return Promise.resolve(null);
      }
      if (memoryCache.has(appPath)) {
        deps.mark?.('apps.icon.memory-cache-hit', { appPath });
        return Promise.resolve(memoryCache.get(appPath) ?? null);
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

export type { AppIconCacheDeps };
export { createAppIconCache, withTimeout };

export type DataLoaderHandle = {
  _kind?: 'loader' | 'stale-while-revalidate';
  _loader: true;
  _fn: () => Promise<any[]>;
  _retry: boolean;
  _initialItems?: any[];
  /** Stale-while-revalidate: cache key for persistent extension storage. */
  _cacheKey?: string;
  /** Stale-while-revalidate: time-to-live in ms for fresh cache. */
  _ttlMs?: number;
  /** Stale-while-revalidate: time-to-live in ms for stale cache. */
  _staleTtlMs?: number;
};

export type StaleWhileRevalidateOptions = {
  cacheKey: string;
  ttlMs?: number;
  staleTtlMs?: number;
  loader: () => Promise<any[]>;
  retry?: boolean;
};

type ViewLoaderEntry = {
  fn: () => Promise<any[]>;
  retry: boolean;
  entry: any;
  handle: DataLoaderHandle;
};

export function createDataLoaderHandle(
  fn: () => Promise<any[]>,
  options: { retry?: boolean } = {},
): DataLoaderHandle {
  return {
    _kind: 'loader',
    _loader: true,
    _fn: fn,
    _retry: Boolean(options.retry),
  };
}

export function createStaleWhileRevalidateHandle(
  options: StaleWhileRevalidateOptions,
): DataLoaderHandle {
  return {
    _kind: 'stale-while-revalidate',
    _loader: true,
    _fn: options.loader,
    _retry: Boolean(options.retry),
    _cacheKey: options.cacheKey,
    _ttlMs: options.ttlMs ?? 60_000,
    _staleTtlMs: options.staleTtlMs ?? 300_000,
  };
}

export function isLoaderHandle(value: unknown): value is DataLoaderHandle {
  return Boolean(
    value &&
      typeof value === 'object' &&
      '_loader' in value &&
      typeof (value as Record<string, unknown>)._fn === 'function',
  );
}

export function isStaleWhileRevalidateHandle(
  value: unknown,
): value is DataLoaderHandle {
  return (
    isLoaderHandle(value) &&
    (value as DataLoaderHandle)._kind === 'stale-while-revalidate'
  );
}

export function normalizeLoaderItems(items: unknown) {
  if (!isLoaderHandle(items)) return items;
  if (items._initialItems) return items._initialItems;
  return [];
}

export function resolveLoaderEmptyView(
  emptyView: any,
  loaderHandle?: DataLoaderHandle,
) {
  if (!loaderHandle || emptyView) return emptyView;
  return { title: 'No items', subtitle: '' };
}

export function createViewLoaderRegistry(deps: {
  sendHydrate: (viewId: string, payload: Record<string, unknown>) => void;
  normalizeItems: (items: any[], entry: any) => any[];
  warn?: (viewId: string, message: string) => void;
  readCache?: (extension: any) => Promise<Record<string, any>>;
  writeCache?: (extension: any, data: Record<string, any>) => Promise<void>;
}) {
  const registry = new Map<string, ViewLoaderEntry>();

  async function spawn(viewId: string) {
    const loader = registry.get(viewId);
    if (!loader) return undefined;

    // Stale-while-revalidate: check persistent cache before running the loader
    const staleHandle = isStaleWhileRevalidateHandle(loader.handle)
      ? loader.handle
      : null;
    let cachedItems: any[] | undefined;
    let isFresh = false;

    if (staleHandle && deps.readCache && loader.entry?.extension) {
      try {
        const data = await deps.readCache(loader.entry.extension);
        const cached = data[staleHandle._cacheKey!];
        if (cached?.value && Array.isArray(cached.value)) {
          const age = Date.now() - Number(cached.updatedAt || 0);
          const maxFresh = staleHandle._ttlMs ?? 60_000;
          const maxStale = staleHandle._staleTtlMs ?? 300_000;

          if (age < maxFresh) {
            // Fresh cache: deliver immediately, skip the loader entirely
            isFresh = true;
            cachedItems = cached.value;
          } else if (age < maxStale) {
            // Stale cache: show items now with loading indicator, then revalidate
            cachedItems = cached.value;
          }
        }
      } catch {
        // Cache read errors are non-fatal; proceed with normal loader
      }
    }

    // Hydrate cached items immediately when available
    if (cachedItems && registry.get(viewId) === loader) {
      deps.sendHydrate(viewId, {
        items: deps.normalizeItems(cachedItems, loader.entry),
        isLoading: !isFresh, // fresh → done; stale → show loading indicator
      });
    }

    // Fresh cache: skip the loader entirely
    if (isFresh) {
      if (registry.get(viewId) === loader) registry.delete(viewId);
      return { ok: true as const, items: cachedItems };
    }

    // Run actual loader (for stale or no-cache cases)
    try {
      const items = await loader.fn();
      // Guard: skip if a newer loader was registered while we awaited
      if (registry.get(viewId) !== loader) return { ok: true as const, items };
      registry.delete(viewId);

      // Send fresh items immediately — do not delay for cache I/O
      deps.sendHydrate(viewId, {
        items: Array.isArray(items)
          ? deps.normalizeItems(items, loader.entry)
          : [],
        isLoading: false,
      });

      // Update persistent cache fire-and-forget (non-blocking, non-fatal)
      if (staleHandle && deps.writeCache && loader.entry?.extension) {
        deps
          .writeCache(loader.entry.extension, {
            ...(await (deps.readCache
              ? deps.readCache(loader.entry.extension).catch(() => ({}))
              : {})),
            [staleHandle._cacheKey!]: { value: items, updatedAt: Date.now() },
          })
          .catch(() => {});
      }
      return { ok: true as const, items };
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      // Guard: skip mutations if a newer loader was registered while we awaited
      if (registry.get(viewId) !== loader)
        return { ok: false as const, error: message, retry: loader.retry };

      // Graceful fallback for stale-while-revalidate: show stale items on failure
      if (staleHandle && cachedItems && Array.isArray(cachedItems)) {
        deps.warn?.(
          viewId,
          `Stale cache fallback for ${staleHandle._cacheKey}: ${message}`,
        );
        registry.delete(viewId);
        deps.sendHydrate(viewId, {
          items: deps.normalizeItems(cachedItems, loader.entry),
          isLoading: false,
        });
        return { ok: true as const, items: cachedItems };
      }

      if (!loader.retry) registry.delete(viewId);
      deps.sendHydrate(viewId, { error: { message }, retry: loader.retry });
      deps.warn?.(viewId, message);
      return { ok: false as const, error: message, retry: loader.retry };
    }
  }

  return {
    register(viewId: string, handle: DataLoaderHandle, entry: any) {
      registry.set(viewId, {
        fn: handle._fn,
        retry: handle._retry,
        entry,
        handle,
      });
    },
    has(viewId: string) {
      return registry.has(viewId);
    },
    retry(viewId: string) {
      return spawn(viewId);
    },
    spawn,
  };
}

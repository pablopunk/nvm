// biome-ignore-all lint: Extension cache values are arbitrary extension-owned payloads.
const EXTENSION_CACHE_MAX_TTL_MS = 24 * 60 * 60_000;
const EXTENSION_CACHE_MAX_ENTRIES = 1000;

export type ExtensionMemoryCacheDeps = {
  invalidateExtensionRootItemsForExtension: (extension: any) => void;
};

export function createExtensionMemoryCache(deps: ExtensionMemoryCacheDeps) {
  const { invalidateExtensionRootItemsForExtension } = deps;
  const extensionCaches = new Map<
    string,
    Map<string, { value: any; expiresAt: number }>
  >();

  function extensionCacheFor(extensionId) {
    let store = extensionCaches.get(extensionId);
    if (!store) {
      store = new Map();
      extensionCaches.set(extensionId, store);
    }
    return store;
  }

  function enforceExtensionCacheBudget(
    store: Map<string, { value: any; expiresAt: number }>,
  ) {
    if (store.size <= EXTENSION_CACHE_MAX_ENTRIES) return;
    const overflow = store.size - EXTENSION_CACHE_MAX_ENTRIES;
    const iterator = store.keys();
    for (let i = 0; i < overflow; i++) {
      const next = iterator.next();
      if (next.done) break;
      store.delete(next.value);
    }
  }

  function createExtensionCache(extension) {
    const store = extensionCacheFor(extension.id);
    return {
      get(key) {
        const entry = store.get(key);
        if (!entry) return;
        if (entry.expiresAt && Date.now() > entry.expiresAt) return;
        return entry.value;
      },
      getStale(key) {
        return store.get(key)?.value;
      },
      has(key) {
        const entry = store.get(key);
        if (!entry) return false;
        return !entry.expiresAt || Date.now() <= entry.expiresAt;
      },
      set(key, value, options: any = {}) {
        const rawTtl = Number(options.ttlMs || 0);
        const clampedTtl =
          rawTtl > 0 ? Math.min(rawTtl, EXTENSION_CACHE_MAX_TTL_MS) : 0;
        store.set(key, {
          value,
          expiresAt: clampedTtl > 0 ? Date.now() + clampedTtl : 0,
        });
        enforceExtensionCacheBudget(store);
        return value;
      },
      invalidate(key) {
        if (key === undefined) store.clear();
        else store.delete(key);
        invalidateExtensionRootItemsForExtension(extension);
      },
      keys() {
        return Array.from(store.keys());
      },
    };
  }

  return { extensionCacheFor, createExtensionCache };
}

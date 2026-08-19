import type {
  createExtensionJsonStore,
  JsonObject,
} from './extension-json-store';

type ExtensionJsonStore = Pick<
  ReturnType<typeof createExtensionJsonStore>,
  'mutate' | 'read' | 'replace'
>;

interface ExtensionStorageOptions {
  storagePath: string;
  cachePath: string;
  store: ExtensionJsonStore;
  refreshes?: Map<string, Promise<unknown>>;
  now?: () => number;
}

interface CachedValue {
  value: unknown;
  updatedAt?: unknown;
}

interface MemoRequest<T> {
  options: ExtensionStorageOptions;
  now: () => number;
  key: string;
  ttlMs: number;
  loader: () => Promise<T> | T;
}

interface StaleMemoRequest<T> extends MemoRequest<T> {
  refreshes: Map<string, Promise<unknown>>;
  staleTtlMs: number;
}

function cachedValue(value: unknown): CachedValue | undefined {
  return value && typeof value === 'object'
    ? (value as CachedValue)
    : undefined;
}

function mutateStorage(
  options: ExtensionStorageOptions,
  update: (current: JsonObject) => JsonObject | Promise<JsonObject>,
) {
  return options.store.mutate(options.storagePath, update);
}

function mutateCache(
  options: ExtensionStorageOptions,
  update: (current: JsonObject) => JsonObject | Promise<JsonObject>,
) {
  return options.store.mutate(options.cachePath, update);
}

async function getStorageValue<T>(
  options: ExtensionStorageOptions,
  key: string,
  fallback: T,
) {
  const data = await options.store.read(options.storagePath);
  return Object.hasOwn(data, key) ? (data[key] as T) : fallback;
}

async function setStorageValue<T>(
  options: ExtensionStorageOptions,
  key: string,
  value: T,
) {
  await mutateStorage(options, (current) => ({ ...current, [key]: value }));
  return value;
}

async function deleteStorageValue(
  options: ExtensionStorageOptions,
  key: string,
) {
  await mutateStorage(options, (current) => {
    const next = { ...current };
    delete next[key];
    return next;
  });
}

async function loadAndCacheValue<T>(
  options: ExtensionStorageOptions,
  now: () => number,
  key: string,
  loader: () => Promise<T> | T,
) {
  const value = await loader();
  await mutateCache(options, (current) => ({
    ...current,
    [key]: { value, updatedAt: now() },
  }));
  return value;
}

async function memoizedValue<T>(request: MemoRequest<T>) {
  const { options, now, key, ttlMs, loader } = request;
  const cache = await options.store.read(options.cachePath);
  const cached = cachedValue(cache[key]);
  if (cached && now() - Number(cached.updatedAt || 0) < Number(ttlMs || 0)) {
    return cached.value as T;
  }
  return loadAndCacheValue(options, now, key, loader);
}

async function staleMemoizedValue<T>(request: StaleMemoRequest<T>) {
  const { options, refreshes, now, key, ttlMs, staleTtlMs, loader } = request;
  const cache = await options.store.read(options.cachePath);
  const cached = cachedValue(cache[key]);
  const age = cached
    ? now() - Number(cached.updatedAt || 0)
    : Number.POSITIVE_INFINITY;
  if (cached && age < Number(ttlMs || 0)) {
    return cached.value as T;
  }

  const refreshKey = `${options.cachePath}:${key}`;
  const refresh =
    refreshes.get(refreshKey) ||
    loadAndCacheValue(options, now, key, loader).finally(() =>
      refreshes.delete(refreshKey),
    );
  refreshes.set(refreshKey, refresh);
  if (cached && age < Number(staleTtlMs || 0)) {
    return cached.value as T;
  }
  return refresh as Promise<T>;
}

function createExtensionStorage(options: ExtensionStorageOptions) {
  const refreshes = options.refreshes ?? new Map<string, Promise<unknown>>();
  const now = options.now ?? Date.now;

  function get<T = unknown>(key: string, fallback: T | null = null) {
    return getStorageValue(options, key, fallback);
  }
  function set<T = unknown>(key: string, value: T) {
    return setStorageValue(options, key, value);
  }
  function deleteValue(key: string) {
    return deleteStorageValue(options, key);
  }
  function clear() {
    return options.store.replace(options.storagePath, {});
  }
  function memo<T = unknown>(
    key: string,
    ttlMs: number,
    loader: () => Promise<T> | T,
  ) {
    return memoizedValue({ options, now, key, ttlMs, loader });
  }
  function memoStale<T = unknown>(
    key: string,
    ttlMs: number,
    staleTtlMs: number,
    loader: () => Promise<T> | T,
  ) {
    return staleMemoizedValue({
      options,
      refreshes,
      now,
      key,
      ttlMs,
      staleTtlMs,
      loader,
    });
  }

  return { get, set, delete: deleteValue, clear, memo, memoStale };
}

export { createExtensionStorage };

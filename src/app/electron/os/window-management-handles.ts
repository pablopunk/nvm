import { randomUUID } from 'node:crypto';

interface WindowHandleEntry extends WindowHandleTarget {
  expiresAt: number;
}

interface WindowHandleStore {
  entries: Map<string, WindowHandleEntry>;
  handlesByKey: Map<string, string>;
  now: () => number;
  createId: () => string;
}

function handleKeyFor(target: WindowHandleTarget) {
  return `${target.pid}:${target.windowKey}`;
}

function targetOf(entry: WindowHandleEntry): WindowHandleTarget {
  return { pid: entry.pid, appId: entry.appId, windowKey: entry.windowKey };
}

function forgetHandle(store: WindowHandleStore, handle: string) {
  const entry = store.entries.get(handle);
  if (!entry) {
    return;
  }
  store.entries.delete(handle);
  const key = handleKeyFor(entry);
  if (store.handlesByKey.get(key) === handle) {
    store.handlesByKey.delete(key);
  }
}

function purgeExpiredHandles(store: WindowHandleStore) {
  const current = store.now();
  for (const [handle, entry] of store.entries) {
    if (entry.expiresAt <= current) {
      forgetHandle(store, handle);
    }
  }
}

function evictOldestHandlesWhenFull(store: WindowHandleStore) {
  while (store.entries.size >= MAX_WINDOW_HANDLES) {
    const oldest = store.entries.keys().next();
    if (oldest.done) {
      return;
    }
    forgetHandle(store, oldest.value);
  }
}

function storeHandle(
  store: WindowHandleStore,
  handle: string,
  target: WindowHandleTarget,
) {
  store.entries.delete(handle);
  store.entries.set(handle, {
    ...target,
    expiresAt: store.now() + WINDOW_HANDLE_TTL_MS,
  });
  return handle;
}

function issueHandle(store: WindowHandleStore, target: WindowHandleTarget) {
  purgeExpiredHandles(store);
  const existing = store.handlesByKey.get(handleKeyFor(target));
  if (existing !== undefined) {
    return storeHandle(store, existing, target);
  }
  evictOldestHandlesWhenFull(store);
  const handle = store.createId();
  store.handlesByKey.set(handleKeyFor(target), handle);
  return storeHandle(store, handle, target);
}

function resolveHandle(store: WindowHandleStore, handle: string) {
  const entry = store.entries.get(handle);
  if (!entry) {
    return;
  }
  if (entry.expiresAt <= store.now()) {
    forgetHandle(store, handle);
    return;
  }
  return targetOf(entry);
}

export const WINDOW_HANDLE_TTL_MS = 600_000;
export const MAX_WINDOW_HANDLES = 500;

export interface WindowHandleTarget {
  pid: number;
  appId: string;
  windowKey: string;
}

export interface WindowHandleRegistryOptions {
  now?: () => number;
  createId?: () => string;
}

export interface WindowHandleRegistry {
  issue(target: WindowHandleTarget): string;
  resolve(handle: string): WindowHandleTarget | undefined;
  forget(handle: string): void;
}

export function createWindowHandleRegistry({
  now = Date.now,
  createId = randomUUID,
}: WindowHandleRegistryOptions = {}): WindowHandleRegistry {
  const store: WindowHandleStore = {
    entries: new Map(),
    handlesByKey: new Map(),
    now,
    createId,
  };
  return {
    issue: (target) => issueHandle(store, target),
    resolve: (handle) => resolveHandle(store, handle),
    forget: (handle) => forgetHandle(store, handle),
  };
}

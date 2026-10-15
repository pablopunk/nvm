// biome-ignore-all lint: Extension storage payloads are arbitrary extension-owned JSON.
import path from 'node:path';
import type { createExtensionJsonStore } from './extension-json-store';
import {
  createExtensionSecrets as createPersistentExtensionSecrets,
  type ExtensionSafeStorage,
  extensionSecretsPath,
} from './extension-secrets';
import { createExtensionStorage as createPersistentExtensionStorage } from './extension-storage';

export type ExtensionStorageServiceDeps = {
  getExtensionStorageDir: () => string;
  getExtensionCacheDir: () => string;
  getExtensionSecretsDir: () => string;
  extensionJsonStore: ReturnType<typeof createExtensionJsonStore>;
  extensionStorageRefreshes: Map<string, Promise<any>>;
  safeStorage: ExtensionSafeStorage;
};

export function extensionSourceBasename(filePath) {
  const base = path.basename(filePath || '');
  if (base.endsWith('.d.ts')) return base.slice(0, -5);
  return base.replace(/\.(cjs|ts)$/i, '');
}

function safeExtensionStorageKey(extension) {
  const key = extension.__filePath
    ? extensionSourceBasename(extension.__filePath)
    : extension.id || 'extension';
  return String(key).replace(/[^a-zA-Z0-9._-]/g, '-');
}

export function createExtensionStorageService(
  deps: ExtensionStorageServiceDeps,
) {
  const {
    getExtensionStorageDir,
    getExtensionCacheDir,
    getExtensionSecretsDir,
    extensionJsonStore,
    extensionStorageRefreshes,
    safeStorage,
  } = deps;

  function extensionStoragePath(extension) {
    return path.join(
      getExtensionStorageDir(),
      `${safeExtensionStorageKey(extension)}.json`,
    );
  }

  function extensionCachePath(extension) {
    return path.join(
      getExtensionCacheDir(),
      `${safeExtensionStorageKey(extension)}.json`,
    );
  }

  async function readExtensionStorage(extension) {
    return extensionJsonStore.read(extensionStoragePath(extension));
  }

  async function mutateExtensionStorage(extension, update) {
    return extensionJsonStore.mutate(extensionStoragePath(extension), update);
  }

  async function readExtensionCache(extension) {
    return extensionJsonStore.read(extensionCachePath(extension));
  }

  async function mutateExtensionCache(extension, update) {
    return extensionJsonStore.mutate(extensionCachePath(extension), update);
  }

  function createExtensionStorage(extension) {
    return createPersistentExtensionStorage({
      storagePath: extensionStoragePath(extension),
      cachePath: extensionCachePath(extension),
      store: extensionJsonStore,
      refreshes: extensionStorageRefreshes,
    });
  }

  function createExtensionSecrets(extension) {
    return createPersistentExtensionSecrets({
      storagePath: extensionSecretsPath(getExtensionSecretsDir(), extension.id),
      store: extensionJsonStore,
      safeStorage,
    });
  }

  return {
    readExtensionStorage,
    mutateExtensionStorage,
    readExtensionCache,
    mutateExtensionCache,
    createExtensionStorage,
    createExtensionSecrets,
  };
}

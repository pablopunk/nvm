import fs from 'node:fs/promises';
import path from 'node:path';

const FILENAME = 'byo-key.json';

interface StoredByoKey {
  encryptedApiKey?: string;
  apiKey?: string;
  providerId: string;
  provider: string;
  api: string;
  baseUrl: string;
  modelId: string;
  modelName: string;
  createdAt: string;
}

type ByoKeySnapshot = {
  providerId: string;
  apiKey: string;
  provider: string;
  api: string;
  baseUrl: string;
  modelId: string;
  modelName: string;
} | null;

interface SafeStorageLike {
  isEncryptionAvailable(): boolean;
  encryptString(v: string): Buffer;
  decryptString(b: Buffer): string;
}

let safeStorageImpl: SafeStorageLike | null = null;
let filePathOverride: string | null = null;
let logWarn: (message: string, data?: unknown) => void = () => {
  /* no-op: overridden in tests */
};

function getSafeStorage(): SafeStorageLike {
  if (safeStorageImpl) {
    return safeStorageImpl;
  }
  const { safeStorage } = require('electron');
  return safeStorage;
}

function byoKeyPath() {
  if (filePathOverride) {
    return filePathOverride;
  }
  const { app } = require('electron');
  return path.join(app.getPath('userData'), FILENAME);
}

function decryptApiKey(data: StoredByoKey): string | null {
  const storage = getSafeStorage();
  if (data.encryptedApiKey && storage.isEncryptionAvailable()) {
    return storage.decryptString(Buffer.from(data.encryptedApiKey, 'base64'));
  }
  if (data.apiKey) {
    if (!storage.isEncryptionAvailable()) {
      logWarn('safeStorage unavailable; reading plaintext BYO key');
    }
    return Buffer.from(data.apiKey, 'base64').toString('utf8');
  }
  return null;
}

let cached: ByoKeySnapshot = null;
let loadPromise: Promise<ByoKeySnapshot> | null = null;

async function getByoKey(): Promise<ByoKeySnapshot> {
  if (loadPromise) return loadPromise;
  loadPromise = (async () => {
    try {
      const raw = await fs.readFile(byoKeyPath(), 'utf8');
      const data = JSON.parse(raw) as StoredByoKey;
      const apiKey = decryptApiKey(data);
      if (!apiKey) {
        cached = null;
        return null;
      }
      cached = {
        providerId: data.providerId,
        apiKey,
        provider: data.provider,
        api: data.api,
        baseUrl: data.baseUrl,
        modelId: data.modelId,
        modelName: data.modelName,
      };
      return cached;
    } catch {
      cached = null;
      return null;
    }
  })();
  return loadPromise;
}

function getCachedByoKey(): ByoKeySnapshot {
  return cached;
}

async function persistByoKey(input: {
  providerId: string;
  apiKey: string;
  provider: string;
  api: string;
  baseUrl: string;
  modelId: string;
  modelName: string;
}) {
  const payload: StoredByoKey = {
    providerId: input.providerId,
    provider: input.provider,
    api: input.api,
    baseUrl: input.baseUrl,
    modelId: input.modelId,
    modelName: input.modelName,
    createdAt: new Date().toISOString(),
  };
  const storage = getSafeStorage();
  if (storage.isEncryptionAvailable()) {
    payload.encryptedApiKey = storage
      .encryptString(input.apiKey)
      .toString('base64');
  } else {
    logWarn('safeStorage unavailable; storing BYO key as plaintext');
    payload.apiKey = Buffer.from(input.apiKey, 'utf8').toString('base64');
  }
  await fs.writeFile(byoKeyPath(), JSON.stringify(payload, null, 2), {
    mode: 0o600,
  });
  if (process.platform !== 'win32') await fs.chmod(byoKeyPath(), 0o600);
  cached = {
    providerId: input.providerId,
    apiKey: input.apiKey,
    provider: input.provider,
    api: input.api,
    baseUrl: input.baseUrl,
    modelId: input.modelId,
    modelName: input.modelName,
  };
  loadPromise = Promise.resolve(cached);
  return cached;
}

async function clearByoKey() {
  await fs.rm(byoKeyPath(), { force: true });
  cached = null;
  loadPromise = Promise.resolve(null);
}

function setByoKeyStorageForTests(impl: SafeStorageLike | null) {
  safeStorageImpl = impl;
}

function setByoKeyFilePathForTests(filePath: string | null) {
  filePathOverride = filePath;
}

function clearByoKeyCacheForTests() {
  cached = null;
  loadPromise = null;
}

function setByoKeyLoggerForTests(logger: {
  warn: (message: string, data?: unknown) => void;
}) {
  logWarn = logger.warn.bind(logger);
}

export {
  getCachedByoKey,
  getByoKey,
  persistByoKey,
  clearByoKey,
  setByoKeyStorageForTests,
  setByoKeyFilePathForTests,
  clearByoKeyCacheForTests,
  setByoKeyLoggerForTests,
  type ByoKeySnapshot,
  type SafeStorageLike,
};

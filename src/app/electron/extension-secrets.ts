import { createHash, randomBytes } from 'node:crypto';
import path from 'node:path';
import type { createExtensionJsonStore } from './extension-json-store';

type ExtensionJsonStore = Pick<
  ReturnType<typeof createExtensionJsonStore>,
  'mutate' | 'read' | 'replace'
>;

interface ExtensionSafeStorage {
  isEncryptionAvailable(): boolean;
  encryptString(value: string): Buffer;
  isAsyncEncryptionAvailable(): Promise<boolean>;
  encryptStringAsync(value: string): Promise<Buffer>;
  decryptStringAsync(value: Buffer): Promise<{
    result: string;
    shouldReEncrypt: boolean;
  }>;
  getSelectedStorageBackend?(): string;
}

interface ExtensionSecretsOptions {
  storagePath: string;
  store: ExtensionJsonStore;
  safeStorage: ExtensionSafeStorage;
}

type SecretValues = Record<string, string>;

const ENCRYPTED_PAYLOAD_VERSION = 1;
const verifiedLinuxEncryption = new WeakMap<
  ExtensionSafeStorage,
  Promise<void>
>();

function secureStorageOperationError(error: unknown, fallback: string) {
  if (
    error instanceof Error &&
    /temporarily unavailable/i.test(error.message)
  ) {
    return new Error('OS secure storage is temporarily unavailable; try again');
  }
  return new Error(fallback);
}

function extensionSecretsPath(directory: string, extensionId: string) {
  const id = String(extensionId || '').trim();
  if (!id) throw new TypeError('Extension secrets require an extension id');
  const fileName = `${createHash('sha256').update(id).digest('hex')}.json`;
  return path.join(directory, fileName);
}

function assertSecretName(name: string) {
  if (typeof name !== 'string' || !name.trim() || name.length > 128) {
    throw new TypeError('Secret names must be 1–128 characters');
  }
}

function assertSecretValue(value: string) {
  if (typeof value !== 'string') {
    throw new TypeError('Extension secrets must be strings');
  }
}

async function verifyLinuxEncryption(safeStorage: ExtensionSafeStorage) {
  if (!safeStorage.isEncryptionAvailable()) {
    throw new Error(
      'Secure extension secrets require an available Linux system keychain',
    );
  }

  const probe = randomBytes(32).toString('hex');
  let decrypted: { result: string; shouldReEncrypt: boolean };
  try {
    const legacyCiphertext = safeStorage.encryptString(probe);
    decrypted = await safeStorage.decryptStringAsync(legacyCiphertext);
  } catch {
    throw new Error('Unable to verify secure Linux extension secret storage');
  }

  if (decrypted.result !== probe || decrypted.shouldReEncrypt !== false) {
    throw new Error(
      'Secure Linux extension secret storage could not be verified; refusing to store secrets',
    );
  }
}

async function assertSecureEncryption(safeStorage: ExtensionSafeStorage) {
  if (!(await safeStorage.isAsyncEncryptionAvailable())) {
    throw new Error(
      'Secure extension secrets are unavailable because OS encryption is not ready',
    );
  }
  // getSelectedStorageBackend exists only on Linux builds. Its presence
  // selects the extra Linux verification; other platforms rely on async
  // availability alone without a platform branch.
  if (typeof safeStorage.getSelectedStorageBackend !== 'function') return;

  const backend = safeStorage.getSelectedStorageBackend?.();
  if (!backend || backend === 'unknown' || backend === 'basic_text') {
    throw new Error(
      'Secure extension secrets require a system keychain or credential store on Linux',
    );
  }

  let verification = verifiedLinuxEncryption.get(safeStorage);
  if (!verification) {
    verification = verifyLinuxEncryption(safeStorage);
    verifiedLinuxEncryption.set(safeStorage, verification);
    verification.catch(() => {
      if (verifiedLinuxEncryption.get(safeStorage) === verification) {
        verifiedLinuxEncryption.delete(safeStorage);
      }
    });
  }
  await verification;
}

function secretValuesFromJson(value: string): SecretValues {
  const parsed: unknown = JSON.parse(value);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Stored extension secrets are invalid');
  }
  const values: SecretValues = Object.create(null);
  for (const [name, secret] of Object.entries(parsed)) {
    if (typeof secret !== 'string') {
      throw new Error('Stored extension secrets are invalid');
    }
    values[name] = secret;
  }
  return values;
}

async function decryptSecretValues(
  data: Record<string, unknown>,
  safeStorage: ExtensionSafeStorage,
): Promise<{ values: SecretValues; shouldReEncrypt: boolean }> {
  if (!Object.hasOwn(data, 'encryptedPayload')) {
    return { values: Object.create(null), shouldReEncrypt: false };
  }
  if (
    data.version !== ENCRYPTED_PAYLOAD_VERSION ||
    typeof data.encryptedPayload !== 'string'
  ) {
    throw new Error('Stored extension secrets are invalid');
  }

  await assertSecureEncryption(safeStorage);
  let decrypted: { result: string; shouldReEncrypt: boolean };
  try {
    decrypted = await safeStorage.decryptStringAsync(
      Buffer.from(data.encryptedPayload, 'base64'),
    );
  } catch (error) {
    throw secureStorageOperationError(
      error,
      'Unable to decrypt stored extension secrets',
    );
  }
  if (typeof decrypted.shouldReEncrypt !== 'boolean') {
    throw new Error(
      'Stored extension secrets require a newer secure-storage format',
    );
  }
  return {
    values: secretValuesFromJson(decrypted.result),
    shouldReEncrypt: decrypted.shouldReEncrypt,
  };
}

async function encryptSecretValues(
  values: SecretValues,
  safeStorage: ExtensionSafeStorage,
) {
  await assertSecureEncryption(safeStorage);
  let encryptedPayload: Buffer;
  try {
    encryptedPayload = await safeStorage.encryptStringAsync(
      JSON.stringify(values),
    );
  } catch (error) {
    throw secureStorageOperationError(
      error,
      'Unable to encrypt stored extension secrets',
    );
  }
  return {
    version: ENCRYPTED_PAYLOAD_VERSION,
    encryptedPayload: encryptedPayload.toString('base64'),
  };
}

function createExtensionSecrets(options: ExtensionSecretsOptions) {
  async function get(name: string): Promise<string | undefined> {
    assertSecretName(name);
    const data = await options.store.read(options.storagePath);
    const decrypted = await decryptSecretValues(data, options.safeStorage);
    if (decrypted.shouldReEncrypt) {
      await options.store.mutate(options.storagePath, async (current) => {
        const latest = await decryptSecretValues(current, options.safeStorage);
        if (!latest.shouldReEncrypt) return current;
        return encryptSecretValues(latest.values, options.safeStorage);
      });
    }
    return Object.hasOwn(decrypted.values, name)
      ? decrypted.values[name]
      : undefined;
  }

  async function set(name: string, value: string): Promise<void> {
    assertSecretName(name);
    assertSecretValue(value);
    await options.store.mutate(options.storagePath, async (current) => {
      const decrypted = await decryptSecretValues(current, options.safeStorage);
      const values = decrypted.values;
      values[name] = value;
      return encryptSecretValues(values, options.safeStorage);
    });
  }

  async function deleteSecret(name: string): Promise<void> {
    assertSecretName(name);
    await options.store.mutate(options.storagePath, async (current) => {
      const decrypted = await decryptSecretValues(current, options.safeStorage);
      const values = decrypted.values;
      delete values[name];
      if (Object.keys(values).length === 0) return {};
      return encryptSecretValues(values, options.safeStorage);
    });
  }

  async function clear(): Promise<void> {
    await options.store.replace(options.storagePath, {});
  }

  return { get, set, delete: deleteSecret, clear };
}

export type { ExtensionSafeStorage };
export { createExtensionSecrets, extensionSecretsPath };

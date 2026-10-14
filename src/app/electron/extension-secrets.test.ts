import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createExtensionJsonStore } from './extension-json-store';
import {
  createExtensionSecrets,
  extensionSecretsPath,
  type ExtensionSafeStorage,
} from './extension-secrets';

const secretName = 'PAPERLESS_API_TOKEN';
const secretValue = 'paperless-private-token';

function createTestSafeStorage(
  options: {
    available?: boolean;
    backend?: string;
    syncAvailable?: boolean;
    probeShouldReEncrypt?: boolean;
    reEncryptStoredSecrets?: boolean;
    encryptUnavailable?: boolean;
    decryptUnavailable?: boolean;
    asyncEncryptionCalls?: number;
  } = {},
): ExtensionSafeStorage {
  return {
    isEncryptionAvailable() {
      return options.syncAvailable ?? true;
    },
    encryptString(value) {
      return Buffer.from(`sync-probe:${value}`);
    },
    async isAsyncEncryptionAvailable() {
      return options.available ?? true;
    },
    async encryptStringAsync(value) {
      if (options.encryptUnavailable) {
        throw new Error('temporarily unavailable');
      }
      options.asyncEncryptionCalls = (options.asyncEncryptionCalls ?? 0) + 1;
      options.reEncryptStoredSecrets = false;
      return Buffer.from(
        `test-encrypted:${options.asyncEncryptionCalls}:${Buffer.from(value).toString('base64')}`,
      );
    },
    async decryptStringAsync(value) {
      if (options.decryptUnavailable) {
        throw new Error('temporarily unavailable');
      }
      const ciphertext = value.toString('utf8');
      if (ciphertext.startsWith('sync-probe:')) {
        return {
          result: ciphertext.slice('sync-probe:'.length),
          shouldReEncrypt: options.probeShouldReEncrypt ?? false,
        };
      }
      const [, , encoded] = ciphertext.split(':');
      return {
        result: Buffer.from(encoded, 'base64').toString('utf8'),
        shouldReEncrypt: options.reEncryptStoredSecrets ?? false,
      };
    },
    getSelectedStorageBackend() {
      return options.backend ?? 'gnome_libsecret';
    },
  };
}

async function withSecrets(
  callback: (input: {
    secrets: ReturnType<typeof createExtensionSecrets>;
    storagePath: string;
  }) => Promise<void>,
  options: {
    platform?: NodeJS.Platform;
    safeStorage?: ExtensionSafeStorage;
  } = {},
) {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-extension-secrets-'),
  );
  const storagePath = path.join(directory, 'secrets.json');
  const secrets = createExtensionSecrets({
    storagePath,
    store: createExtensionJsonStore(),
    safeStorage: options.safeStorage ?? createTestSafeStorage(),
    platform: options.platform ?? 'linux',
  });
  try {
    await callback({ secrets, storagePath });
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
}

test('stores an opaque encrypted payload without plaintext keys or values', async () => {
  await withSecrets(async ({ secrets, storagePath }) => {
    await secrets.set(secretName, secretValue);

    assert.equal(await secrets.get(secretName), secretValue);
    const stored = await fs.readFile(storagePath, 'utf8');
    assert.equal(stored.includes(secretName), false);
    assert.equal(stored.includes(secretValue), false);
  });
});

test('concurrent writes preserve separate secrets', async () => {
  await withSecrets(async ({ secrets }) => {
    await Promise.all([
      secrets.set('PAPERLESS_API_TOKEN', 'paperless-token'),
      secrets.set('PAPERLESS_URL', 'https://docs.example'),
    ]);

    assert.equal(await secrets.get('PAPERLESS_API_TOKEN'), 'paperless-token');
    assert.equal(await secrets.get('PAPERLESS_URL'), 'https://docs.example');
  });
});

test('delete and clear remove only this extension secret data', async () => {
  await withSecrets(async ({ secrets, storagePath }) => {
    await secrets.set('first', 'one');
    await secrets.set('second', 'two');
    await secrets.delete('first');
    assert.equal(await secrets.get('first'), undefined);
    assert.equal(await secrets.get('second'), 'two');

    await secrets.clear();
    assert.deepEqual(JSON.parse(await fs.readFile(storagePath, 'utf8')), {});
    assert.equal(await secrets.get('second'), undefined);
  });
});

test('fails closed without writing when secure encryption is unavailable', async () => {
  await withSecrets(
    async ({ secrets, storagePath }) => {
      await assert.rejects(
        secrets.set(secretName, secretValue),
        /OS encryption is not ready/,
      );
      await assert.rejects(fs.readFile(storagePath), { code: 'ENOENT' });
    },
    { safeStorage: createTestSafeStorage({ available: false }) },
  );
});

test('fails closed when Linux has no secure legacy keyring', async () => {
  await withSecrets(
    async ({ secrets, storagePath }) => {
      await assert.rejects(
        secrets.set(secretName, secretValue),
        /available Linux system keychain/,
      );
      await assert.rejects(fs.readFile(storagePath), { code: 'ENOENT' });
    },
    { safeStorage: createTestSafeStorage({ syncAvailable: false }) },
  );
});

test('fails closed when the Linux async provider differs from the keyring', async () => {
  await withSecrets(
    async ({ secrets, storagePath }) => {
      await assert.rejects(
        secrets.set(secretName, secretValue),
        /could not be verified/,
      );
      await assert.rejects(fs.readFile(storagePath), { code: 'ENOENT' });
    },
    { safeStorage: createTestSafeStorage({ probeShouldReEncrypt: true }) },
  );
});

test('fails closed for Linux basic_text and unknown storage backends', async () => {
  for (const backend of ['basic_text', 'unknown']) {
    await withSecrets(
      async ({ secrets, storagePath }) => {
        await assert.rejects(
          secrets.set(secretName, secretValue),
          /system keychain or credential store/,
        );
        await assert.rejects(fs.readFile(storagePath), { code: 'ENOENT' });
      },
      { safeStorage: createTestSafeStorage({ backend }) },
    );
  }
});

test('re-encrypts stored values when safeStorage reports key rotation', async () => {
  const safeStorageOptions: {
    reEncryptStoredSecrets: boolean;
    asyncEncryptionCalls?: number;
  } = { reEncryptStoredSecrets: false };
  const safeStorage = createTestSafeStorage(safeStorageOptions);
  await withSecrets(
    async ({ secrets }) => {
      await secrets.set(secretName, secretValue);
      assert.equal(safeStorageOptions.asyncEncryptionCalls, 1);

      safeStorageOptions.reEncryptStoredSecrets = true;
      assert.equal(await secrets.get(secretName), secretValue);
      assert.equal(safeStorageOptions.asyncEncryptionCalls, 2);
    },
    { safeStorage },
  );
});

test('does not replace the old payload when key rotation cannot be resealed', async () => {
  const safeStorageOptions: {
    reEncryptStoredSecrets: boolean;
    encryptUnavailable: boolean;
  } = { reEncryptStoredSecrets: false, encryptUnavailable: false };
  const safeStorage = createTestSafeStorage(safeStorageOptions);
  await withSecrets(
    async ({ secrets, storagePath }) => {
      await secrets.set(secretName, secretValue);
      const original = await fs.readFile(storagePath, 'utf8');
      safeStorageOptions.reEncryptStoredSecrets = true;
      safeStorageOptions.encryptUnavailable = true;

      await assert.rejects(
        secrets.get(secretName),
        /OS secure storage is temporarily unavailable/,
      );
      assert.equal(await fs.readFile(storagePath, 'utf8'), original);
    },
    { safeStorage },
  );
});

test('reports temporary keychain unavailability as retryable', async () => {
  const safeStorageOptions = { decryptUnavailable: false };
  const safeStorage = createTestSafeStorage(safeStorageOptions);
  await withSecrets(
    async ({ secrets }) => {
      await secrets.set(secretName, secretValue);
      safeStorageOptions.decryptUnavailable = true;
      await assert.rejects(
        secrets.get(secretName),
        /OS secure storage is temporarily unavailable/,
      );
    },
    { safeStorage },
  );
});

test('derives private stable storage paths from extension IDs', () => {
  const first = extensionSecretsPath('/private/secrets', 'example.search');
  assert.equal(
    first,
    extensionSecretsPath('/private/secrets', 'example.search'),
  );
  assert.notEqual(
    first,
    extensionSecretsPath('/private/secrets', 'other.search'),
  );
  assert.equal(first.includes('example.search'), false);
  assert.throws(
    () => extensionSecretsPath('/private/secrets', ''),
    /extension id/,
  );
});

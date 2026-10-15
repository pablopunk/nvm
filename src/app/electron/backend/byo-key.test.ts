import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {
  getByoKey,
  persistByoKey,
  clearByoKey,
  setByoKeyStorageForTests,
  setByoKeyFilePathForTests,
  clearByoKeyCacheForTests,
  type SafeStorageLike,
} from './byo-key';

function createFakeSafeStorage(): SafeStorageLike {
  return {
    isEncryptionAvailable: () => true,
    encryptString: (v: string) => Buffer.from(v, 'utf8'),
    decryptString: (b: Buffer) => b.toString('utf8'),
  };
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: test suite with multiple test cases
describe('byo-key', () => {
  let testDir: string;
  let testFilePath: string;

  beforeEach(async () => {
    testDir = await fs.mkdtemp(path.join(os.tmpdir(), 'byo-key-test-'));
    testFilePath = path.join(testDir, 'byo-key.json');
    setByoKeyFilePathForTests(testFilePath);
    setByoKeyStorageForTests(createFakeSafeStorage());
    clearByoKeyCacheForTests();
  });

  afterEach(async () => {
    setByoKeyFilePathForTests(null);
    setByoKeyStorageForTests(null);
    clearByoKeyCacheForTests();
    await fs.rm(testDir, { recursive: true, force: true });
  });

  it('returns null when no key stored', async () => {
    const key = await getByoKey();
    assert.equal(key, null);
  });

  it('persists and retrieves a key', async () => {
    await persistByoKey({
      providerId: 'openai',
      apiKey: 'sk-test-key-123',
      provider: 'openai',
      api: 'openai',
      baseUrl: 'https://api.openai.com/v1',
      modelId: 'gpt-4',
      modelName: 'GPT-4',
    });
    const key = await getByoKey();
    assert.ok(key);
    assert.equal(key.providerId, 'openai');
    assert.equal(key.apiKey, 'sk-test-key-123');
    assert.equal(key.provider, 'openai');
    assert.equal(key.api, 'openai');
    assert.equal(key.baseUrl, 'https://api.openai.com/v1');
    assert.equal(key.modelId, 'gpt-4');
    assert.equal(key.modelName, 'GPT-4');
  });

  it('clears the key', async () => {
    await persistByoKey({
      providerId: 'openai',
      apiKey: 'sk-test-key',
      provider: 'openai',
      api: 'openai',
      baseUrl: 'https://api.openai.com/v1',
      modelId: 'gpt-4',
      modelName: 'GPT-4',
    });
    await clearByoKey();
    const key = await getByoKey();
    assert.equal(key, null);
  });

  it('returns null after cleared', async () => {
    await persistByoKey({
      providerId: 'openai',
      apiKey: 'sk-test-key',
      provider: 'openai',
      api: 'openai',
      baseUrl: 'https://api.openai.com/v1',
      modelId: 'gpt-4',
      modelName: 'GPT-4',
    });
    await clearByoKey();
    assert.equal(await getByoKey(), null);
  });

  it('handles unavailable safeStorage by degrading to plaintext', async () => {
    setByoKeyStorageForTests({
      isEncryptionAvailable: () => false,
      encryptString: () => {
        throw new Error('not available');
      },
      decryptString: () => {
        throw new Error('not available');
      },
    });
    const input = {
      providerId: 'openai',
      apiKey: 'sk-plaintext',
      provider: 'openai',
      api: 'openai',
      baseUrl: 'https://api.openai.com/v1',
      modelId: 'gpt-4',
      modelName: 'GPT-4',
    };
    await persistByoKey(input);
    const key = await getByoKey();
    assert.ok(key);
    assert.equal(key.apiKey, 'sk-plaintext');
  });
});

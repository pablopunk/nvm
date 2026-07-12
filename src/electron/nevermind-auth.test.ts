import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

mock.module('electron', {
  namedExports: {
    app: { getPath: () => '/tmp/nevermind-auth-test' },
    safeStorage: {
      isEncryptionAvailable: () => false,
      encryptString: () => Buffer.from(''),
      decryptString: () => '',
    },
  },
});

const {
  clearNevermindAuthCacheForTests,
  getNevermindAuth,
  setActiveNevermindAuthBaseUrl,
  setNevermindAuthFilePathForTests,
} = await import('./nevermind-auth');

const production = 'https://api.nvm.fyi';

function storedAuth(baseUrl: string, token: string) {
  return {
    token: Buffer.from(token).toString('base64'),
    email: `${token}@example.com`,
    role: 'member',
    baseUrl,
    connectedAt: new Date().toISOString(),
  };
}

test('migrates a legacy auth file into the per-origin store', async () => {
  const userData = await fs.mkdtemp(path.join(os.tmpdir(), 'nevermind-auth-'));
  const storePath = path.join(userData, 'nevermind-auth-by-origin.json');
  const legacyPath = path.join(userData, 'nevermind-auth.json');
  setNevermindAuthFilePathForTests(storePath);
  setActiveNevermindAuthBaseUrl(production);
  await fs.writeFile(legacyPath, JSON.stringify(storedAuth(production, 'one')));

  const auth = await getNevermindAuth();
  assert.equal(auth?.token, 'one');
  assert.deepEqual(
    JSON.parse(await fs.readFile(storePath, 'utf8'))[production].token,
    Buffer.from('one').toString('base64'),
  );
  assert.equal(await fs.stat(`${legacyPath}.bak`).then(() => true), true);
  await fs.rm(userData, { recursive: true, force: true });
  setNevermindAuthFilePathForTests(null);
  clearNevermindAuthCacheForTests();
});

test('leaves a corrupt legacy auth file in place', async () => {
  const userData = await fs.mkdtemp(path.join(os.tmpdir(), 'nevermind-auth-'));
  const storePath = path.join(userData, 'nevermind-auth-by-origin.json');
  const legacyPath = path.join(userData, 'nevermind-auth.json');
  setNevermindAuthFilePathForTests(storePath);
  setActiveNevermindAuthBaseUrl(production);
  await fs.writeFile(legacyPath, 'not json');

  assert.equal(await getNevermindAuth(), null);
  assert.equal(
    await fs.stat(`${legacyPath}.bak`).then(
      () => true,
      () => false,
    ),
    false,
  );
  await fs.rm(userData, { recursive: true, force: true });
  setNevermindAuthFilePathForTests(null);
  clearNevermindAuthCacheForTests();
});

test('handles an empty userData directory and skips migration after success', async () => {
  const userData = await fs.mkdtemp(path.join(os.tmpdir(), 'nevermind-auth-'));
  const storePath = path.join(userData, 'nevermind-auth-by-origin.json');
  const legacyPath = path.join(userData, 'nevermind-auth.json');
  setNevermindAuthFilePathForTests(storePath);
  setActiveNevermindAuthBaseUrl(production);
  assert.equal(await getNevermindAuth(), null);

  await fs.writeFile(legacyPath, JSON.stringify(storedAuth(production, 'one')));
  clearNevermindAuthCacheForTests();
  setActiveNevermindAuthBaseUrl(production);
  await getNevermindAuth();
  const backupMtime = (await fs.stat(`${legacyPath}.bak`)).mtimeMs;
  clearNevermindAuthCacheForTests();
  setActiveNevermindAuthBaseUrl(production);
  assert.equal((await getNevermindAuth())?.token, 'one');
  assert.equal((await fs.stat(`${legacyPath}.bak`)).mtimeMs, backupMtime);

  await fs.rm(userData, { recursive: true, force: true });
  setNevermindAuthFilePathForTests(null);
  clearNevermindAuthCacheForTests();
});

test('isolates auth snapshots by active backend origin', async () => {
  const userData = await fs.mkdtemp(path.join(os.tmpdir(), 'nevermind-auth-'));
  const storePath = path.join(userData, 'nevermind-auth-by-origin.json');
  const preview = 'https://preview.example.com';
  setNevermindAuthFilePathForTests(storePath);
  await fs.writeFile(
    storePath,
    JSON.stringify({
      [production]: storedAuth(production, 'production-token'),
      [preview]: storedAuth(preview, 'preview-token'),
    }),
  );

  setActiveNevermindAuthBaseUrl(preview);
  assert.equal((await getNevermindAuth())?.token, 'preview-token');
  setActiveNevermindAuthBaseUrl(production);
  assert.equal((await getNevermindAuth())?.token, 'production-token');

  await fs.rm(userData, { recursive: true, force: true });
  setNevermindAuthFilePathForTests(null);
  clearNevermindAuthCacheForTests();
});

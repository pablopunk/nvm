import assert from 'node:assert/strict';
import test from 'node:test';
import {
  type NevermindBackendEnvironment,
  type NevermindBackendEnvironmentDeps,
  PRODUCTION_NEVERMIND_BASE_URL,
  switchNevermindBackendEnvironment,
} from './nevermind-backend-environment';

const manifest = {
  backend: { environment: 'preview', version: 'abcdef0' },
  api: { currentVersion: 1, supportedVersions: [1] },
  desktop: {
    minimumSupportedVersion: '0.1.0',
    latestVersion: '0.2.0',
    updateUrl: 'https://example.com/update',
  },
  client: { compatible: true, unsupportedReason: null },
  features: { proxy_streaming: true },
};

function createDeps(overrides: Partial<NevermindBackendEnvironmentDeps> = {}) {
  const calls: string[] = [];
  let selection: NevermindBackendEnvironment = {
    environment: 'production',
    baseUrl: PRODUCTION_NEVERMIND_BASE_URL,
  };
  let activeAuthBaseUrl = selection.baseUrl;
  const deps: NevermindBackendEnvironmentDeps = {
    isPackaged: false,
    selectedEnvironment: () => selection,
    resolvesToUnsafeAddress: async () => false,
    invalidateCompatibilityCache: async (baseUrl) => {
      calls.push(`invalidate:${baseUrl}`);
    },
    checkCompatibility: async (baseUrl) => {
      calls.push(`check:${baseUrl}`);
      return manifest;
    },
    setSelectedEnvironment: (next) => {
      selection = next;
      calls.push(`select:${next.baseUrl}`);
    },
    scheduleSaveState: () => calls.push('save'),
    setActiveAuthBaseUrl: (baseUrl) => {
      activeAuthBaseUrl = baseUrl;
      calls.push(`auth-origin:${baseUrl}`);
    },
    getAuth: async () => {
      calls.push(`get-auth:${activeAuthBaseUrl}`);
      return {
        token: 'preview-token',
        email: 'preview@example.com',
        role: 'member',
        environment: 'custom',
        baseUrl: activeAuthBaseUrl,
      };
    },
    signIn: async () => ({ ok: false, error: 'unexpected sign-in' }),
    setActiveBaseUrl: (baseUrl) => calls.push(`active:${baseUrl}`),
    warmCompatibilityCache: (baseUrl) => calls.push(`warm:${baseUrl}`),
    disposeAiSessions: async () => {
      calls.push('dispose-ai');
    },
    invalidateExtensionRootItems: () => calls.push('invalidate-extensions'),
    broadcastAuthChanged: ({ email }) => calls.push(`broadcast:${email}`),
    ...overrides,
  };
  return { deps, calls, selection: () => selection };
}

test('rejects malformed, insecure, credentialed, and packaged-loopback URLs', async (t) => {
  const cases = [
    {
      name: 'malformed',
      baseUrl: 'not a URL',
      message: 'Enter a valid backend URL.',
    },
    {
      name: 'http',
      baseUrl: 'http://preview.example',
      message: 'Backend URL must use HTTPS.',
    },
    {
      name: 'credentials',
      baseUrl: 'https://user:secret@preview.example',
      message: 'Backend URL must not include credentials.',
    },
  ];
  for (const scenario of cases) {
    await t.test(scenario.name, async () => {
      const { deps, calls } = createDeps();
      assert.deepEqual(
        await switchNevermindBackendEnvironment(
          { environment: 'custom', baseUrl: scenario.baseUrl },
          deps,
        ),
        { ok: false, message: scenario.message },
      );
      assert.deepEqual(calls, []);
    });
  }

  const { deps, calls } = createDeps({
    isPackaged: true,
    resolvesToUnsafeAddress: async (hostname) => hostname === 'localhost',
  });
  assert.equal(
    (
      await switchNevermindBackendEnvironment(
        { environment: 'custom', baseUrl: 'https://localhost:4443' },
        deps,
      )
    ).ok,
    false,
  );
  assert.deepEqual(calls, []);
});

test('keeps the active backend unchanged when compatibility validation fails', async (t) => {
  for (const scenario of [
    { name: 'missing manifest', result: null, error: null },
    {
      name: 'network error',
      result: null,
      error: new Error('compatibility request failed'),
    },
    {
      name: 'incompatible manifest',
      result: null,
      error: new Error('This Nevermind version is no longer supported.'),
    },
  ]) {
    await t.test(scenario.name, async () => {
      const { deps, calls, selection } = createDeps({
        checkCompatibility: async () => {
          if (scenario.error) throw scenario.error;
          return scenario.result;
        },
      });
      const result = await switchNevermindBackendEnvironment(
        { environment: 'custom', baseUrl: 'https://preview.example/' },
        deps,
      );
      assert.equal(result.ok, false);
      assert.deepEqual(selection(), {
        environment: 'production',
        baseUrl: PRODUCTION_NEVERMIND_BASE_URL,
      });
      assert.deepEqual(calls, ['invalidate:https://preview.example']);
    });
  }
});

test('switches only after validation and directs subsequent auth work to the normalized origin', async () => {
  const { deps, calls, selection } = createDeps();
  assert.deepEqual(
    await switchNevermindBackendEnvironment(
      { environment: 'custom', baseUrl: '  https://PREVIEW.example:443/  ' },
      deps,
    ),
    { ok: true, message: 'Connected to https://preview.example' },
  );
  assert.deepEqual(selection(), {
    environment: 'custom',
    baseUrl: 'https://preview.example',
  });
  assert.deepEqual(calls, [
    'invalidate:https://preview.example',
    'check:https://preview.example',
    'select:https://preview.example',
    'save',
    'auth-origin:https://preview.example',
    `invalidate:${PRODUCTION_NEVERMIND_BASE_URL}`,
    'get-auth:https://preview.example',
    'active:https://preview.example',
    'auth-origin:https://preview.example',
    'warm:https://preview.example',
    'dispose-ai',
    'invalidate-extensions',
    'broadcast:preview@example.com',
  ]);
});

test('uses the production origin and rolls back selection when sign-in fails', async () => {
  const { deps, calls, selection } = createDeps({
    getAuth: async () => null,
    signIn: async () => ({ ok: false, error: 'cancelled' }),
  });
  const result = await switchNevermindBackendEnvironment(
    {
      environment: 'production',
      baseUrl: 'https://ignored.example',
    },
    deps,
  );
  assert.deepEqual(result, { ok: false, message: 'Sign-in failed: cancelled' });
  assert.deepEqual(selection(), {
    environment: 'production',
    baseUrl: PRODUCTION_NEVERMIND_BASE_URL,
  });
  assert.equal(
    calls.filter((call) => call === `select:${PRODUCTION_NEVERMIND_BASE_URL}`)
      .length,
    2,
  );
});

import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: {
      getPath: () => '/tmp/nevermind-compatibility-test',
      getVersion: () => '0.13.2',
    },
  },
});

const {
  checkNevermindCompatibility,
  currentNevermindCompatibilityManifest,
  invalidateNevermindCompatibilityCache,
  isNevermindCompatibilityManifest,
} = await import('./nevermind-compatibility');

const validManifest = {
  backend: { environment: 'preview', version: 'abcdef0' },
  api: { currentVersion: 1, supportedVersions: [1] },
  desktop: {
    minimumSupportedVersion: '0.13.0',
    latestVersion: '0.13.2',
    updateUrl: 'https://example.com/update',
  },
  client: { compatible: true, unsupportedReason: null },
  // biome-ignore lint/style/useNamingConvention: Backend feature names use snake_case.
  features: { proxy_streaming: true },
};

test('accepts the compatibility contract and rejects malformed manifests', () => {
  assert.equal(isNevermindCompatibilityManifest(validManifest), true);
  for (const malformed of [
    null,
    {},
    { ...validManifest, backend: {} },
    { ...validManifest, api: { currentVersion: '1', supportedVersions: [1] } },
    { ...validManifest, desktop: { minimumSupportedVersion: '0.13.0' } },
    { ...validManifest, client: { compatible: 'yes' } },
    {
      ...validManifest,
      // biome-ignore lint/style/useNamingConvention: Backend feature names use snake_case.
      features: { proxy_streaming: 'yes' },
    },
  ]) {
    assert.equal(isNevermindCompatibilityManifest(malformed), false);
  }
});

test('coalesces concurrent compatibility refreshes', async () => {
  const originalFetch = globalThis.fetch;
  let fetches = 0;
  globalThis.fetch = () => {
    fetches += 1;
    return Promise.resolve(Response.json(validManifest));
  };
  try {
    await Promise.all([
      checkNevermindCompatibility('https://coalescing.example'),
      checkNevermindCompatibility('https://coalescing.example'),
    ]);
  } finally {
    globalThis.fetch = originalFetch;
  }
  assert.equal(fetches, 1);
});

test('does not restore a manifest fetched before invalidation', async () => {
  const originalFetch = globalThis.fetch;
  let resolveResponse!: (response: Response) => void;
  let markFetchStarted!: () => void;
  const fetchStarted = new Promise<void>((resolve) => {
    markFetchStarted = resolve;
  });
  globalThis.fetch = () => {
    markFetchStarted();
    return new Promise<Response>((resolve) => {
      resolveResponse = resolve;
    });
  };
  const baseUrl = 'https://invalidation.example';
  try {
    const checking = checkNevermindCompatibility(baseUrl);
    await fetchStarted;
    await invalidateNevermindCompatibilityCache(baseUrl);
    resolveResponse(Response.json(validManifest));
    assert.equal(await checking, null);
    assert.equal(currentNevermindCompatibilityManifest(baseUrl), null);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

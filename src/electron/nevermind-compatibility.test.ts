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

const { isNevermindCompatibilityManifest } = await import(
  './nevermind-compatibility'
);

const validManifest = {
  backend: { environment: 'preview', version: 'abcdef0' },
  api: { currentVersion: 1, supportedVersions: [1] },
  desktop: {
    minimumSupportedVersion: '0.13.0',
    latestVersion: '0.13.2',
    updateUrl: 'https://example.com/update',
  },
  client: { compatible: true, unsupportedReason: null },
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
    { ...validManifest, features: { proxy_streaming: 'yes' } },
  ]) {
    assert.equal(isNevermindCompatibilityManifest(malformed), false);
  }
});

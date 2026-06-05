import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import {
  compareVersions,
  compatibilityError,
  compatibilityManifestForRequest,
  desktopClientFromRequest,
  unsupportedClientReason,
} from './compatibility';

afterEach(() => {
  delete process.env.NEVERMIND_MIN_DESKTOP_VERSION;
  delete process.env.NEVERMIND_LATEST_DESKTOP_VERSION;
  delete process.env.NEVERMIND_DESKTOP_UPDATE_URL;
});

test('parses desktop compatibility headers', () => {
  const request = new Request('https://api.nvm.fyi/api/compatibility', {
    headers: {
      'x-nevermind-client': 'desktop',
      'x-nevermind-client-version': '0.6.0',
      'x-nevermind-api-version': '1',
      'x-nevermind-platform': 'darwin',
      'x-nevermind-arch': 'arm64',
    },
  });

  assert.deepEqual(desktopClientFromRequest(request), {
    name: 'desktop',
    version: '0.6.0',
    apiVersion: 1,
    platform: 'darwin',
    arch: 'arm64',
  });
});

test('keeps older clients without headers compatible by default', () => {
  const request = new Request('https://api.nvm.fyi/api/compatibility');
  const manifest = compatibilityManifestForRequest(request);

  assert.equal(manifest.client.compatible, true);
  assert.equal(manifest.client.unsupportedReason, null);
  assert.equal(manifest.api.currentVersion, 1);
  assert.deepEqual(manifest.api.supportedVersions, [1]);
});

test('detects unsupported desktop versions and API versions', () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.6.0';

  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.5.9', apiVersion: 1, platform: 'darwin', arch: 'arm64' }), 'unsupported_desktop_version');
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.6.0', apiVersion: 2, platform: 'darwin', arch: 'arm64' }), 'unsupported_api_version');
});

test('compares semver-like desktop versions', () => {
  assert.equal(compareVersions('v0.6.0', '0.6.0'), 0);
  assert.equal(compareVersions('0.6.1', '0.6.0'), 1);
  assert.equal(compareVersions('0.5.9', '0.6.0'), -1);
});

test('returns a stable unsupported-client error shape', async () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.6.0';
  process.env.NEVERMIND_LATEST_DESKTOP_VERSION = '0.7.0';
  process.env.NEVERMIND_DESKTOP_UPDATE_URL = 'https://example.com/update';

  const response = compatibilityError(new Request('https://api.nvm.fyi/api/v1/active-model', {
    headers: { 'x-request-id': 'req_123' },
  }));
  const body = await response.json() as any;

  assert.equal(response.status, 426);
  assert.equal(response.headers.get('x-request-id'), 'req_123');
  assert.equal(body.error.type, 'unsupported_client');
  assert.equal(body.error.minimum_supported_desktop_version, '0.6.0');
  assert.equal(body.error.latest_desktop_version, '0.7.0');
  assert.equal(body.error.update_url, 'https://example.com/update');
});

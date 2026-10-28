import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { afterEach, test } from 'node:test';
import {
  backendKillSwitchEnabled,
  compatibilityError,
  compatibilityManifestForRequest,
  desktopClientFromRequest,
  unsupportedClientReason,
} from './compatibility';

afterEach(() => {
  delete process.env.NEVERMIND_MIN_DESKTOP_VERSION;
  delete process.env.NEVERMIND_LATEST_DESKTOP_VERSION;
  delete process.env.NEVERMIND_DESKTOP_UPDATE_URL;
  delete process.env.NEVERMIND_KILL_SWITCHES;
  delete process.env.VERCEL_GIT_COMMIT_SHA;
  delete process.env.VERCEL_ENV;
});

function desktopRequestWithVersion(version: string) {
  return new Request('https://api.nvm.fyi/api/compatibility', {
    headers: {
      'x-nevermind-client': 'desktop',
      'x-nevermind-client-version': version,
      'x-nevermind-api-version': '1',
      'x-nevermind-platform': 'linux',
      'x-nevermind-arch': 'x64',
    },
  });
}

function fixture(name: string) {
  return JSON.parse(readFileSync(new URL(`../fixtures/contracts/desktop-v1/${name}.json`, import.meta.url), 'utf8'));
}

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
  assert.deepEqual(manifest.features, {
    active_model_descriptor: true,
    ai_credit_status: true,
    proxy_streaming: true,
    extension_ai_model_roles: true,
  });
});

test('detects unsupported desktop versions and API versions', () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.6.0';

  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.5.9', apiVersion: 1, platform: 'darwin', arch: 'arm64' }), 'unsupported_desktop_version');
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.6.0', apiVersion: 2, platform: 'darwin', arch: 'arm64' }), 'unsupported_api_version');
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.6.0', apiVersion: 0, platform: 'darwin', arch: 'arm64' }), 'unsupported_api_version');
});

test('treats nightly prereleases of the minimum base version as unsupported', () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.7.0';

  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.7.0-nightly.20261028.1', apiVersion: 1, platform: 'linux', arch: 'x64' }), 'unsupported_desktop_version');
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.7.0', apiVersion: 1, platform: 'linux', arch: 'x64' }), null);
});

test('admits nightlies above the minimum and rejects nightlies just below a stable minimum', () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.7.1';

  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.7.1-nightly.20261028.9', apiVersion: 1, platform: 'linux', arch: 'x64' }), 'unsupported_desktop_version');
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.7.2-nightly.20261028.1', apiVersion: 1, platform: 'linux', arch: 'x64' }), null);
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.7.1-nightly.20261029.1', apiVersion: 1, platform: 'linux', arch: 'x64' }), 'unsupported_desktop_version');
});

test('keeps stable desktop clients and their unsupported reasons unchanged', () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.6.0';

  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.6.0', apiVersion: 1, platform: 'darwin', arch: 'arm64' }), null);
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.6.1', apiVersion: 1, platform: 'darwin', arch: 'arm64' }), null);
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.5.9', apiVersion: 1, platform: 'darwin', arch: 'arm64' }), 'unsupported_desktop_version');
});

test('continues to accept legacy requests without desktop identity headers', () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.7.0';
  const manifest = compatibilityManifestForRequest(new Request('https://api.nvm.fyi/api/compatibility'));
  assert.equal(manifest.client.version, null);
  assert.equal(manifest.client.compatible, true);
  assert.equal(manifest.client.unsupportedReason, null);
});

test('reports compatibility for nightly clients in the manifest', () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.7.0';
  const belowMinimum = compatibilityManifestForRequest(desktopRequestWithVersion('0.7.0-nightly.20261028.1'));
  assert.equal(belowMinimum.client.compatible, false);
  assert.equal(belowMinimum.client.unsupportedReason, 'unsupported_desktop_version');
  assert.equal(belowMinimum.desktop.minimumSupportedVersion, '0.7.0');

  const aboveMinimum = compatibilityManifestForRequest(desktopRequestWithVersion('0.7.1-nightly.20261028.1'));
  assert.equal(aboveMinimum.client.compatible, true);
  assert.equal(aboveMinimum.client.unsupportedReason, null);
});

test('keeps legacy results for versions that are not strict semver', () => {
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.6.0';

  assert.equal(unsupportedClientReason({ name: 'desktop', version: 'not-a-version', apiVersion: 1, platform: 'darwin', arch: 'arm64' }), 'unsupported_desktop_version');
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.6.0.1', apiVersion: 1, platform: 'darwin', arch: 'arm64' }), null);
  assert.equal(unsupportedClientReason({ name: 'desktop', version: '0.5.9.9', apiVersion: 1, platform: 'darwin', arch: 'arm64' }), 'unsupported_desktop_version');
});

test('evaluates backend kill switches from comma-list and JSON config', () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'ai_proxy,auth_device';
  assert.equal(backendKillSwitchEnabled('ai_proxy'), true);
  assert.equal(backendKillSwitchEnabled('ai_streaming'), false);

  process.env.NEVERMIND_KILL_SWITCHES = JSON.stringify({ ai_streaming: true, ai_proxy: false });
  assert.equal(backendKillSwitchEnabled('ai_proxy'), false);
  assert.equal(backendKillSwitchEnabled('ai_streaming'), true);
});

test('matches the desktop-v1 compatibility manifest fixture', () => {
  process.env.VERCEL_GIT_COMMIT_SHA = 'abcdef1234567890';
  process.env.VERCEL_ENV = 'preview';
  process.env.NEVERMIND_MIN_DESKTOP_VERSION = '0.6.0';
  process.env.NEVERMIND_LATEST_DESKTOP_VERSION = '0.7.0';
  process.env.NEVERMIND_DESKTOP_UPDATE_URL = 'https://example.com/update';
  const request = new Request('https://api.nvm.fyi/api/compatibility', {
    headers: {
      'x-nevermind-client': 'desktop',
      'x-nevermind-client-version': '0.6.1',
      'x-nevermind-api-version': '1',
      'x-nevermind-platform': 'darwin',
      'x-nevermind-arch': 'arm64',
    },
  });

  assert.deepEqual(compatibilityManifestForRequest(request), fixture('compatibility-manifest'));
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
  assert.deepEqual(body, fixture('unsupported-client-error'));
});

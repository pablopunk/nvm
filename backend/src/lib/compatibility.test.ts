import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { afterEach, test } from 'node:test';
import {
  backendKillSwitchEnabled,
  compareVersions,
  compatibilityError,
  compatibilityFeaturesForClient,
  compatibilityManifestForRequest,
  desktopClientFromRequest,
  unsupportedClientReason,
} from './compatibility';

afterEach(() => {
  delete process.env.NEVERMIND_MIN_DESKTOP_VERSION;
  delete process.env.NEVERMIND_LATEST_DESKTOP_VERSION;
  delete process.env.NEVERMIND_DESKTOP_UPDATE_URL;
  delete process.env.NEVERMIND_FEATURE_FLAGS;
  delete process.env.NEVERMIND_KILL_SWITCHES;
  delete process.env.VERCEL_GIT_COMMIT_SHA;
  delete process.env.VERCEL_ENV;
});

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

test('returns comma-list feature flags in the manifest', () => {
  process.env.NEVERMIND_FEATURE_FLAGS = 'new_models,streaming_v2';
  const request = new Request('https://api.nvm.fyi/api/compatibility', {
    headers: { 'x-nevermind-client-version': '0.6.0' },
  });

  const manifest = compatibilityManifestForRequest(request, { requestId: 'req_flags' });

  assert.deepEqual(manifest.features, { active_model_descriptor: true, proxy_streaming: true, new_models: true, streaming_v2: true });
});

test('evaluates version, user, plan, and rollout feature rules', () => {
  const client = { name: 'desktop', version: '0.6.0', apiVersion: 1, platform: 'darwin', arch: 'arm64' };
  process.env.NEVERMIND_FEATURE_FLAGS = JSON.stringify({
    enabled: true,
    needs_newer_desktop: { minDesktopVersion: '0.7.0' },
    allowed_user_plan: { users: ['user_1'], plans: ['pro'] },
    blocked_user_plan: { users: ['user_2'], plans: ['pro'] },
    zero_rollout: { rolloutPercent: 0 },
    full_rollout: { rolloutPercent: 100 },
  });

  assert.deepEqual(compatibilityFeaturesForClient(client, { userId: 'user_1', plan: 'pro' }), {
    active_model_descriptor: true,
    proxy_streaming: true,
    enabled: true,
    needs_newer_desktop: false,
    allowed_user_plan: true,
    blocked_user_plan: false,
    zero_rollout: false,
    full_rollout: true,
  });
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
  process.env.NEVERMIND_FEATURE_FLAGS = 'streaming_v2';

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

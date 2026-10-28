import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import {
  CATALOG_BODY,
  installUpstreamStub,
  KEY_BODY,
  OPENCODE_MODELS_URL,
  OPENROUTER_KEY_URL,
  OPENROUTER_MODELS_URL,
  useUpstreamTestEnv,
} from '../test-support/upstream-stub';
import { PROVIDER_CATALOG_MAX_BYTES, probeProviderReadiness } from './upstream-probe';

const ANTHROPIC_MODELS_URL = 'https://anthropic.test/v1/models';
const GOOGLE_MODELS_URL = 'https://google.test/v1beta/models';

let restoreEnv: () => void;

beforeEach(() => {
  restoreEnv = useUpstreamTestEnv();
  process.env.ANTHROPIC_BASE_URL = 'https://anthropic.test';
  process.env.ANTHROPIC_API_KEY = 'test-anthropic-key';
  process.env.GOOGLE_BASE_URL = 'https://google.test/v1beta';
  process.env.GOOGLE_API_KEY = 'test-google-key';
});

afterEach(() => {
  restoreEnv();
});

test('openrouter is ready only when its catalog and its authenticated key check both answer 2xx', async () => {
  installUpstreamStub({
    [OPENROUTER_MODELS_URL]: { status: 200, body: CATALOG_BODY },
    [OPENROUTER_KEY_URL]: { status: 200, body: KEY_BODY },
  });
  assert.deepEqual(await probeProviderReadiness('openrouter'), {
    provider: 'openrouter',
    reachable: true,
    ready: true,
  });
});

test('an anonymous openrouter catalog does not prove the key: a rejected key stays reachable but not ready', async () => {
  const requested = installUpstreamStub({
    [OPENROUTER_MODELS_URL]: { status: 200, body: CATALOG_BODY },
    [OPENROUTER_KEY_URL]: { status: 401, body: '{"error":"invalid"}' },
  });
  assert.deepEqual(await probeProviderReadiness('openrouter'), {
    provider: 'openrouter',
    reachable: true,
    ready: false,
  });
  assert.ok(requested.includes(OPENROUTER_KEY_URL));
});

test('a 5xx from the openrouter key check is unreachable, so liveness fails closed', async () => {
  installUpstreamStub({
    [OPENROUTER_MODELS_URL]: { status: 200, body: CATALOG_BODY },
    [OPENROUTER_KEY_URL]: { status: 503, body: '{}' },
  });
  assert.deepEqual(await probeProviderReadiness('openrouter'), {
    provider: 'openrouter',
    reachable: false,
    ready: false,
  });
});

test('a timed-out key check is unreachable and not ready', async () => {
  installUpstreamStub({
    [OPENROUTER_MODELS_URL]: { status: 200, body: CATALOG_BODY },
    [OPENROUTER_KEY_URL]: 'timeout',
  });
  assert.deepEqual(await probeProviderReadiness('openrouter'), {
    provider: 'openrouter',
    reachable: false,
    ready: false,
  });
});

for (const status of [401, 403, 404, 429]) {
  test(`released liveness holds for an anthropic catalog answering ${status}, which is never ready`, async () => {
    installUpstreamStub({ [ANTHROPIC_MODELS_URL]: { status, body: '{"error":"rejected"}' } });
    assert.deepEqual(await probeProviderReadiness('anthropic'), {
      provider: 'anthropic',
      reachable: true,
      ready: false,
    });
  });
}

test('a 2xx catalog without a data array is reachable but not ready', async () => {
  installUpstreamStub({
    [OPENROUTER_MODELS_URL]: { status: 200, body: '{"models":[]}' },
    [OPENROUTER_KEY_URL]: { status: 200, body: KEY_BODY },
  });
  assert.deepEqual(await probeProviderReadiness('openrouter'), {
    provider: 'openrouter',
    reachable: true,
    ready: false,
  });
});

test('a 2xx html page is reachable but not a model catalog', async () => {
  installUpstreamStub({
    [OPENROUTER_MODELS_URL]: { status: 200, contentType: 'text/html', body: '<!doctype html><html></html>' },
    [OPENROUTER_KEY_URL]: { status: 200, body: KEY_BODY },
  });
  assert.equal((await probeProviderReadiness('openrouter')).ready, false);
});

test('google is ready from its models list, which is not a data array', async () => {
  installUpstreamStub({
    [GOOGLE_MODELS_URL]: { status: 200, body: JSON.stringify({ models: [{ name: 'models/example' }] }) },
  });
  assert.deepEqual(await probeProviderReadiness('google'), {
    provider: 'google',
    reachable: true,
    ready: true,
  });
});

test('google with only a data array is not a Google catalog and is not ready', async () => {
  installUpstreamStub({
    [GOOGLE_MODELS_URL]: { status: 200, body: JSON.stringify({ data: [{ id: 'model-a' }] }) },
  });
  assert.equal((await probeProviderReadiness('google')).ready, false);
});

test('opencode is ready on a 2xx catalog because the configured key was sent and invalid keys get 401', async () => {
  process.env.OPENCODE_API_KEY = 'test-opencode-key';
  const requested = installUpstreamStub({ [OPENCODE_MODELS_URL]: { status: 200, body: CATALOG_BODY } });
  assert.deepEqual(await probeProviderReadiness('opencode_zen'), {
    provider: 'opencode_zen',
    reachable: true,
    ready: true,
  });
  assert.deepEqual(requested, [OPENCODE_MODELS_URL]);
});

test('a catalog larger than the byte bound is treated as not a catalog rather than read in full', async () => {
  installUpstreamStub({
    [OPENROUTER_MODELS_URL]: {
      status: 200,
      body: JSON.stringify({ data: ['x'.repeat(PROVIDER_CATALOG_MAX_BYTES)] }),
    },
    [OPENROUTER_KEY_URL]: { status: 200, body: KEY_BODY },
  });
  assert.deepEqual(await probeProviderReadiness('openrouter'), {
    provider: 'openrouter',
    reachable: true,
    ready: false,
  });
});

test('a provider without a configured key is unreachable and never probed', async () => {
  delete process.env.OPENCODE_API_KEY;
  const requested = installUpstreamStub({});
  assert.deepEqual(await probeProviderReadiness('opencode_zen'), {
    provider: 'opencode_zen',
    reachable: false,
    ready: false,
  });
  assert.deepEqual(requested, []);
});

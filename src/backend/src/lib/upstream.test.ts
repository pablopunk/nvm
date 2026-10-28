import assert from 'node:assert/strict';
import { test, describe, afterEach } from 'node:test';
import { resetPricingCacheForTests } from './pricing';
import { getUpstreamConfig, getUpstreamProbeSpec, selectApiForModel, selectApiForModelChain, providerSupportsFormat, UpstreamConfigError } from './upstream';

const originalFetch = globalThis.fetch;

function mockCatalog(json: unknown) {
  globalThis.fetch = (async () => Response.json(json)) as typeof globalThis.fetch;
}

afterEach(() => {
  globalThis.fetch = originalFetch;
  resetPricingCacheForTests();
  delete process.env.OPENCODE_API_KEY;
  delete process.env.OPENROUTER_API_KEY;
  delete process.env.ANTHROPIC_API_KEY;
  delete process.env.OPENAI_API_KEY;
  delete process.env.GOOGLE_API_KEY;
});

describe('selectApiForModel', () => {
  test('reports the catalog API even when the provider cannot serve it, so failover skips it', async () => {
    mockCatalog({ openrouter: { models: { 'vendor/model': { id: 'vendor/model', provider: { npm: '@ai-sdk/anthropic' } } } } });
    assert.equal(await selectApiForModel('openrouter', 'vendor/model'), 'anthropic-messages');
    assert.equal(providerSupportsFormat('openrouter', 'anthropic-messages'), false);
  });

  test('unknown models default to Chat Completions', async () => {
    mockCatalog({});
    assert.equal(await selectApiForModel('opencode_zen', 'some-unknown-model'), 'openai-completions');
  });

  test('fallback chains use the primary route API only for the primary model', async () => {
    mockCatalog({ openai: { npm: '@ai-sdk/openai', models: { model: { id: 'model' } } } });
    assert.equal(await selectApiForModelChain({ provider: 'openai', modelId: 'model' }, 'model'), 'openai-responses');
    assert.equal(await selectApiForModelChain(null, 'model'), 'openai-completions');
  });
});

describe('providerSupportsFormat', () => {
  test('opencode_zen supports all three formats', () => {
    assert.strictEqual(providerSupportsFormat('opencode_zen', 'openai-completions'), true);
    assert.strictEqual(providerSupportsFormat('opencode_zen', 'anthropic-messages'), true);
    assert.strictEqual(providerSupportsFormat('opencode_zen', 'google-generative-ai'), true);
  });

  test('openrouter supports OpenAI formats', () => {
    assert.strictEqual(providerSupportsFormat('openrouter', 'openai-completions'), true);
    assert.strictEqual(providerSupportsFormat('openrouter', 'openai-responses'), true);
    assert.strictEqual(providerSupportsFormat('openrouter', 'anthropic-messages'), false);
    assert.strictEqual(providerSupportsFormat('openrouter', 'google-generative-ai'), false);
  });

  test('unknown provider returns false', () => {
    assert.strictEqual(providerSupportsFormat('made-up-provider', 'openai-completions'), false);
  });
});

describe('getUpstreamConfig', () => {
  test('returns baseUrl and apiKey for a known provider', () => {
    process.env.OPENCODE_API_KEY = 'test-key';
    const config = getUpstreamConfig('opencode_zen');
    assert.strictEqual(config.apiKey, 'test-key');
    assert.strictEqual(config.baseUrl, 'https://opencode.ai/zen/v1');
  });

  test('uses custom baseUrl from env', () => {
    process.env.OPENCODE_API_KEY = 'test-key';
    process.env.OPENCODE_BASE_URL = 'https://custom.example.com/v1/';
    const config = getUpstreamConfig('opencode_zen');
    assert.strictEqual(config.baseUrl, 'https://custom.example.com/v1');
    delete process.env.OPENCODE_BASE_URL;
  });

  test('strips trailing slash from base URL', () => {
    process.env.ANTHROPIC_API_KEY = 'anthro-key';
    process.env.ANTHROPIC_BASE_URL = 'https://api.anthropic.com/';
    const config = getUpstreamConfig('anthropic');
    assert.strictEqual(config.baseUrl, 'https://api.anthropic.com');
    delete process.env.ANTHROPIC_BASE_URL;
  });

  test('throws UpstreamConfigError for unknown provider', () => {
    assert.throws(
      () => getUpstreamConfig('unknown-provider'),
      UpstreamConfigError,
    );
  });

  test('throws UpstreamConfigError when apiKey env is missing', () => {
    delete process.env.OPENROUTER_API_KEY;
    assert.throws(
      () => getUpstreamConfig('openrouter'),
      UpstreamConfigError,
    );
  });
});

describe('getUpstreamProbeSpec', () => {
  test('openrouter probes an anonymous catalog and verifies the key on /key', () => {
    process.env.OPENROUTER_API_KEY = 'or-key';
    const spec = getUpstreamProbeSpec('openrouter');
    assert.deepStrictEqual(spec.headers, { authorization: 'Bearer or-key' });
    assert.strictEqual(spec.catalogPath, '/models');
    assert.deepStrictEqual(spec.verification, { kind: 'key', path: '/key' });
  });

  test('opencode_zen is verified by its key-gated catalog', () => {
    process.env.OPENCODE_API_KEY = 'zen-key';
    const spec = getUpstreamProbeSpec('opencode_zen');
    assert.deepStrictEqual(spec.verification, { kind: 'catalog' });
    assert.strictEqual(spec.headers.authorization, 'Bearer zen-key');
  });

  test('anthropic uses the x-api-key header, its versioned catalog path, and the required version header', () => {
    process.env.ANTHROPIC_API_KEY = 'anthro-key';
    const spec = getUpstreamProbeSpec('anthropic');
    assert.strictEqual(spec.catalogPath, '/v1/models');
    assert.deepStrictEqual(spec.headers, {
      'anthropic-version': '2023-06-01',
      'x-api-key': 'anthro-key',
    });
    assert.deepStrictEqual(spec.verification, { kind: 'catalog' });
  });

  test('google uses the x-goog-api-key header and lists its catalog under models', () => {
    process.env.GOOGLE_API_KEY = 'google-key';
    const spec = getUpstreamProbeSpec('google');
    assert.deepStrictEqual(spec.headers, { 'x-goog-api-key': 'google-key' });
    assert.strictEqual(spec.catalogListKey, 'models');
  });

  test('every other catalog lists its models under data', () => {
    process.env.OPENROUTER_API_KEY = 'or-key';
    assert.strictEqual(getUpstreamProbeSpec('openrouter').catalogListKey, 'data');
  });

  test('a missing key or unknown provider throws instead of producing an unauthenticated probe', () => {
    delete process.env.OPENAI_API_KEY;
    assert.throws(() => getUpstreamProbeSpec('openai'), UpstreamConfigError);
    assert.throws(() => getUpstreamProbeSpec('made-up-provider'), UpstreamConfigError);
  });
});

import assert from 'node:assert/strict';
import { test, describe, afterEach } from 'node:test';
import { getUpstreamConfig, selectApiForModel, providerSupportsFormat, UpstreamConfigError } from './upstream';

afterEach(() => {
  delete process.env.OPENCODE_API_KEY;
  delete process.env.OPENROUTER_API_KEY;
  delete process.env.ANTHROPIC_API_KEY;
  delete process.env.OPENAI_API_KEY;
  delete process.env.GOOGLE_API_KEY;
});

describe('selectApiForModel', () => {
  test('openrouter provider → openai-completions', () => {
    assert.strictEqual(selectApiForModel('openrouter', 'any-model'), 'openai-completions');
  });

  test('gemini- prefixed models → google-generative-ai', () => {
    assert.strictEqual(selectApiForModel('opencode_zen', 'gemini-3-flash'), 'google-generative-ai');
  });

  test('claude- prefixed models → anthropic-messages', () => {
    assert.strictEqual(selectApiForModel('opencode_zen', 'claude-sonnet-4-6'), 'anthropic-messages');
  });

  test('openrouter provider check wins over model prefix check', () => {
    assert.strictEqual(selectApiForModel('openrouter', 'gemini-flash'), 'openai-completions');
  });

  test('unknown model defaults to openai-completions', () => {
    assert.strictEqual(selectApiForModel('opencode_zen', 'some-unknown-model'), 'openai-completions');
  });
});

describe('providerSupportsFormat', () => {
  test('opencode_zen supports all three formats', () => {
    assert.strictEqual(providerSupportsFormat('opencode_zen', 'openai-completions'), true);
    assert.strictEqual(providerSupportsFormat('opencode_zen', 'anthropic-messages'), true);
    assert.strictEqual(providerSupportsFormat('opencode_zen', 'google-generative-ai'), true);
  });

  test('openrouter supports only openai-completions', () => {
    assert.strictEqual(providerSupportsFormat('openrouter', 'openai-completions'), true);
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

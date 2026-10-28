import assert from 'node:assert/strict';
import { after, afterEach, before, test } from 'node:test';
import {
  listModelsForRouteKind,
  modelFitsRouteKind,
  resetPricingCacheForTests,
} from './pricing';
import { selectApiForModel } from './upstream';

let originalFetch: typeof globalThis.fetch;

function mockCatalog(json: unknown) {
  globalThis.fetch = (async () => ({
    ok: true,
    json: async () => json,
  })) as unknown as typeof globalThis.fetch;
}

const toolAndImageModel = {
  tool_call: true,
  modalities: { input: ['text', 'image'], output: ['text'] },
  limit: { context: 200_000 },
  cost: { input: 1, output: 2 },
};

before(() => {
  originalFetch = globalThis.fetch;
});

afterEach(() => {
  resetPricingCacheForTests();
});

after(() => {
  globalThis.fetch = originalFetch;
});

test('any catalog model with the tier capabilities is selectable without a code change', async () => {
  mockCatalog({
    openrouter: {
      models: {
        'vendor/brand-new-reasoner': { id: 'vendor/brand-new-reasoner', reasoning: true, ...toolAndImageModel },
        'vendor/brand-new-fast': { id: 'vendor/brand-new-fast', reasoning: false, ...toolAndImageModel },
      },
    },
  });

  const smart = await listModelsForRouteKind('openrouter', 'smart');
  const fast = await listModelsForRouteKind('openrouter', 'fast');

  assert.ok(smart.includes('vendor/brand-new-reasoner'));
  assert.ok(fast.includes('vendor/brand-new-reasoner'));
  assert.ok(fast.includes('vendor/brand-new-fast'));
  assert.ok(!smart.includes('vendor/brand-new-fast'));
  assert.equal(await modelFitsRouteKind('openrouter', 'vendor/brand-new-reasoner', 'smart'), true);
  assert.equal(await modelFitsRouteKind('openrouter', 'vendor/brand-new-fast', 'fast'), true);
});

test('models missing a capability every tier uses are not selectable', async () => {
  mockCatalog({
    openrouter: {
      models: {
        'vendor/no-tools': { id: 'vendor/no-tools', reasoning: true, ...toolAndImageModel, tool_call: false },
        'vendor/text-only': { id: 'vendor/text-only', reasoning: true, ...toolAndImageModel, modalities: { input: ['text'], output: ['text'] } },
        'vendor/unpriced': { id: 'vendor/unpriced', reasoning: true, ...toolAndImageModel, cost: undefined },
      },
    },
  });

  for (const kind of ['smart', 'fast'] as const) {
    assert.deepEqual(await listModelsForRouteKind('openrouter', kind), []);
  }
});

test('the upstream API follows the SDK the catalog declares for each model', async () => {
  mockCatalog({
    opencode: {
      npm: '@ai-sdk/openai-compatible',
      models: {
        'new-anthropic-model': { id: 'new-anthropic-model', provider: { npm: '@ai-sdk/anthropic' } },
        'new-google-model': { id: 'new-google-model', provider: { npm: '@ai-sdk/google' } },
        'new-openai-model': { id: 'new-openai-model', provider: { npm: '@ai-sdk/openai' } },
        'new-responses-shape-model': { id: 'new-responses-shape-model', provider: { npm: '@ai-sdk/openai-compatible', shape: 'responses' } },
        'new-compatible-model': { id: 'new-compatible-model' },
      },
    },
    openai: { npm: '@ai-sdk/openai', models: { 'new-direct-model': { id: 'new-direct-model' } } },
    openrouter: { npm: '@openrouter/ai-sdk-provider', models: { 'vendor/new-model': { id: 'vendor/new-model' } } },
  });

  assert.equal(await selectApiForModel('opencode_zen', 'new-anthropic-model'), 'anthropic-messages');
  assert.equal(await selectApiForModel('opencode_zen', 'new-google-model'), 'google-generative-ai');
  assert.equal(await selectApiForModel('opencode_zen', 'new-openai-model'), 'openai-responses');
  assert.equal(await selectApiForModel('opencode_zen', 'new-responses-shape-model'), 'openai-responses');
  assert.equal(await selectApiForModel('opencode_zen', 'new-compatible-model'), 'openai-completions');
  assert.equal(await selectApiForModel('openai', 'new-direct-model'), 'openai-responses');
  assert.equal(await selectApiForModel('openrouter', 'vendor/new-model'), 'openai-completions');
});

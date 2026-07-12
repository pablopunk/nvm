import assert from 'node:assert/strict';
import { after, afterEach, before, test } from 'node:test';
import { lookupModelCost, resetPricingCacheForTests } from './pricing';

let originalFetch: typeof globalThis.fetch;

function mockFetch(json: unknown) {
  return (async () => ({ ok: true, json: async () => json })) as typeof globalThis.fetch;
}

before(() => {
  originalFetch = globalThis.fetch;
});

afterEach(() => {
  resetPricingCacheForTests();
});

after(() => {
  globalThis.fetch = originalFetch;
  resetPricingCacheForTests();
});

test('lookupModelCost resolves provider slug and returns cost', async () => {
  globalThis.fetch = mockFetch({
    opencode: {
      models: {
        'claude-haiku-4-5': { id: 'claude-haiku-4-5', cost: { input: 1, output: 5 } },
      },
    },
  });

  const result = await lookupModelCost('opencode_zen', 'claude-haiku-4-5');
  assert.deepEqual(result, {
    provider: 'opencode_zen',
    modelId: 'claude-haiku-4-5',
    inputUsdPerMtok: 1,
    outputUsdPerMtok: 5,
  });
});

test('lookupModelCost uses provider as-is when no slug mapping exists', async () => {
  globalThis.fetch = mockFetch({
    openrouter: {
      models: {
        'some-model': { id: 'some-model', cost: { input: 0.5, output: 2 } },
      },
    },
  });

  const result = await lookupModelCost('openrouter', 'some-model');
  assert.deepEqual(result, {
    provider: 'openrouter',
    modelId: 'some-model',
    inputUsdPerMtok: 0.5,
    outputUsdPerMtok: 2,
  });
});

test('lookupModelCost returns null for unknown model', async () => {
  globalThis.fetch = mockFetch({ opencode: { models: {} } });

  const result = await lookupModelCost('opencode_zen', 'nonexistent-model');
  assert.strictEqual(result, null);
});

test('lookupModelCost returns null when cost fields are missing', async () => {
  globalThis.fetch = mockFetch({
    opencode: {
      models: { 'no-cost-model': { id: 'no-cost-model', cost: {} } },
    },
  });

  const result = await lookupModelCost('opencode_zen', 'no-cost-model');
  assert.strictEqual(result, null);
});

test('lookupModelCost returns null for unmapped provider', async () => {
  globalThis.fetch = mockFetch({});

  const result = await lookupModelCost('unknown-provider', 'some-model');
  assert.strictEqual(result, null);
});

test('resetPricingCacheForTests clears the internal cache', async () => {
  globalThis.fetch = mockFetch({
    opencode: {
      models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 1 } } },
    },
  });

  const first = await lookupModelCost('opencode_zen', 'model-a');
  assert.ok(first);

  resetPricingCacheForTests();

  globalThis.fetch = mockFetch({
    opencode: {
      models: { 'model-a': { id: 'model-a', cost: { input: 9, output: 9 } } },
    },
  });

  const afterReset = await lookupModelCost('opencode_zen', 'model-a');
  assert.deepEqual(afterReset, {
    provider: 'opencode_zen',
    modelId: 'model-a',
    inputUsdPerMtok: 9,
    outputUsdPerMtok: 9,
  });
});

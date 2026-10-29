import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import {
  IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE,
  imageTokenLimits,
} from './image-cost-bound';
import {
  conservativeImageRequestInputTokenCeiling,
  supportsBoundedImageInput,
} from './limits';
import { resetPricingCacheForTests } from './pricing';
import { refreshPricingSnapshots } from './pricing-refresh';
import {
  openRouterEndpointsBodyFromCost,
  openRouterEndpointsModelId,
} from '../test-support/openrouter-endpoints';

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  resetPricingCacheForTests();
});

test('bounds image billing by the route model input limit', () => {
  assert.equal(IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE, 8_192);
  assert.equal(
    conservativeImageRequestInputTokenCeiling(
      {
        contextWindow: 1_050_000,
        maxInputTokens: 922_000,
        maxTokens: 128_000,
      },
      512,
      8_192,
    ),
    922_000,
  );
  assert.equal(
    conservativeImageRequestInputTokenCeiling(
      { contextWindow: 1_048_576, maxTokens: 131_072 },
      undefined,
      8_192,
    ),
    913_408,
  );
  assert.equal(
    conservativeImageRequestInputTokenCeiling(
      { contextWindow: 16_384, maxInputTokens: 12_000, maxTokens: 4_096 },
      4_096,
      13_000,
    ),
    undefined,
  );
});

test('image billing limits come from the loaded catalog and endpoint price', async () => {
  const catalog = {
      openrouter: {
        models: {
          'vendor/image-model': {
            id: 'vendor/image-model',
            modalities: { input: ['text', 'image'] },
            limit: { context: 1_050_000, input: 922_000, output: 128_000 },
            cost: {
              input: 2,
              output: 10,
              cache_read: 0.1,
              tiers: [{ input: 4, output: 15, tier: { type: 'context', size: 272_000 } }],
            },
          },
          'vendor/text-model': {
            id: 'vendor/text-model',
            modalities: { input: ['text'] },
            limit: { context: 32_000, output: 4_096 },
            cost: { input: 0.1, output: 0.2 },
          },
          'vendor/unlimited-image-model': {
            id: 'vendor/unlimited-image-model',
            modalities: { input: ['text', 'image'] },
            cost: { input: 0.1, output: 0.2 },
          },
        },
      },
  };
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    const endpointsModel = openRouterEndpointsModelId(url);
    if (endpointsModel) {
      const models: Record<string, { cost: Parameters<typeof openRouterEndpointsBodyFromCost>[1] }> =
        catalog.openrouter.models;
      const model = Object.hasOwn(models, endpointsModel) ? models[endpointsModel] : undefined;
      return model
        ? Response.json(openRouterEndpointsBodyFromCost(endpointsModel, model.cost))
        : new Response('{}', { status: 404 });
    }
    return Response.json(catalog);
  }) as typeof globalThis.fetch;
  // Image bounds read the published endpoint price synchronously, so the refresh runs first.
  const report = await refreshPricingSnapshots(['vendor/image-model']);
  assert.equal(report.catalog, 'refreshed');
  assert.deepEqual(report.openRouter.failed, []);

  assert.deepEqual(imageTokenLimits('openrouter', 'vendor/image-model'), {
    contextWindow: 1_050_000,
    maxInputTokens: 922_000,
    maxTokens: 128_000,
    maximumCost: {
      inputUsdPerMtok: 2,
      outputUsdPerMtok: 10,
      cacheReadUsdPerMtok: 0.1,
      // Endpoint cache writes the catalog omits bill as ordinary input, so they inherit the prompt rate.
      cacheWriteUsdPerMtok: 2,
      tiers: [
        {
          thresholdTokens: 272_000,
          inputUsdPerMtok: 4,
          outputUsdPerMtok: 15,
          cacheReadUsdPerMtok: 0.1,
          cacheWriteUsdPerMtok: 4,
        },
      ],
    },
  });
  assert.equal(imageTokenLimits('openrouter', 'vendor/text-model'), undefined);
  assert.equal(imageTokenLimits('openrouter', 'vendor/unlimited-image-model'), undefined);
  assert.equal(
    supportsBoundedImageInput('openrouter', 'vendor/image-model', ['text', 'image']),
    true,
  );
  assert.equal(
    supportsBoundedImageInput('openrouter', 'vendor/image-model', ['text']),
    false,
  );
});

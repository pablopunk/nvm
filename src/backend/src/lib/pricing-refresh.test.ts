import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import {
  openRouterEndpointsBodyFromCost,
  openRouterEndpointsModelId,
} from '../test-support/openrouter-endpoints';
import { lookupModelCost, resetPricingCacheForTests } from './pricing';
import {
  pricingRefreshSucceeded,
  refreshPricingSnapshots,
  refreshRoutePricing,
} from './pricing-refresh';

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  resetPricingCacheForTests();
});

/** models.dev answers empty; each OpenRouter endpoint answers only for a priced model. */
function endpointsFetch(priced: Record<string, { input: number; output: number }>) {
  return (async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') return Response.json({});
    const modelId = openRouterEndpointsModelId(url);
    const cost = modelId ? priced[modelId] : undefined;
    if (!modelId || !cost) return new Response('{}', { status: 404 });
    return Response.json(openRouterEndpointsBodyFromCost(modelId, cost));
  }) as typeof globalThis.fetch;
}

test('one failing OpenRouter endpoint never blocks the other models in the same refresh', async () => {
  globalThis.fetch = endpointsFetch({ 'vendor/ok': { input: 1, output: 2 } });

  const report = await refreshPricingSnapshots(['vendor/ok', 'vendor/missing']);

  assert.equal(report.catalog, 'refreshed');
  assert.deepEqual(report.openRouter, {
    refreshed: ['vendor/ok'],
    failed: ['vendor/missing'],
  });
  assert.ok(await lookupModelCost('openrouter', 'vendor/ok'));
  assert.equal(await lookupModelCost('openrouter', 'vendor/missing'), null);
});

test('a refresh fetches each OpenRouter model once, however often it is listed', async () => {
  const calls: string[] = [];
  const answer = endpointsFetch({ 'vendor/ok': { input: 1, output: 2 } });
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    calls.push(String(input instanceof Request ? input.url : input));
    return answer(input, init);
  }) as typeof globalThis.fetch;

  const report = await refreshPricingSnapshots(['vendor/ok', 'vendor/ok']);

  assert.deepEqual(report.openRouter.refreshed, ['vendor/ok']);
  assert.equal(calls.filter((url) => url.includes('/models/vendor/ok/endpoints')).length, 1);
});

test('the scheduled refresh validates the audio and decision catalogs, and a failing one fails the refresh', async () => {
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url === 'https://models.dev/api.json') return Response.json({});
    if (url.endsWith('/models?output_modalities=decisions')) return Response.json({ data: [] });
    if (url.endsWith('/models?input_modalities=audio&output_modalities=transcription'))
      return new Response('{}', { status: 503 });
    return new Response('{}', { status: 404 });
  }) as typeof globalThis.fetch;

  const report = await refreshPricingSnapshots([]);

  assert.deepEqual(report.openRouterCatalogs, { transcription: 'failed', decision: 'refreshed' });
  assert.equal(pricingRefreshSucceeded(report), false);
  assert.equal(
    pricingRefreshSucceeded({
      ...report,
      openRouterCatalogs: { transcription: 'refreshed', decision: 'refreshed' },
    }),
    true,
  );
});

test('an admin route refresh reports failure instead of throwing, so the route is refused', async () => {
  globalThis.fetch = (async () => {
    throw new Error('network down');
  }) as typeof globalThis.fetch;

  assert.equal(await refreshRoutePricing('openrouter', 'vendor/ok'), false);
  assert.equal(await refreshRoutePricing('opencode_zen', 'gpt-5.6-luna'), false);
});

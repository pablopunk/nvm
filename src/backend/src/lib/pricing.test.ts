import assert from 'node:assert/strict';
import { after, afterEach, before, test } from 'node:test';
import { openRouterEndpointsModelId } from '../test-support/openrouter-endpoints';
import {
  catalogSdkForModel,
  expirePricingCacheForTests,
  listModelsForProvider,
  listModelsForRouteKind,
  loadedImageTokenLimits,
  lookupModelCost,
  lookupModelDescriptor,
  lookupModelPricing,
  modelFitsRouteKind,
  resetPricingCacheForTests,
  reservationMarginFor,
  resolveModelCost,
  simulateColdInstanceForTests,
  STALE_RESERVATION_MARGIN,
} from './pricing';
import { refreshPricingSnapshots } from './pricing-refresh';
import {
  BUNDLED_CATALOG_VERIFIED_AT,
  setBundledClockForTests,
} from './pricing-bundled';
import { setStaleAdmissionsHaltedForTests } from './pricing-overrun-cap';
import {
  SNAPSHOT_CURRENT_MAX_AGE_MS,
  SNAPSHOT_MAX_AGE_MS,
} from './pricing-snapshot-store';

let originalFetch: typeof globalThis.fetch;

type Routes = { catalog?: unknown; endpoints?: Record<string, unknown> };

/** Answers models.dev from `catalog` and each OpenRouter endpoints path from `endpoints`. */
function routedFetch(routes: Routes, calls: string[] = []) {
  return (async (input: string | URL | Request) => {
    const url = String(input instanceof Request ? input.url : input);
    calls.push(url);
    if (url === 'https://models.dev/api.json')
      return { ok: true, json: async () => routes.catalog ?? {} };
    const modelId = openRouterEndpointsModelId(url);
    const body = modelId ? routes.endpoints?.[modelId] : undefined;
    if (body === undefined)
      return { ok: false, status: 404, json: async () => ({}) };
    return { ok: true, json: async () => body };
  }) as unknown as typeof globalThis.fetch;
}

/**
 * Runs the scheduled refresh against `routes` and asserts it published everything.
 * Lookups read only what this wrote, so a test that wants a price publishes it first.
 */
async function publishPricing(routes: Routes, calls: string[] = []) {
  globalThis.fetch = routedFetch(routes, calls);
  const report = await refreshPricingSnapshots(Object.keys(routes.endpoints ?? {}));
  assert.equal(report.catalog, 'refreshed');
  assert.deepEqual(report.openRouter.failed, []);
  return report;
}

function failingFetch(calls: string[] = []) {
  return (async (input: string | URL | Request) => {
    calls.push(String(input instanceof Request ? input.url : input));
    throw new Error('network down');
  }) as unknown as typeof globalThis.fetch;
}

function endpointListing(modelId: string, pricings: Array<Record<string, unknown>>) {
  return {
    data: {
      id: modelId,
      name: modelId,
      endpoints: pricings.map((pricing, index) => ({
        provider_name: `Provider ${index}`,
        status: 0,
        pricing,
      })),
    },
  };
}

before(() => {
  originalFetch = globalThis.fetch;
  // Snapshots must persist in memory for the restore tests; the Postgres store is not reachable here.
  resetPricingCacheForTests();
});

afterEach(() => {
  resetPricingCacheForTests();
});

after(() => {
  globalThis.fetch = originalFetch;
  resetPricingCacheForTests();
});

test('lookupModelCost resolves provider slug and returns cost', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: {
          'claude-haiku-4-5': {
            id: 'claude-haiku-4-5',
            cost: { input: 1, output: 5 },
          },
        },
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
  await publishPricing({
    catalog: {
      anthropic: {
        models: {
          'some-model': { id: 'some-model', cost: { input: 0.5, output: 2 } },
        },
      },
    },
  });

  const result = await lookupModelCost('anthropic', 'some-model');
  assert.deepEqual(result, {
    provider: 'anthropic',
    modelId: 'some-model',
    inputUsdPerMtok: 0.5,
    outputUsdPerMtok: 2,
  });
});

test('lookupModelCost returns null for unknown model', async () => {
  await publishPricing({ catalog: { opencode: { models: {} } } });

  const result = await lookupModelCost('opencode_zen', 'nonexistent-model');
  assert.strictEqual(result, null);
});

test('lookupModelCost returns null when cost fields are missing', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: { 'no-cost-model': { id: 'no-cost-model', cost: {} } },
      },
    },
  });

  const result = await lookupModelCost('opencode_zen', 'no-cost-model');
  assert.strictEqual(result, null);
});

test('lookupModelCost fails closed on invalid base and tier rates', async () => {
  await publishPricing({
    catalog: {
      anthropic: {
        models: {
          'negative-base': {
            id: 'negative-base',
            cost: { input: -1, output: 1 },
          },
          'negative-tier': {
            id: 'negative-tier',
            cost: {
              input: 1,
              output: 1,
              tiers: [{ input: -2, tier: { size: 100 } }],
            },
          },
        },
      },
    },
  });
  assert.equal(await lookupModelCost('anthropic', 'negative-base'), null);
  assert.equal(await lookupModelCost('anthropic', 'negative-tier'), null);
});

test('lookupModelCost returns null for unmapped provider', async () => {
  await publishPricing({ catalog: {} });

  const result = await lookupModelCost('unknown-provider', 'some-model');
  assert.strictEqual(result, null);
});

test('lookupModelPricing preserves source cache and tier prices', async () => {
  await publishPricing({
    catalog: {
      anthropic: {
        models: {
          'openai/test': {
            id: 'openai/test',
            cost: {
              input: 2,
              output: 10,
              cache_read: 0.2,
              cache_write: 2.5,
              tiers: [
                {
                  input: 4,
                  output: 15,
                  tier: { type: 'context', size: 272_000 },
                },
              ],
            },
          },
        },
      },
    },
  });

  assert.deepEqual(await lookupModelPricing('anthropic', 'openai/test'), {
    provider: 'anthropic',
    modelId: 'openai/test',
    inputUsdPerMtok: 2,
    outputUsdPerMtok: 10,
    cacheReadUsdPerMtok: 0.2,
    cacheWriteUsdPerMtok: 2.5,
    tiers: [
      {
        thresholdTokens: 272_000,
        inputUsdPerMtok: 4,
        outputUsdPerMtok: 15,
        cacheReadUsdPerMtok: null,
        cacheWriteUsdPerMtok: null,
      },
    ],
    source: 'models.dev',
  });
});

test('admin model listing includes bundled runtime models', async () => {
  await publishPricing({ catalog: { openrouter: { models: {} } } });
  const models = await listModelsForProvider('openrouter');
  assert.ok(models.includes('deepseek/deepseek-v4-flash-0731'));
});

test('route choices require function tools', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: {
          'no-tools': {
            id: 'no-tools',
            reasoning: true,
            tool_call: false,
            modalities: { input: ['text', 'image'], output: ['text'] },
            limit: { context: 100_000 },
            cost: { input: 1, output: 2 },
          },
          'unknown-tools': {
            id: 'unknown-tools',
            reasoning: true,
            modalities: { input: ['text', 'image'], output: ['text'] },
            limit: { context: 100_000 },
            cost: { input: 1, output: 2 },
          },
        },
      },
    },
  });

  for (const kind of ['smart', 'fast'] as const) {
    const choices = await listModelsForRouteKind('opencode_zen', kind);
    for (const model of ['no-tools', 'unknown-tools']) {
      assert.ok(!choices.includes(model));
      assert.equal(await modelFitsRouteKind('opencode_zen', model, kind), false);
    }
  }
});

test('route choices come from catalog capabilities, images, tools, pricing and context', async () => {
  await publishPricing({
    catalog: {
      openrouter: { models: {
        'vendor/reasoner': {
          id: 'vendor/reasoner', tool_call: true, reasoning: true,
          modalities: { input: ['text', 'image'], output: ['text'] },
          limit: { context: 100_000 }, cost: { input: 1, output: 2 },
        },
        'vendor/non-reasoner': {
          id: 'vendor/non-reasoner', tool_call: true, reasoning: false,
          modalities: { input: ['text', 'image'], output: ['text'] },
          limit: { context: 100_000 }, cost: { input: 1, output: 2 },
        },
        'vendor/text-only': {
          id: 'vendor/text-only', tool_call: true, reasoning: true,
          modalities: { input: ['text'], output: ['text'] },
          limit: { context: 100_000 }, cost: { input: 1, output: 2 },
        },
        'vendor/no-context': {
          id: 'vendor/no-context', tool_call: true, reasoning: true,
          modalities: { input: ['text', 'image'], output: ['text'] },
          cost: { input: 1, output: 2 },
        },
        'vendor/unpriced': {
          id: 'vendor/unpriced', tool_call: true, reasoning: true,
          modalities: { input: ['text', 'image'], output: ['text'] },
          limit: { context: 100_000 },
        },
        'vendor/spoofed': {
          id: 'vendor/reasoner', tool_call: true, reasoning: true,
          modalities: { input: ['text', 'image'], output: ['text'] },
          limit: { context: 100_000 }, cost: { input: 1, output: 2 },
        },
      } },
    },
  });
  const smart = await listModelsForRouteKind('openrouter', 'smart');
  const fast = await listModelsForRouteKind('openrouter', 'fast');
  assert.ok(smart.includes('vendor/reasoner'));
  assert.ok(!smart.includes('vendor/non-reasoner'));
  assert.ok(fast.includes('vendor/reasoner'));
  assert.ok(fast.includes('vendor/non-reasoner'));
  for (const model of ['vendor/text-only', 'vendor/no-context', 'vendor/unpriced', 'vendor/spoofed']) {
    assert.equal(await modelFitsRouteKind('openrouter', model, 'smart'), false);
    assert.equal(await modelFitsRouteKind('openrouter', model, 'fast'), false);
  }
});

test('route listing and validation use the same current catalog capabilities', async () => {
  await publishPricing({
    catalog: {
      openrouter: {
        models: {
          'google/gemini-2.5-flash': {
            id: 'google/gemini-2.5-flash',
            reasoning: true,
            tool_call: false,
            modalities: { input: ['text', 'image'], output: ['text'] },
            cost: { input: 1, output: 2 },
          },
        },
      },
    },
  });
  for (const kind of ['smart', 'fast'] as const) {
    assert.ok(!(await listModelsForRouteKind('openrouter', kind)).includes('google/gemini-2.5-flash'));
    assert.equal(await modelFitsRouteKind('openrouter', 'google/gemini-2.5-flash', kind), false);
  }
});

test('a model is rejected when tool metadata is false or absent', async () => {
  for (const toolCall of [false, undefined]) {
    resetPricingCacheForTests();
    await publishPricing({
      catalog: {
        openrouter: { models: { 'google/gemini-2.5-flash': {
          id: 'google/gemini-2.5-flash', reasoning: true, tool_call: toolCall,
          modalities: { input: ['text', 'image'], output: ['text'] },
          limit: { context: 100_000 }, cost: { input: 1, output: 2 },
        } } },
      },
    });
    for (const kind of ['smart', 'fast'] as const) {
      assert.ok(!(await listModelsForRouteKind('openrouter', kind)).includes('google/gemini-2.5-flash'));
      assert.equal(await modelFitsRouteKind('openrouter', 'google/gemini-2.5-flash', kind), false);
    }
  }
});

test('active runtime models do not wait for the remote catalog', async () => {
  globalThis.fetch = (() => new Promise(() => {})) as typeof globalThis.fetch;

  const cost = await lookupModelCost('opencode_zen', 'gpt-5.6-luna');
  const descriptor = await lookupModelDescriptor(
    'opencode_zen',
    'deepseek-v4-flash',
  );
  const openRouterDescriptor = await lookupModelDescriptor(
    'openrouter',
    'google/gemini-2.5-flash',
  );
  const deepSeekFlashDescriptor = await lookupModelDescriptor(
    'openrouter',
    'deepseek/deepseek-v4-flash-0731',
  );

  assert.deepEqual(cost, {
    provider: 'opencode_zen',
    modelId: 'gpt-5.6-luna',
    inputUsdPerMtok: 0.2,
    outputUsdPerMtok: 1.2,
    tiers: [{ thresholdTokens: 272_000, inputUsdPerMtok: 0.4, outputUsdPerMtok: 1.8 }],
  });
  assert.equal(descriptor?.name, 'DeepSeek V4 Flash');
  assert.equal(descriptor?.contextWindow, 1_000_000);
  assert.equal(openRouterDescriptor?.name, 'Gemini 2.5 Flash');
  assert.equal(openRouterDescriptor?.contextWindow, 1_048_576);
  assert.deepEqual(openRouterDescriptor?.input, [
    'text',
    'image',
    'audio',
    'video',
    'pdf',
  ]);
  assert.equal(deepSeekFlashDescriptor?.name, 'DeepSeek V4 Flash 0731');
  assert.equal(deepSeekFlashDescriptor?.contextWindow, 1_048_576);
  assert.equal(deepSeekFlashDescriptor?.maxTokens, 943_718);
});

test('request-path lookups never reach the network, even with no snapshot', async () => {
  const calls: string[] = [];
  globalThis.fetch = failingFetch(calls);

  assert.equal(await lookupModelCost('openrouter', 'vendor/never-published'), null);
  assert.equal(await lookupModelCost('opencode_zen', 'never-published'), null);
  await listModelsForProvider('opencode_zen');
  assert.deepEqual(calls, []);
});

test('model descriptors retain catalog max-input limits for safe image reservations', async () => {
  await publishPricing({
    catalog: {
      openrouter: {
        models: {
          'openai/gpt-6.1-sol': {
            id: 'openai/gpt-6.1-sol',
            modalities: { input: ['text', 'image'] },
            limit: { context: 1_050_000, input: 922_000, output: 128_000 },
            cost: { input: 2, output: 10 },
          },
        },
      },
    },
  });

  const descriptor = await lookupModelDescriptor(
    'openrouter',
    'openai/gpt-6.1-sol',
  );

  assert.equal(descriptor?.contextWindow, 1_050_000);
  assert.equal(descriptor?.maxInputTokens, 922_000);
  assert.equal(descriptor?.maxTokens, 128_000);
  assert.deepEqual(descriptor?.input, ['text', 'image']);
});

test('bundled pricing is usable only for the exact bundled provider/model entry', async () => {
  await publishPricing({ catalog: { opencode: { models: {} } } });
  const bundled = await lookupModelPricing('opencode_zen', 'gpt-5.6-luna');
  assert.equal(bundled?.source, 'bundled');
  assert.equal(await lookupModelCost('opencode_zen', 'not-bundled'), null);
});

test('a failed refresh publishes nothing, so unverified catalog rates are not admitted', async () => {
  globalThis.fetch = (async () => ({
    ok: false,
    status: 503,
  })) as unknown as typeof globalThis.fetch;
  const report = await refreshPricingSnapshots([]);

  assert.equal(report.catalog, 'failed');
  assert.equal(await lookupModelCost('opencode_zen', 'claude-haiku-4-5'), null);
  assert.deepEqual(await resolveModelCost('opencode_zen', 'claude-haiku-4-5'), {
    ok: false,
    reason: 'unverified',
  });
});

test('a failed refresh keeps the last validated price serving until the hard bound', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: { 'stale-model': { id: 'stale-model', cost: { input: 1, output: 2 } } },
      },
    },
  });
  assert.ok(await lookupModelCost('opencode_zen', 'stale-model'));

  const calls: string[] = [];
  globalThis.fetch = failingFetch(calls);
  assert.equal((await refreshPricingSnapshots([])).catalog, 'failed');
  const refreshCalls = calls.length;
  assert.ok(refreshCalls > 0, 'the scheduled refresh is the only path that fetches');
  assert.ok(await lookupModelCost('opencode_zen', 'stale-model'));
  assert.equal(calls.length, refreshCalls, 'lookups never reach the network');

  expirePricingCacheForTests();
  assert.deepEqual(await resolveModelCost('opencode_zen', 'stale-model'), {
    ok: false,
    reason: 'stale',
  });
  assert.equal(calls.length, refreshCalls, 'expired lookups do not fetch either');
});

test('the overrun cap refuses a stale price only while it is reached, and never refuses a current one', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } },
      },
    },
  });
  try {
    setStaleAdmissionsHaltedForTests(true);
    assert.equal((await resolveModelCost('opencode_zen', 'model-a')).ok, true, 'a current price admits under the cap');

    expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
    assert.deepEqual(await resolveModelCost('opencode_zen', 'model-a'), { ok: false, reason: 'stale' });
    assert.equal(await lookupModelCost('opencode_zen', 'model-a'), null);
    assert.equal(await lookupModelPricing('opencode_zen', 'model-a'), null);

    setStaleAdmissionsHaltedForTests(false);
    const admitted = await resolveModelCost('opencode_zen', 'model-a');
    assert.equal(admitted.ok && admitted.cost.stale, true, 'below the cap the stale price admits at the stale margin');
  } finally {
    setStaleAdmissionsHaltedForTests(null);
  }
});

test('a validation older than the current window is admitted for paid pricing and marked stale', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } },
      },
    },
  });
  expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);

  assert.deepEqual(await lookupModelCost('opencode_zen', 'model-a'), {
    provider: 'opencode_zen',
    modelId: 'model-a',
    inputUsdPerMtok: 1,
    outputUsdPerMtok: 2,
    stale: true,
  });
  assert.equal(
    (await lookupModelPricing('opencode_zen', 'model-a'))?.stale,
    true,
  );
});

test('a stale price reserves at the stale margin, and a current price reserves at cost', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } },
      },
    },
  });
  const current = await lookupModelCost('opencode_zen', 'model-a');
  assert.equal(reservationMarginFor(current ?? {}), 1);

  expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
  const stale = await lookupModelCost('opencode_zen', 'model-a');
  assert.equal(reservationMarginFor(stale ?? {}), STALE_RESERVATION_MARGIN);
  assert.ok(STALE_RESERVATION_MARGIN > 1);
});

test('a validation past the hard bound is refused for every tier', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } },
      },
    },
  });
  expirePricingCacheForTests(SNAPSHOT_MAX_AGE_MS + 1);

  assert.deepEqual(await resolveModelCost('opencode_zen', 'model-a'), {
    ok: false,
    reason: 'stale',
  });
});

test('resetPricingCacheForTests clears the internal cache', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 1 } } },
      },
    },
  });

  const first = await lookupModelCost('opencode_zen', 'model-a');
  assert.ok(first);

  resetPricingCacheForTests();

  await publishPricing({
    catalog: {
      opencode: {
        models: { 'model-a': { id: 'model-a', cost: { input: 9, output: 9 } } },
      },
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

test('an invalid catalog shape is rejected and the validated section still serves until it ages out', async () => {
  await publishPricing({
    catalog: {
      opencode: { models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } } },
    },
  });
  assert.ok(await lookupModelCost('opencode_zen', 'model-a'));

  globalThis.fetch = routedFetch({ catalog: { opencode: null } });
  assert.equal((await refreshPricingSnapshots([])).catalog, 'failed');
  assert.ok(await lookupModelCost('opencode_zen', 'model-a'));

  expirePricingCacheForTests();
  assert.deepEqual(await resolveModelCost('opencode_zen', 'model-a'), {
    ok: false,
    reason: 'stale',
  });
});

test('a partial refresh keeps a provider section it dropped, bounded by its own last validation', async () => {
  await publishPricing({
    catalog: {
      opencode: { models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } } },
      anthropic: { models: { 'vendor/model-b': { id: 'vendor/model-b', cost: { input: 3, output: 4 } } } },
    },
  });
  assert.ok(await lookupModelCost('anthropic', 'vendor/model-b'));

  expirePricingCacheForTests();
  await publishPricing({
    catalog: {
      opencode: { models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } } },
    },
  });

  assert.ok(await lookupModelCost('opencode_zen', 'model-a'));
  assert.deepEqual(await resolveModelCost('anthropic', 'vendor/model-b'), {
    ok: false,
    reason: 'stale',
  });
});

test('resolveModelCost names why a model is unavailable', async () => {
  await publishPricing({ catalog: { opencode: { models: {} } } });
  assert.deepEqual(await resolveModelCost('opencode_zen', 'missing-model'), {
    ok: false,
    reason: 'unpriced',
  });

  resetPricingCacheForTests();
  globalThis.fetch = (async () => ({ ok: false, status: 503 })) as unknown as typeof globalThis.fetch;
  await refreshPricingSnapshots([]);
  assert.deepEqual(await resolveModelCost('openrouter', 'missing-model'), {
    ok: false,
    reason: 'unverified',
  });
});

test('undeclared providers fail closed without reading any catalog', async () => {
  let calls = 0;
  globalThis.fetch = (async () => {
    calls += 1;
    return { ok: true, json: async () => ({}) };
  }) as unknown as typeof globalThis.fetch;

  assert.deepEqual(await resolveModelCost('undeclared-provider', 'some-model'), {
    ok: false,
    reason: 'unverified',
  });
  assert.equal(calls, 0);
});

test('a current catalog entry wins over the bundled entry it shadows', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: {
          'gpt-5.6-luna': { id: 'gpt-5.6-luna', cost: { input: 9, output: 9 } },
        },
      },
    },
  });

  assert.deepEqual(await lookupModelPricing('opencode_zen', 'gpt-5.6-luna'), {
    provider: 'opencode_zen',
    modelId: 'gpt-5.6-luna',
    inputUsdPerMtok: 9,
    outputUsdPerMtok: 9,
    cacheReadUsdPerMtok: null,
    cacheWriteUsdPerMtok: null,
    tiers: [],
    source: 'models.dev',
  });
});

test('a stale catalog entry never overrides the bundled entry that prices the same model', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: {
          'gpt-5.6-luna': {
            id: 'gpt-5.6-luna',
            name: 'Catalog Luna',
            cost: { input: 9, output: 9 },
          },
        },
      },
    },
  });
  expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);

  const pricing = await lookupModelPricing('opencode_zen', 'gpt-5.6-luna');
  const descriptor = await lookupModelDescriptor('opencode_zen', 'gpt-5.6-luna');
  assert.equal(pricing?.source, 'bundled');
  assert.equal(pricing?.inputUsdPerMtok, 0.2);
  assert.equal(descriptor?.name, 'GPT-5.6 Luna');
  assert.ok((await listModelsForProvider('opencode_zen')).includes('gpt-5.6-luna'));
});

test('listing shows catalog entries inside the hard bound and drops them past it', async () => {
  await publishPricing({
    catalog: {
      opencode: {
        models: { 'model-x': { id: 'model-x', cost: { input: 1, output: 2 } } },
      },
    },
  });
  expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
  assert.ok((await listModelsForProvider('opencode_zen')).includes('model-x'));

  expirePricingCacheForTests();
  assert.ok(!(await listModelsForProvider('opencode_zen')).includes('model-x'));
});

test('an openrouter endpoint price is not admitted for a model with no capability entry', async () => {
  await publishPricing({
    catalog: { openrouter: { models: {} } },
    endpoints: {
      'vendor/uncatalogued': endpointListing('vendor/uncatalogued', [
        { prompt: '0.000001', completion: '0.000001' },
      ]),
    },
  });

  assert.deepEqual(await resolveModelCost('openrouter', 'vendor/uncatalogued'), {
    ok: false,
    reason: 'unpriced',
  });
});

test('openrouter admission reserves at the most expensive live endpoint, never the model-level price', async () => {
  const calls: string[] = [];
  await publishPricing(
    {
      catalog: {
        openrouter: {
          models: {
            'vendor/model-x': {
              id: 'vendor/model-x',
              cost: { input: 1, output: 1 },
            },
          },
        },
      },
      endpoints: {
        'vendor/model-x': endpointListing('vendor/model-x', [
          { prompt: '0.000001', completion: '0.000001' },
          { prompt: '0.000005', completion: '0.000002' },
        ]),
      },
    },
    calls,
  );

  assert.deepEqual(await lookupModelCost('openrouter', 'vendor/model-x'), {
    provider: 'openrouter',
    modelId: 'vendor/model-x',
    inputUsdPerMtok: 5,
    outputUsdPerMtok: 2,
    cacheReadUsdPerMtok: 5,
    cacheWriteUsdPerMtok: 5,
  });
  assert.ok(calls.includes('https://openrouter.ai/api/v1/models/vendor/model-x/endpoints'));
});

test('an openrouter model whose endpoint charges a per-request fee is unpriced, not admitted at a guess', async () => {
  await publishPricing({
    catalog: {
      openrouter: {
        models: { 'vendor/fee': { id: 'vendor/fee', cost: { input: 1, output: 1 } } },
      },
    },
    endpoints: {
      'vendor/fee': endpointListing('vendor/fee', [
        { prompt: '0.000001', completion: '0.000001', request: '0.01' },
      ]),
    },
  });

  assert.deepEqual(await resolveModelCost('openrouter', 'vendor/fee'), {
    ok: false,
    reason: 'unpriced',
  });
});

test('a dynamic openrouter router reporting -1 prices is unpriced', async () => {
  await publishPricing({
    catalog: {
      openrouter: {
        models: { 'openrouter/auto': { id: 'openrouter/auto', cost: { input: 1, output: 1 } } },
      },
    },
    endpoints: {
      'openrouter/auto': endpointListing('openrouter/auto', [
        { prompt: '-1', completion: '-1' },
      ]),
    },
  });

  assert.deepEqual(await resolveModelCost('openrouter', 'openrouter/auto'), {
    ok: false,
    reason: 'unpriced',
  });
});

test('a cold instance restores the validated snapshot instead of reaching the network', async () => {
  const calls: string[] = [];
  await publishPricing(
    {
      catalog: {
        opencode: { models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } } },
      },
    },
    calls,
  );
  assert.ok(await lookupModelCost('opencode_zen', 'model-a'));
  const callsBeforeRestart = calls.length;

  simulateColdInstanceForTests();
  globalThis.fetch = failingFetch(calls);

  assert.deepEqual(await lookupModelCost('opencode_zen', 'model-a'), {
    provider: 'opencode_zen',
    modelId: 'model-a',
    inputUsdPerMtok: 1,
    outputUsdPerMtok: 2,
  });
  assert.equal(calls.length, callsBeforeRestart);
});

test('a restored snapshot past the hard bound is refused while the network is down', async () => {
  await publishPricing({
    catalog: {
      opencode: { models: { 'model-a': { id: 'model-a', cost: { input: 1, output: 2 } } } },
    },
  });
  assert.ok(await lookupModelCost('opencode_zen', 'model-a'));

  expirePricingCacheForTests();
  simulateColdInstanceForTests();
  globalThis.fetch = failingFetch();

  assert.deepEqual(await resolveModelCost('opencode_zen', 'model-a'), {
    ok: false,
    reason: 'stale',
  });
});

test('openrouter keeps its catalog wire format while its price comes from endpoints', async () => {
  await publishPricing({
    catalog: {
      openrouter: {
        npm: '@openrouter/ai-sdk-provider',
        models: {
          'vendor/x': {
            id: 'vendor/x',
            provider: { npm: '@ai-sdk/anthropic', shape: 'messages' },
            cost: { input: 1, output: 1 },
          },
        },
      },
    },
    endpoints: {
      'vendor/x': endpointListing('vendor/x', [
        { prompt: '0.000003', completion: '0.000003' },
      ]),
    },
  });

  assert.deepEqual(await catalogSdkForModel('openrouter', 'vendor/x'), {
    npm: '@ai-sdk/anthropic',
    shape: 'messages',
  });
  assert.equal((await lookupModelCost('openrouter', 'vendor/x'))?.inputUsdPerMtok, 3);
});

test('image bounds read the endpoint price only once it has been published', async () => {
  assert.equal(loadedImageTokenLimits('openrouter', 'vendor/img'), undefined);
  await publishPricing({
    catalog: {
      openrouter: {
        models: {
          'vendor/img': {
            id: 'vendor/img',
            modalities: { input: ['text', 'image'] },
            limit: { context: 100_000, output: 8_000 },
            cost: { input: 1, output: 1 },
          },
        },
      },
    },
    endpoints: {
      'vendor/img': endpointListing('vendor/img', [
        { prompt: '0.000002', completion: '0.000001' },
      ]),
    },
  });

  assert.equal(
    loadedImageTokenLimits('openrouter', 'vendor/img')?.maximumCost.inputUsdPerMtok,
    2,
  );
});

test('a bundled entry is current inside its window, stale inside the hard bound, and refused past it', async () => {
  setBundledClockForTests(BUNDLED_CATALOG_VERIFIED_AT + SNAPSHOT_CURRENT_MAX_AGE_MS - 1);
  const current = await resolveModelCost('opencode_zen', 'gpt-5.6-luna');
  assert.ok(current.ok);
  assert.equal(current.cost.stale, undefined);

  setBundledClockForTests(BUNDLED_CATALOG_VERIFIED_AT + SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
  const stale = await resolveModelCost('opencode_zen', 'gpt-5.6-luna');
  assert.ok(stale.ok);
  assert.equal(stale.cost.stale, true);

  setBundledClockForTests(BUNDLED_CATALOG_VERIFIED_AT + SNAPSHOT_MAX_AGE_MS + 1);
  assert.deepEqual(await resolveModelCost('opencode_zen', 'gpt-5.6-luna'), {
    ok: false,
    reason: 'unverified',
  });
  assert.equal(await lookupModelDescriptor('opencode_zen', 'gpt-5.6-luna'), null);
});

test('a validated catalog that omits a model cannot fall back to the bundle once the bundle is past its bound', async () => {
  await publishPricing({ catalog: { opencode: { models: {} } } });
  assert.ok((await resolveModelCost('opencode_zen', 'gpt-5.6-luna')).ok);

  setBundledClockForTests(BUNDLED_CATALOG_VERIFIED_AT + SNAPSHOT_MAX_AGE_MS + 1);
  assert.deepEqual(await resolveModelCost('opencode_zen', 'gpt-5.6-luna'), {
    ok: false,
    reason: 'unpriced',
  });
});

import assert from 'node:assert/strict';
import { test } from 'node:test';
import { expirePricingCacheForTests, resetPricingCacheForTests } from '../pricing';
import { setStaleAdmissionsHaltedForTests } from '../pricing-overrun-cap';
import {
  SNAPSHOT_CURRENT_MAX_AGE_MS,
  SNAPSHOT_MAX_AGE_MS,
} from '../pricing-snapshot-store';
import {
  decisionModelFromCatalog,
  isSelectableDecisionModel,
  listDecisionModels,
  lookupDecisionModelCost,
  refreshDecisionCatalog,
} from './decision-models';

const JEV_CATALOG = {
  data: [
    {
      id: 'typesafe/jev-1.14',
      name: 'Jev',
      context_length: 262144,
      pricing: { prompt: '0.000000042', completion: '0', image: '0', request: '0' },
      architecture: { input_modalities: ['text', 'image'], output_modalities: ['decisions'] },
    },
  ],
};

test('a decision catalog refresh publishes a durable copy that lookups price without fetching', async () => {
  const originalFetch = globalThis.fetch;
  const requests: string[] = [];
  globalThis.fetch = (async (input: string | URL | Request) => {
    requests.push(String(input instanceof Request ? input.url : input));
    return Response.json(JEV_CATALOG);
  }) as unknown as typeof globalThis.fetch;
  resetPricingCacheForTests();
  try {
    assert.deepEqual(await listDecisionModels(), []);
    assert.equal(requests.length, 0, 'an unvalidated catalog is never fetched on read');

    await refreshDecisionCatalog();
    assert.equal(requests.length, 1);
    assert.match(requests[0], /\/models\?output_modalities=decisions$/);
    assert.equal((await listDecisionModels()).length, 1);

    const cost = await lookupDecisionModelCost('openrouter', 'typesafe/jev-1.14');
    assert.equal(cost?.modelId, 'typesafe/jev-1.14');
    assert.equal(cost?.stale, undefined);
    assert.equal(requests.length, 1, 'lookups read the durable copy only');

    expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
    assert.equal((await lookupDecisionModelCost('openrouter', 'typesafe/jev-1.14'))?.stale, true);

    expirePricingCacheForTests(SNAPSHOT_MAX_AGE_MS + 1);
    assert.equal(await lookupDecisionModelCost('openrouter', 'typesafe/jev-1.14'), null);
    assert.equal(requests.length, 1);
  } finally {
    globalThis.fetch = originalFetch;
    resetPricingCacheForTests();
  }
});

test('the overrun cap refuses a stale decision price only while it is reached', async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => Response.json(JEV_CATALOG)) as unknown as typeof globalThis.fetch;
  resetPricingCacheForTests();
  try {
    await refreshDecisionCatalog();
    setStaleAdmissionsHaltedForTests(true);
    assert.equal((await lookupDecisionModelCost('openrouter', 'typesafe/jev-1.14'))?.stale, undefined, 'current is never refused');

    expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
    assert.equal(await lookupDecisionModelCost('openrouter', 'typesafe/jev-1.14'), null);

    setStaleAdmissionsHaltedForTests(false);
    assert.equal((await lookupDecisionModelCost('openrouter', 'typesafe/jev-1.14'))?.stale, true);
  } finally {
    setStaleAdmissionsHaltedForTests(null);
    globalThis.fetch = originalFetch;
    resetPricingCacheForTests();
  }
});

test('uses decision catalog token prices and records image billing bounds', () => {
  const model = decisionModelFromCatalog({
    id: 'typesafe/jev-1.14',
    architecture: {
      input_modalities: ['text', 'image'],
      output_modalities: ['decisions'],
    },
    context_length: 262_144,
    pricing: {
      prompt: '0.000000042',
      completion: '0',
      image: '0',
      request: '0',
    },
  });
  assert.ok(Math.abs((model?.cost?.inputUsdPerMtok ?? 0) - 0.042) < 1e-12);
  assert.equal(model?.cost?.outputUsdPerMtok, 0);
  assert.equal(model?.imageTokenPriced, true);
  assert.equal(model?.contextLength, 262_144);
});

test('does not treat missing or nonzero image pricing as token-only', () => {
  const base = {
    id: 'vendor/model',
    architecture: { output_modalities: ['decisions'] },
  };
  assert.equal(
    decisionModelFromCatalog({
      ...base,
      pricing: { prompt: '1', completion: '0' },
    })?.imageTokenPriced,
    false,
  );
  assert.equal(
    decisionModelFromCatalog({
      ...base,
      pricing: { prompt: '1', completion: '0', image: '0.01' },
    })?.imageTokenPriced,
    false,
  );
});

test('accepts the documented token-priced image billing for Jev 1.14 when the catalog omits an image price', () => {
  const model = decisionModelFromCatalog({
    id: 'typesafe/jev-1.14',
    architecture: {
      input_modalities: ['text', 'image'],
      output_modalities: ['decisions'],
    },
    context_length: 262_144,
    pricing: { prompt: '0.000000042', completion: '0' },
  });
  assert.equal(model?.imageTokenPriced, true);
  assert.ok(model && isSelectableDecisionModel(model));
});

test('only offers image decision models with safe billing bounds', () => {
  const base = {
    provider: 'openrouter' as const,
    modelId: 'model',
    name: 'Model',
    acceptsImages: true,
    contextLength: 1000,
    imageTokenPriced: true,
    cost: {
      provider: 'openrouter',
      modelId: 'model',
      inputUsdPerMtok: 1,
      outputUsdPerMtok: 0,
    },
  };
  assert.equal(isSelectableDecisionModel(base), true);
  assert.equal(
    isSelectableDecisionModel({ ...base, acceptsImages: false }),
    false,
  );
  assert.equal(
    isSelectableDecisionModel({ ...base, imageTokenPriced: false }),
    false,
  );
  assert.equal(
    isSelectableDecisionModel({ ...base, contextLength: undefined }),
    false,
  );
  assert.equal(isSelectableDecisionModel({ ...base, cost: null }), false);
  assert.equal(
    isSelectableDecisionModel({
      ...base,
      cost: { ...base.cost, outputUsdPerMtok: 1 },
    }),
    false,
  );
});

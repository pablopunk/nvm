import assert from 'node:assert/strict';
import { test } from 'node:test';
import { decisionModelFromCatalog, isSelectableDecisionModel } from './decision-models';

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
  const base = { provider: 'openrouter' as const, modelId: 'model', name: 'Model', acceptsImages: true, contextLength: 1000, imageTokenPriced: true, cost: { provider: 'openrouter', modelId: 'model', inputUsdPerMtok: 1, outputUsdPerMtok: 0 } };
  assert.equal(isSelectableDecisionModel(base), true);
  assert.equal(isSelectableDecisionModel({ ...base, acceptsImages: false }), false);
  assert.equal(isSelectableDecisionModel({ ...base, imageTokenPriced: false }), false);
  assert.equal(isSelectableDecisionModel({ ...base, contextLength: undefined }), false);
  assert.equal(isSelectableDecisionModel({ ...base, cost: null }), false);
  assert.equal(isSelectableDecisionModel({ ...base, cost: { ...base.cost, outputUsdPerMtok: 1 } }), false);
});

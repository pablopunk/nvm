import assert from 'node:assert/strict';
import { test } from 'node:test';
import { decisionModelFromCatalog } from './decision-models';

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
  assert.equal(model?.cost?.inputUsdPerMtok, 0.042);
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

import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE,
  reviewedImageTokenLimits,
} from './image-cost-bound';
import {
  conservativeImageRequestInputTokenCeiling,
  supportsBoundedImageInput,
} from './limits';

test('bounds image billing by the reviewed route model input limit', () => {
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
  assert.equal(
    supportsBoundedImageInput('openrouter', 'openai/gpt-6.1-sol', [
      'text',
      'image',
    ]),
    true,
  );
  assert.equal(
    supportsBoundedImageInput('openrouter', 'meta/muse-spark-1.3-contributor', [
      'text',
      'image',
    ]),
    true,
  );
  assert.equal(
    supportsBoundedImageInput(
      'opencode_zen',
      'muse-spark-1.3-contributor-free',
      ['text', 'image'],
    ),
    false,
  );
  assert.equal(
    supportsBoundedImageInput('openrouter', 'openai/gpt-6.1-sol', ['text']),
    false,
  );
  assert.deepEqual(
    reviewedImageTokenLimits('openrouter', 'openai/gpt-6.1-sol'),
    {
      contextWindow: 1_050_000,
      maxInputTokens: 922_000,
      maxTokens: 128_000,
      maximumCost: {
        inputUsdPerMtok: 2,
        outputUsdPerMtok: 10,
        cacheReadUsdPerMtok: 0.1,
        cacheWriteUsdPerMtok: 2.5,
        tiers: [
          {
            thresholdTokens: 272_000,
            inputUsdPerMtok: 4,
            outputUsdPerMtok: 15,
            cacheReadUsdPerMtok: 0.2,
            cacheWriteUsdPerMtok: 5,
          },
        ],
      },
    },
  );
  assert.equal(
    reviewedImageTokenLimits('opencode_zen', 'muse-spark-1.3-contributor-free'),
    undefined,
  );
});

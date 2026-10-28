import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { ModelCost } from '../cost';
import { computeUsdCost } from '../cost';
import { IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE } from '../image-cost-bound';
import {
  decisionInputTokenCeiling,
  decisionOutputTokenCeiling,
  decisionReservationCostUsd,
  settleDecisionCost,
} from './decision-billing';
import { type DecisionRequest, parseDecisionRequest } from './decision-request';

const COST: ModelCost = {
  provider: 'openrouter',
  modelId: 'vendor/model',
  inputUsdPerMtok: 1,
  outputUsdPerMtok: 4,
};
const PNG_DATA = 'iVBORw0KGgo=';

function requestWith(imageCount: number, questionCount = 1): DecisionRequest {
  const parsed = parseDecisionRequest({
    input: 'Shared context',
    images: Array.from({ length: imageCount }, () => ({
      mediaType: 'image/png',
      data: PNG_DATA,
    })),
    questions: Array.from({ length: questionCount }, (_, index) => ({
      id: `q${index}`,
      type: 'predicate',
      question: 'Is it?',
    })),
  });
  if (!parsed.ok) throw new Error(parsed.message);
  return parsed.request;
}

test('reserves a higher ceiling for images and more questions', () => {
  const textOnly = requestWith(0);
  const withImage = requestWith(1);
  assert.equal(
    decisionInputTokenCeiling(withImage) - decisionInputTokenCeiling(textOnly),
    IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE,
  );
  assert.ok(
    decisionReservationCostUsd(COST, withImage) >
      decisionReservationCostUsd(COST, textOnly),
  );
  assert.equal(decisionOutputTokenCeiling(requestWith(0, 3)), 64 + 3 * 64);
});

test('reserves exactly the catalog price of the ceiling tokens', () => {
  const request = requestWith(2, 2);
  assert.equal(
    decisionReservationCostUsd(COST, request),
    computeUsdCost(
      COST,
      decisionInputTokenCeiling(request),
      decisionOutputTokenCeiling(request),
    ),
  );
});

test('settles with the upstream-reported cost when present', () => {
  const settlement = settleDecisionCost({
    costRow: COST,
    reservedCostUsd: 0.01,
    usage: { inputTokens: 10, outputTokens: 2 },
    providerCostUsd: 0.0002,
  });
  assert.deepEqual(settlement, {
    providerCostUsd: 0.0002,
    costSource: 'provider_reported',
    tokens: { inputTokens: 10, outputTokens: 2 },
  });
});

test('prices reported tokens from the catalog when the upstream omits cost', () => {
  const settlement = settleDecisionCost({
    costRow: COST,
    reservedCostUsd: 0.01,
    usage: { inputTokens: 1_000, outputTokens: 100 },
  });
  assert.equal(settlement.costSource, 'catalog_estimate');
  assert.equal(settlement.providerCostUsd, computeUsdCost(COST, 1_000, 100));
});

test('keeps the reservation ceiling when a 2xx response carries no usable usage', () => {
  const settlement = settleDecisionCost({
    costRow: COST,
    reservedCostUsd: 0.01,
  });
  assert.deepEqual(settlement, {
    providerCostUsd: 0.01,
    costSource: 'catalog_estimate',
    tokens: { inputTokens: 0, outputTokens: 0 },
  });
});

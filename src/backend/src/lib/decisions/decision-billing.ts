import { computeUsdCost, type ModelCost, usdToCredits } from '../cost';
import { estimateInputTokensFromValue } from '../limits';
import { IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE } from '../image-cost-bound';
import type { DecisionUsage } from './decision-contract';
import type { DecisionRequest } from './decision-request';

export const DECISION_OUTPUT_TOKENS_BASE = 64;
export const DECISION_OUTPUT_TOKENS_PER_QUESTION = 64;

export type DecisionCostSource = 'provider_reported' | 'catalog_estimate';

export function decisionInputTokenCeiling(request: DecisionRequest): number {
  const textTokens = estimateInputTokensFromValue({
    input: request.input,
    questions: request.questions,
  });
  return (
    textTokens + request.images.length * IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE
  );
}

export function decisionOutputTokenCeiling(request: DecisionRequest): number {
  return (
    DECISION_OUTPUT_TOKENS_BASE +
    DECISION_OUTPUT_TOKENS_PER_QUESTION * request.questions.length
  );
}

export function decisionReservationCostUsd(
  costRow: ModelCost,
  request: DecisionRequest,
): number {
  return computeUsdCost(
    costRow,
    decisionInputTokenCeiling(request),
    decisionOutputTokenCeiling(request),
  );
}

export function decisionReservationCredits(
  costRow: ModelCost,
  request: DecisionRequest,
): number {
  return usdToCredits(decisionReservationCostUsd(costRow, request));
}

export type DecisionSettlement = {
  providerCostUsd: number;
  costSource: DecisionCostSource;
  tokens: DecisionUsage;
};

/**
 * Upstream-reported cost wins. Without it, the catalog price of the reported
 * tokens is used; when even tokens are missing on a 2xx response, the
 * reservation ceiling is kept so ambiguous upstream usage never settles cheaper.
 */
export function settleDecisionCost(input: {
  costRow: ModelCost;
  reservedCostUsd: number;
  usage?: DecisionUsage;
  providerCostUsd?: number;
}): DecisionSettlement {
  const usage = input.usage ?? { inputTokens: 0, outputTokens: 0 };
  if (input.providerCostUsd !== undefined)
    return {
      providerCostUsd: input.providerCostUsd,
      costSource: 'provider_reported',
      tokens: usage,
    };
  if (input.usage)
    return {
      providerCostUsd: computeUsdCost(
        input.costRow,
        usage.inputTokens,
        usage.outputTokens,
      ),
      costSource: 'catalog_estimate',
      tokens: usage,
    };
  return {
    providerCostUsd: input.reservedCostUsd,
    costSource: 'catalog_estimate',
    tokens: usage,
  };
}

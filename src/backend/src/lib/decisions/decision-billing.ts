import { computeUsdCost, type ModelCost, usdToCredits } from '../cost';
import { reservationMarginFor } from '../pricing';
import { estimateInputTokensFromValue } from '../limits';
import { IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE } from '../image-cost-bound';
import type { DecisionUsage } from './decision-contract';
import type { DecisionRequest } from './decision-request';

export const DECISION_OUTPUT_TOKENS_BASE = 64;
export const DECISION_OUTPUT_TOKENS_PER_QUESTION = 64;

export type DecisionCostSource = 'provider_reported' | 'catalog_estimate';

/** Decisions are only bounded when every output tier is free of charge. */
export function decisionOutputIsUnbilled(costRow: ModelCost): boolean {
  return (
    costRow.outputUsdPerMtok === 0 &&
    !costRow.tiers?.some(
      (tier) => (tier.outputUsdPerMtok ?? costRow.outputUsdPerMtok) !== 0,
    )
  );
}

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

/** A stale catalog price holds the stale margin, so the reservation covers a rate that moved. */
export function decisionReservationCostUsd(
  costRow: ModelCost,
  request: DecisionRequest,
): number {
  return (
    reservationMarginFor(costRow) *
    computeUsdCost(
      costRow,
      decisionInputTokenCeiling(request),
      decisionOutputTokenCeiling(request),
    )
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

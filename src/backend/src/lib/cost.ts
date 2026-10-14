export type { ModelCost } from './pricing';
export { lookupModelCost } from './pricing';

import { env } from './env';
import type { ModelCost } from './pricing';

function readPositiveNumberEnv(key: string, fallback: number): number {
  const raw = env(key);
  const parsed = raw && raw.trim() ? Number(raw) : fallback;
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function numberEnvSource(
  key: string,
  fallback: number,
): 'environment' | 'default' {
  const raw = env(key);
  const parsed = raw && raw.trim() ? Number(raw) : fallback;
  return Number.isFinite(parsed) && parsed > 0 && raw?.trim()
    ? 'environment'
    : 'default';
}

export const CREDIT_USD = readPositiveNumberEnv('CREDIT_USD', 0.01);
export const MARKUP = readPositiveNumberEnv('CREDIT_MARKUP', 5);
export const CREDIT_USD_SOURCE = numberEnvSource('CREDIT_USD', 0.01);
export const MARKUP_SOURCE = numberEnvSource('CREDIT_MARKUP', 5);

export function computeUsdCost(
  cost: ModelCost,
  inputTokens: number,
  outputTokens: number,
  details: { cachedInputTokens?: number; cacheWriteInputTokens?: number } = {},
): number {
  const tier = [...(cost.tiers ?? [])]
    .filter((candidate) => inputTokens >= candidate.thresholdTokens)
    .sort((a, b) => b.thresholdTokens - a.thresholdTokens)[0];
  const inputRate = tier?.inputUsdPerMtok ?? cost.inputUsdPerMtok;
  const outputRate = tier?.outputUsdPerMtok ?? cost.outputUsdPerMtok;
  const cacheReadRate =
    tier?.cacheReadUsdPerMtok ?? cost.cacheReadUsdPerMtok ?? inputRate;
  const cacheWriteRate =
    tier?.cacheWriteUsdPerMtok ?? cost.cacheWriteUsdPerMtok ?? inputRate;
  const cachedInputTokens = Math.max(
    0,
    Math.min(inputTokens, details.cachedInputTokens ?? 0),
  );
  const cacheWriteInputTokens = Math.max(
    0,
    Math.min(
      inputTokens - cachedInputTokens,
      details.cacheWriteInputTokens ?? 0,
    ),
  );
  const regularInputTokens = Math.max(
    0,
    inputTokens - cachedInputTokens - cacheWriteInputTokens,
  );
  return (
    (regularInputTokens * inputRate +
      cachedInputTokens * cacheReadRate +
      cacheWriteInputTokens * cacheWriteRate +
      outputTokens * outputRate) /
    1_000_000
  );
}

/** Cost ceiling when preflight cannot know how input tokens will be cached. */
export function computeMaximumTokenPricedCost(
  cost: ModelCost,
  inputTokens: number,
  outputTokens: number,
  reviewedMaximum?: Pick<
    ModelCost,
    | 'inputUsdPerMtok'
    | 'outputUsdPerMtok'
    | 'cacheReadUsdPerMtok'
    | 'cacheWriteUsdPerMtok'
    | 'tiers'
  >,
): number {
  const priceRows = reviewedMaximum ? [cost, reviewedMaximum] : [cost];
  const inputRates = priceRows.flatMap((row) => {
    const applicableTiers = (row.tiers ?? []).filter(
      (tier) => inputTokens >= tier.thresholdTokens,
    );
    return [
      row.inputUsdPerMtok,
      row.cacheReadUsdPerMtok ?? row.inputUsdPerMtok,
      row.cacheWriteUsdPerMtok ?? row.inputUsdPerMtok,
      ...applicableTiers.flatMap((tier) => [
        tier.inputUsdPerMtok ?? row.inputUsdPerMtok,
        tier.cacheReadUsdPerMtok ??
          row.cacheReadUsdPerMtok ??
          tier.inputUsdPerMtok ??
          row.inputUsdPerMtok,
        tier.cacheWriteUsdPerMtok ??
          row.cacheWriteUsdPerMtok ??
          tier.inputUsdPerMtok ??
          row.inputUsdPerMtok,
      ]),
    ];
  });
  const outputRates = priceRows.flatMap((row) => [
    row.outputUsdPerMtok,
    ...(row.tiers ?? [])
      .filter((tier) => inputTokens >= tier.thresholdTokens)
      .map((tier) => tier.outputUsdPerMtok ?? row.outputUsdPerMtok),
  ]);
  return (
    (inputTokens * Math.max(...inputRates) +
      outputTokens * Math.max(...outputRates)) /
    1_000_000
  );
}

export function usdToCredits(costUsd: number): number {
  return Math.max(1, Math.ceil((costUsd * MARKUP) / CREDIT_USD));
}

export function usdToMicrocents(costUsd: number): number {
  return Math.round(costUsd * 1e8);
}

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

export function estimateAudioProviderCostUsd(
  cost: ModelCost,
  input: { audioBytes: number; maxInputBytes: number; outputTokens?: number },
): number {
  if (
    !Number.isSafeInteger(input.audioBytes) ||
    !Number.isSafeInteger(input.maxInputBytes) ||
    input.audioBytes < 0 ||
    input.maxInputBytes < 1 ||
    input.audioBytes > input.maxInputBytes
  ) {
    throw new Error('Invalid audio size for provider-cost estimate');
  }
  const estimatedInputTokens = Math.ceil(input.audioBytes / 4);
  return computeUsdCost(
    cost,
    estimatedInputTokens,
    input.outputTokens ?? 32_000,
  );
}

export function usdToCredits(costUsd: number): number {
  return Math.max(1, Math.ceil((costUsd * MARKUP) / CREDIT_USD));
}

export function usdToMicrocents(costUsd: number): number {
  return Math.round(costUsd * 1e8);
}

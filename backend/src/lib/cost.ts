export type { ModelCost } from './pricing';
export { lookupModelCost } from './pricing';
import { env } from './env';
import type { ModelCost } from './pricing';

function readPositiveNumberEnv(key: string, fallback: number): number {
  const raw = env(key);
  const parsed = raw && raw.trim() ? Number(raw) : fallback;
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

export const CREDIT_USD = readPositiveNumberEnv('CREDIT_USD', 0.01);
export const MARKUP = readPositiveNumberEnv('CREDIT_MARKUP', 5);

export function computeUsdCost(cost: ModelCost, inputTokens: number, outputTokens: number): number {
  return (inputTokens * cost.inputUsdPerMtok + outputTokens * cost.outputUsdPerMtok) / 1_000_000;
}

export function usdToCredits(costUsd: number): number {
  return Math.max(1, Math.ceil((costUsd * MARKUP) / CREDIT_USD));
}

export function usdToMicrocents(costUsd: number): number {
  return Math.round(costUsd * 1e8);
}

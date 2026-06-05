export type { ModelCost } from './pricing';
export { lookupModelCost } from './pricing';
import { env } from './env';
import type { ModelCost } from './pricing';

export const CREDIT_USD = Number(env('CREDIT_USD') ?? 0.0001);
export const MARKUP = Number(env('CREDIT_MARKUP') ?? 1.2);

export function computeUsdCost(cost: ModelCost, inputTokens: number, outputTokens: number): number {
  return (inputTokens * cost.inputUsdPerMtok + outputTokens * cost.outputUsdPerMtok) / 1_000_000;
}

export function usdToCredits(costUsd: number): number {
  return Math.max(1, Math.ceil((costUsd * MARKUP) / CREDIT_USD));
}

export function usdToMicrocents(costUsd: number): number {
  return Math.round(costUsd * 1e8);
}

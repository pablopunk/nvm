import { and, eq } from 'drizzle-orm';
import { db } from '../db/client';
import { modelCosts } from '../db/schema';

export const CREDIT_USD = Number(import.meta.env.CREDIT_USD ?? 0.0001);
export const MARKUP = Number(import.meta.env.CREDIT_MARKUP ?? 1.2);

export type ModelCost = {
  provider: string;
  modelId: string;
  inputUsdPerMtok: number;
  outputUsdPerMtok: number;
};

export async function lookupModelCost(provider: string, modelId: string): Promise<ModelCost | null> {
  const [row] = await db
    .select()
    .from(modelCosts)
    .where(and(eq(modelCosts.provider, provider), eq(modelCosts.modelId, modelId)))
    .limit(1);
  if (!row) return null;
  return {
    provider: row.provider,
    modelId: row.modelId,
    inputUsdPerMtok: Number(row.inputUsdPerMtok),
    outputUsdPerMtok: Number(row.outputUsdPerMtok),
  };
}

export function computeUsdCost(cost: ModelCost, inputTokens: number, outputTokens: number): number {
  return (inputTokens * cost.inputUsdPerMtok + outputTokens * cost.outputUsdPerMtok) / 1_000_000;
}

export function usdToCredits(costUsd: number): number {
  return Math.max(1, Math.ceil((costUsd * MARKUP) / CREDIT_USD));
}

export function usdToMicrocents(costUsd: number): number {
  return Math.round(costUsd * 1e8);
}

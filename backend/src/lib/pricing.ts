export type ModelPricing = {
  realModel: string;
  inputCreditsPerMTok: number;
  outputCreditsPerMTok: number;
};

const MODELS: Record<string, ModelPricing> = {
  'gemini-3-flash': { realModel: 'gemini-3-flash', inputCreditsPerMTok: 30, outputCreditsPerMTok: 250 },
  'gemini-3.5-flash': { realModel: 'gemini-3.5-flash', inputCreditsPerMTok: 30, outputCreditsPerMTok: 250 },
  'gemini-3.1-pro': { realModel: 'gemini-3.1-pro', inputCreditsPerMTok: 1250, outputCreditsPerMTok: 10000 },
  'claude-haiku-4-5': { realModel: 'claude-haiku-4-5', inputCreditsPerMTok: 800, outputCreditsPerMTok: 4000 },
  'claude-sonnet-4-6': { realModel: 'claude-sonnet-4-6', inputCreditsPerMTok: 3000, outputCreditsPerMTok: 15000 },
  'deepseek-v4-flash-free': { realModel: 'deepseek-v4-flash-free', inputCreditsPerMTok: 0, outputCreditsPerMTok: 0 },
  auto: { realModel: 'gemini-3-flash', inputCreditsPerMTok: 30, outputCreditsPerMTok: 250 },
};

export const DEFAULT_MODEL = 'gemini-3-flash';

export function resolveModel(requested: string | undefined) {
  return MODELS[requested ?? DEFAULT_MODEL] ?? MODELS[DEFAULT_MODEL];
}

export function computeCostCredits(pricing: ModelPricing, inputTokens: number, outputTokens: number) {
  const input = (inputTokens * pricing.inputCreditsPerMTok) / 1_000_000;
  const output = (outputTokens * pricing.outputCreditsPerMTok) / 1_000_000;
  return Math.max(1, Math.ceil(input + output));
}

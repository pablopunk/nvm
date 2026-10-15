// Input-limit heuristic only. Billing reserves the model's full input ceiling.
export const IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE = 8_192;

// These routes bill image input through the same token-priced input pool as
// text. Keep these ceilings pinned to reviewed upstream limits instead of
// falling back to the generic descriptor defaults. Keep Responses-only routes
// out until the proxy supports them.
// Sources reviewed 2026-10-14:
// - OpenAI GPT-6.1 Sol: image input, 922,000 max input, 1,050,000 context,
//   128,000 max output, and token/tier pricing.
//   https://developers.openai.com/api/docs/models/gpt-6.1-sol
//   https://developers.openai.com/api/docs/guides/images-vision
//   https://openrouter.ai/openai/gpt-6.1-sol
// - OpenRouter's live model catalog documents the routed context/output caps,
//   modalities, token prices, and context tiers for all three allowlisted slugs.
//   https://openrouter.ai/api/v1/models
// - Gemini 2.5 Flash: Google documents 1,048,576 input and 65,536 output;
//   OpenRouter caps this route at 65,535 and prices image tokens at input rate.
//   https://ai.google.dev/gemini-api/docs/models/gemini-2.5-flash
// - Muse Spark Contributor: OpenRouter documents image input, a 1,048,576
//   context, and token-based input/output pricing, with 943,718 max output.
//   Meta confirms image usage is token-counted and documents a separate fee for
//   hosted web search. The OpenRouter slug uses this proxy's Chat Completions
//   gateway; Meta's separate direct API uses /responses and is not allowlisted.
//   https://openrouter.ai/meta/muse-spark-1.3-contributor
//   https://dev.meta.ai/docs/token-counting
//   https://dev.meta.ai/docs/pricing-rate-limits
// - OpenRouter's documented chat API accepts image content parts and provides
//   the Chat Completions endpoint used by this proxy.
//   https://openrouter.ai/docs/api/api-reference/chat/create-a-chat-completion
export type ReviewedImageTokenLimits = {
  contextWindow: number;
  maxInputTokens: number;
  maxTokens: number;
  maximumCost: {
    inputUsdPerMtok: number;
    outputUsdPerMtok: number;
    cacheReadUsdPerMtok?: number;
    cacheWriteUsdPerMtok?: number;
    tiers?: Array<{
      thresholdTokens: number;
      inputUsdPerMtok?: number;
      outputUsdPerMtok?: number;
      cacheReadUsdPerMtok?: number;
      cacheWriteUsdPerMtok?: number;
    }>;
  };
};

const REVIEWED_IMAGE_TOKEN_LIMITS: Record<string, ReviewedImageTokenLimits> = {
  'openrouter/openai/gpt-6.1-sol': {
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
  'openrouter/google/gemini-2.5-flash': {
    contextWindow: 1_048_576,
    maxInputTokens: 1_048_576,
    maxTokens: 65_535,
    maximumCost: {
      inputUsdPerMtok: 0.3,
      outputUsdPerMtok: 2.5,
      cacheReadUsdPerMtok: 0.03,
      cacheWriteUsdPerMtok: 0.083333,
    },
  },
  'openrouter/meta/muse-spark-1.3-contributor': {
    contextWindow: 1_048_576,
    maxInputTokens: 1_048_576,
    maxTokens: 943_718,
    maximumCost: {
      inputUsdPerMtok: 0.1,
      outputUsdPerMtok: 0.2,
      cacheReadUsdPerMtok: 0.002,
    },
  },
};

export function reviewedImageTokenLimits(
  provider: string,
  modelId: string,
): ReviewedImageTokenLimits | undefined {
  return REVIEWED_IMAGE_TOKEN_LIMITS[`${provider}/${modelId}`];
}

export function hasReviewedImageBilling(
  provider: string,
  modelId: string,
): boolean {
  return reviewedImageTokenLimits(provider, modelId) !== undefined;
}

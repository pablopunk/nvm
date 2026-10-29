type ModelsDevCost = {
  input?: number;
  output?: number;
  cache_read?: number;
  cache_write?: number;
  tiers?: Array<{
    input?: number;
    output?: number;
    cache_read?: number;
    cache_write?: number;
    tier?: { size?: number };
  }>;
};

const OPENROUTER_ENDPOINTS_PATH = /\/models\/(.+)\/endpoints$/;

/** The model id a `/models/{id}/endpoints` URL asks about, or null for any other URL. */
export function openRouterEndpointsModelId(url: string): string | null {
  const match = OPENROUTER_ENDPOINTS_PATH.exec(new URL(url).pathname);
  if (!match) return null;
  return decodeURIComponent(match[1]);
}

/**
 * An endpoint listing whose single endpoint charges the given models.dev cost, in
 * OpenRouter's per-token decimal-string form. Tests keep their expected amounts
 * because the endpoint price equals the cost the test already declares.
 */
export function openRouterEndpointsBodyFromCost(
  modelId: string,
  cost: ModelsDevCost,
) {
  const perToken = (perMtok: number | undefined) =>
    perMtok === undefined ? undefined : (perMtok / 1_000_000).toFixed(12);
  const pricing = {
    prompt: perToken(cost.input),
    completion: perToken(cost.output),
    input_cache_read: perToken(cost.cache_read),
    input_cache_write: perToken(cost.cache_write),
    discount: 0,
    overrides: (cost.tiers ?? []).flatMap((tier) =>
      tier.tier?.size == null
        ? []
        : [
            {
              min_prompt_tokens: tier.tier.size,
              prompt: perToken(tier.input),
              completion: perToken(tier.output),
              input_cache_read: perToken(tier.cache_read),
              input_cache_write: perToken(tier.cache_write),
            },
          ],
    ),
  };
  return {
    data: {
      id: modelId,
      name: modelId,
      endpoints: [{ provider_name: 'Test', status: 0, pricing }],
    },
  };
}

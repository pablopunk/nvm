import { loadedImageTokenLimits, type ImageTokenLimits } from './pricing';

// Input-limit heuristic only. Billing reserves the model's full input ceiling.
export const IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE = 8_192;

export type { ImageTokenLimits };

/** Image requests bill through the token-priced input pool, bounded by the catalog. */
export function imageTokenLimits(
  provider: string,
  modelId: string,
): ImageTokenLimits | undefined {
  return loadedImageTokenLimits(provider, modelId);
}

export function hasImageBilling(provider: string, modelId: string): boolean {
  return imageTokenLimits(provider, modelId) !== undefined;
}

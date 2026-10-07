import { DEFAULT_NEVERMIND_MAX_INPUT_TOKENS } from '../../../app/shared/ai-context-window';
import { computeUsdCost, type ModelCost, usdToCredits } from './cost';

export const MAX_INPUT_TOKENS = Number(
  process.env.MAX_INPUT_TOKENS ?? DEFAULT_NEVERMIND_MAX_INPUT_TOKENS,
);

const CHARS_PER_TOKEN = 4;
const TOKENS_PER_IMAGE = 1_600;
const IMAGE_TYPE_PATTERN = /image/i;
const IMAGE_DATA_URL_PATTERN = /^data:image\//i;
const IMAGE_FIELD_PATTERN = /(?:image_url|imageUrl|input_image|inlineData)["']?\s*:/i;
type InputEstimate = { textCharacters: number; images: number };

function imagePayloadCount(node: Record<string, unknown>): number {
  const type = typeof node.type === 'string' ? node.type.toLowerCase() : '';
  const hasImageKey = ['image', 'image_url', 'imageUrl', 'input_image', 'inlineData']
    .some((key) => Object.hasOwn(node, key));
  const source = node.source;
  const inlineData = node.inlineData;
  const hasImageMime = (candidate: unknown) =>
    typeof candidate === 'string' && candidate.toLowerCase().startsWith('image/');
  const hasImageSource = source !== null && typeof source === 'object'
    && hasImageMime((source as Record<string, unknown>).media_type);
  const hasImageInlineData = inlineData !== null && typeof inlineData === 'object'
    && hasImageMime((inlineData as Record<string, unknown>).mimeType);

  // Admission rejects image modalities instead of trusting estimated pixels or
  // counting remote URLs/base64 as ordinary text. Detect intent, not validity.
  return IMAGE_TYPE_PATTERN.test(type) || hasImageKey || hasImageSource || hasImageInlineData
    ? 1
    : 0;
}

function collectInputEstimate(node: unknown, estimate: InputEstimate): void {
  if (typeof node === 'string') {
    if (IMAGE_DATA_URL_PATTERN.test(node)) {
      estimate.images += 1;
      return;
    }
    estimate.textCharacters += node.length;
    return;
  }
  if (Array.isArray(node)) {
    for (const value of node) collectInputEstimate(value, estimate);
    return;
  }
  if (node && typeof node === 'object') {
    const imageCount = imagePayloadCount(node as Record<string, unknown>);
    if (imageCount) {
      estimate.images += imageCount;
      return;
    }
    for (const value of Object.values(node))
      collectInputEstimate(value, estimate);
  }
}

export function containsImagePayload(value: unknown): boolean {
  if (typeof value === 'string') return IMAGE_DATA_URL_PATTERN.test(value) || IMAGE_FIELD_PATTERN.test(value);
  if (Array.isArray(value)) return value.some(containsImagePayload);
  if (!value || typeof value !== 'object') return false;
  const node = value as Record<string, unknown>;
  if (imagePayloadCount(node) > 0) return true;
  return Object.values(node).some(containsImagePayload);
}

export function estimateInputTokensFromValue(value: unknown): number {
  const estimate: InputEstimate = { textCharacters: 0, images: 0 };
  collectInputEstimate(value, estimate);
  return (
    Math.ceil(estimate.textCharacters / CHARS_PER_TOKEN) +
    estimate.images * TOKENS_PER_IMAGE
  );
}

export function estimateInputTokensFromBody(bodyText: string): number {
  if (!bodyText) return 0;
  try {
    return estimateInputTokensFromValue(JSON.parse(bodyText));
  } catch {
    return Math.ceil(bodyText.length / CHARS_PER_TOKEN);
  }
}

export function estimatePromptCredits(
  inputTokens: number,
  cost: ModelCost,
): number {
  return usdToCredits(computeUsdCost(cost, inputTokens, 0));
}

export function estimateRequestCredits(
  inputTokens: number,
  maxOutputTokens: number,
  cost: ModelCost,
): number {
  return usdToCredits(computeUsdCost(cost, inputTokens, maxOutputTokens));
}

function positiveInteger(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
    ? Math.floor(value)
    : undefined;
}

/** Extract the provider-specific output cap from a parsed request body. */
export function requestedMaxOutputTokens(body: unknown): number | undefined {
  if (!body || typeof body !== 'object' || Array.isArray(body))
    return undefined;
  const request = body as Record<string, unknown>;
  return (
    positiveInteger(request.max_tokens) ??
    positiveInteger(request.max_completion_tokens) ??
    (request.generationConfig && typeof request.generationConfig === 'object'
      ? positiveInteger(
          (request.generationConfig as Record<string, unknown>).maxOutputTokens,
        )
      : undefined)
  );
}

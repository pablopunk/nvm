import { DEFAULT_NEVERMIND_MAX_INPUT_TOKENS } from '../../../app/shared/ai-context-window';
import { computeUsdCost, type ModelCost, usdToCredits } from './cost';

export const MAX_INPUT_TOKENS = Number(process.env.MAX_INPUT_TOKENS ?? DEFAULT_NEVERMIND_MAX_INPUT_TOKENS);

const CHARS_PER_TOKEN = 4;
const TOKENS_PER_IMAGE = 1_600;
const IMAGE_TYPE_PATTERN = /image/i;
const IMAGE_DATA_URL_PATTERN = /^data:image\//i;
const MULTIMODAL_DATA_URI_PATTERN = /^data:(?:image|audio|video|application\/(?:pdf|msword|vnd\.openxmlformats-officedocument)|text\/)/i;
const MULTIMODAL_FIELD_NAMES =
  'image_url|imageUrl|input_image|inlineData|audio_url|audioUrl|input_audio|audio|video_url|videoUrl|video|pdf|document|file_url|fileData|inline_data';
const MULTIMODAL_FIELD_PATTERN = new RegExp(`["'](?:${MULTIMODAL_FIELD_NAMES})["']\\s*:`, 'i');
const MULTIMODAL_KEY_PATTERN = new RegExp(`^(?:${MULTIMODAL_FIELD_NAMES})$`, 'i');
const MULTIMODAL_TYPE_PATTERN = /(?:^|[_-])(?:image|audio|video|pdf|document|file)(?:$|[_-])/i;
const MULTIMODAL_MIME_PATTERN = /^(?:image|audio|video)\//i;
const DOCUMENT_MIME_PATTERN = /^(?:application\/(?:pdf|msword|vnd\.openxmlformats-officedocument)|text\/)/i;
type InputEstimate = { textCharacters: number; images: number };

function imagePayloadCount(node: Record<string, unknown>): number {
  const type = typeof node.type === 'string' ? node.type.toLowerCase() : '';
  const hasImageKey = ['image', 'image_url', 'imageUrl', 'input_image', 'inlineData'].some((key) => Object.hasOwn(node, key));
  const source = node.source;
  const inlineData = node.inlineData;
  const hasImageMime = (candidate: unknown) => typeof candidate === 'string' && candidate.toLowerCase().startsWith('image/');
  const hasImageSource = source !== null && typeof source === 'object' && hasImageMime((source as Record<string, unknown>).media_type);
  const hasImageInlineData =
    inlineData !== null && typeof inlineData === 'object' && hasImageMime((inlineData as Record<string, unknown>).mimeType);

  // Admission rejects image modalities instead of trusting estimated pixels or
  // counting remote URLs/base64 as ordinary text. Detect intent, not validity.
  return IMAGE_TYPE_PATTERN.test(type) || hasImageKey || hasImageSource || hasImageInlineData ? 1 : 0;
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
    for (const value of Object.values(node)) collectInputEstimate(value, estimate);
  }
}

export function containsImagePayload(value: unknown): boolean {
  return containsUnsupportedMultimodalPayload(value);
}

/** Detect modalities whose provider cost cannot be bounded by the text estimator. */
export function containsUnsupportedMultimodalPayload(value: unknown): boolean {
  if (typeof value === 'string') {
    return MULTIMODAL_DATA_URI_PATTERN.test(value) || MULTIMODAL_FIELD_PATTERN.test(value);
  }
  if (Array.isArray(value)) return value.some(containsUnsupportedMultimodalPayload);
  if (!value || typeof value !== 'object') return false;
  const node = value as Record<string, unknown>;
  if (imagePayloadCount(node) > 0) return true;
  if (Object.keys(node).some((key) => MULTIMODAL_KEY_PATTERN.test(key))) return true;
  if (typeof node.type === 'string' && MULTIMODAL_TYPE_PATTERN.test(node.type)) return true;
  for (const field of ['mimeType', 'mime_type', 'media_type', 'content_type']) {
    const value = node[field];
    if (typeof value === 'string' && (MULTIMODAL_MIME_PATTERN.test(value) || DOCUMENT_MIME_PATTERN.test(value))) return true;
  }
  return Object.values(node).some(containsUnsupportedMultimodalPayload);
}

export function estimateInputTokensFromValue(value: unknown): number {
  const estimate: InputEstimate = { textCharacters: 0, images: 0 };
  collectInputEstimate(value, estimate);
  return Math.ceil(estimate.textCharacters / CHARS_PER_TOKEN) + estimate.images * TOKENS_PER_IMAGE;
}

export function estimateInputTokensFromBody(bodyText: string): number {
  if (!bodyText) return 0;
  try {
    return estimateInputTokensFromValue(JSON.parse(bodyText));
  } catch {
    return Math.ceil(bodyText.length / CHARS_PER_TOKEN);
  }
}

export function estimatePromptCredits(inputTokens: number, cost: ModelCost): number {
  return usdToCredits(computeUsdCost(cost, inputTokens, 0));
}

export function estimateRequestCredits(inputTokens: number, maxOutputTokens: number, cost: ModelCost): number {
  return usdToCredits(computeUsdCost(cost, inputTokens, maxOutputTokens));
}

function positiveInteger(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.floor(value) : undefined;
}

/** Extract the provider-specific output cap from a parsed request body. */
export function requestedMaxOutputTokens(body: unknown): number | undefined {
  if (!body || typeof body !== 'object' || Array.isArray(body)) return undefined;
  const request = body as Record<string, unknown>;
  return (
    positiveInteger(request.max_tokens) ??
    positiveInteger(request.max_completion_tokens) ??
    (request.generationConfig && typeof request.generationConfig === 'object'
      ? positiveInteger((request.generationConfig as Record<string, unknown>).maxOutputTokens)
      : undefined)
  );
}

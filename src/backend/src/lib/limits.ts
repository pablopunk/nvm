import { computeUsdCost, usdToCredits, type ModelCost } from './cost';

export const MAX_INPUT_TOKENS = Number(process.env.MAX_INPUT_TOKENS ?? 100_000);

const CHARS_PER_TOKEN = 4;
const TOKENS_PER_IMAGE = 1_600;
const IMAGE_MIME_TYPES = new Set([
  'image/png',
  'image/jpeg',
  'image/webp',
  'image/gif',
]);

type InputEstimate = { textCharacters: number; images: number };

function isBase64(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    value.length > 0 &&
    value.length % 4 === 0 &&
    /^[A-Za-z0-9+/]*={0,2}$/.test(value)
  );
}

function isImageDataUrl(value: unknown): value is string {
  if (typeof value !== 'string') return false;
  const separator = value.indexOf(',');
  if (separator < 0) return false;
  const mimeType = value.slice(5, separator - ';base64'.length).toLowerCase();
  return (
    value.slice(0, 5) === 'data:' &&
    value.slice(separator - ';base64'.length, separator) === ';base64' &&
    IMAGE_MIME_TYPES.has(mimeType) &&
    isBase64(value.slice(separator + 1))
  );
}

function hasOnlyKeys(
  node: Record<string, unknown>,
  keys: readonly string[],
) {
  return Object.keys(node).every((key) => keys.includes(key));
}

function isAnthropicCacheControl(value: unknown) {
  if (value === undefined) return true;
  if (!(value && typeof value === 'object')) return false;
  const cacheControl = value as Record<string, unknown>;
  return (
    hasOnlyKeys(cacheControl, ['type', 'ttl']) &&
    cacheControl.type === 'ephemeral' &&
    (cacheControl.ttl === undefined ||
      cacheControl.ttl === '5m' ||
      cacheControl.ttl === '1h')
  );
}

function imagePayloadCount(node: Record<string, unknown>): number {
  const imageUrl = node.image_url as Record<string, unknown> | undefined;
  if (
    node.type === 'image_url' &&
    hasOnlyKeys(node, ['type', 'image_url']) &&
    isImageDataUrl(node.image_url)
  )
    return 1;

  if (
    node.type === 'image_url' &&
    hasOnlyKeys(node, ['type', 'image_url']) &&
    imageUrl &&
    typeof imageUrl === 'object' &&
    hasOnlyKeys(imageUrl, ['url']) &&
    isImageDataUrl(imageUrl.url)
  )
    return 1;

  if (
    node.type === 'input_image' &&
    hasOnlyKeys(node, ['type', 'detail', 'image_url']) &&
    node.detail === 'auto' &&
    isImageDataUrl(node.image_url)
  )
    return 1;

  const source = node.source as Record<string, unknown> | undefined;
  if (
    node.type === 'image' &&
    hasOnlyKeys(node, ['type', 'source', 'cache_control']) &&
    isAnthropicCacheControl(node.cache_control) &&
    source &&
    typeof source === 'object' &&
    hasOnlyKeys(source, ['type', 'media_type', 'data']) &&
    source?.type === 'base64' &&
    typeof source.media_type === 'string' &&
    IMAGE_MIME_TYPES.has(source.media_type.toLowerCase()) &&
    isBase64(source.data)
  )
    return 1;

  const inlineData = node.inlineData as Record<string, unknown> | undefined;
  if (
    hasOnlyKeys(node, ['inlineData']) &&
    inlineData &&
    typeof inlineData === 'object' &&
    hasOnlyKeys(inlineData, ['mimeType', 'data']) &&
    typeof inlineData?.mimeType === 'string' &&
    IMAGE_MIME_TYPES.has(inlineData.mimeType.toLowerCase()) &&
    isBase64(inlineData.data)
  )
    return 1;

  return 0;
}

function collectInputEstimate(node: unknown, estimate: InputEstimate): void {
  if (typeof node === 'string') {
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

export function estimateInputTokensFromBody(bodyText: string): number {
  if (!bodyText) return 0;
  let parsed: unknown;
  try { parsed = JSON.parse(bodyText); } catch { return Math.ceil(bodyText.length / CHARS_PER_TOKEN); }
  const estimate: InputEstimate = { textCharacters: 0, images: 0 };
  collectInputEstimate(parsed, estimate);
  return (
    Math.ceil(estimate.textCharacters / CHARS_PER_TOKEN) +
    estimate.images * TOKENS_PER_IMAGE
  );
}

export function estimatePromptCredits(inputTokens: number, cost: ModelCost): number {
  return usdToCredits(computeUsdCost(cost, inputTokens, 0));
}

export function estimateRequestCredits(inputTokens: number, maxOutputTokens: number, cost: ModelCost): number {
  return usdToCredits(computeUsdCost(cost, inputTokens, maxOutputTokens));
}

function positiveInteger(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
    ? Math.floor(value)
    : undefined;
}

/** Extract the provider-specific output cap from a parsed request body. */
export function requestedMaxOutputTokens(body: unknown): number | undefined {
  if (!body || typeof body !== 'object' || Array.isArray(body)) return undefined;
  const request = body as Record<string, unknown>;
  return positiveInteger(request.max_tokens)
    ?? positiveInteger(request.max_completion_tokens)
    ?? (request.generationConfig && typeof request.generationConfig === 'object'
      ? positiveInteger((request.generationConfig as Record<string, unknown>).maxOutputTokens)
      : undefined);
}

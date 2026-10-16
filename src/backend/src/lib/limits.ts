import { DEFAULT_NEVERMIND_MAX_INPUT_TOKENS } from '../../../app/shared/ai-context-window';
import { analyzeAiImageInputs, imagePayloadIntent } from './ai-image-input';
import { computeUsdCost, type ModelCost, usdToCredits } from './cost';
import {
  hasReviewedImageBilling,
  IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE,
} from './image-cost-bound';

export const MAX_INPUT_TOKENS = Number(
  process.env.MAX_INPUT_TOKENS ?? DEFAULT_NEVERMIND_MAX_INPUT_TOKENS,
);

const CHARS_PER_TOKEN = 4;
export const MAX_CHAT_COMPLETION_CHOICES = 128;
export const OUTPUT_CONTEXT_SAFETY_TOKENS = 4_096;
export const MAX_PROXY_REQUEST_BODY_BYTES = 4 * 1024 * 1024;
const MULTIMODAL_DATA_URI_PATTERN =
  /^data:(?:image|audio|video|application\/(?:pdf|msword|vnd\.openxmlformats-officedocument)|text\/)/i;
const MULTIMODAL_FIELD_NAMES =
  'image_url|imageUrl|input_image|inlineData|audio_url|audioUrl|input_audio|audio|video_url|videoUrl|video|pdf|document|file_url|fileData|inline_data';
const MULTIMODAL_FIELD_PATTERN = new RegExp(
  `["'](?:${MULTIMODAL_FIELD_NAMES})["']\\s*:`,
  'i',
);
const MULTIMODAL_KEY_PATTERN = new RegExp(
  `^(?:${MULTIMODAL_FIELD_NAMES})$`,
  'i',
);
const UNPRICED_TOOL_TYPES = new Set(['openrouter:web_search']);
const MULTIMODAL_TYPE_PATTERN =
  /(?:^|[_-])(?:image|audio|video|pdf|document|file)(?:$|[_-])/i;
const MULTIMODAL_MIME_PATTERN = /^(?:image|audio|video)\//i;
const DOCUMENT_MIME_PATTERN =
  /^(?:application\/(?:pdf|msword|vnd\.openxmlformats-officedocument)|text\/)/i;
type InputEstimate = { textCharacters: number; images: number };

function collectInputEstimate(node: unknown, estimate: InputEstimate): void {
  if (typeof node === 'string') {
    if (/^data:image\//i.test(node)) {
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
    if (imagePayloadIntent(node as Record<string, unknown>)) {
      estimate.images += 1;
      return;
    }
    for (const value of Object.values(node))
      collectInputEstimate(value, estimate);
  }
}

export function containsImagePayload(value: unknown): boolean {
  return (
    analyzeAiImageInputs(value).imageCount > 0 ||
    containsUnsupportedMultimodalPayload(value)
  );
}

/** OpenRouter search is separately billed per use, outside token pricing. */
export function containsUnpricedProviderTool(value: unknown): boolean {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const node = value as Record<string, unknown>;
  if (
    Array.isArray(node.tools) &&
    node.tools.some((tool) => {
      if (!tool || typeof tool !== 'object' || Array.isArray(tool))
        return false;
      const type = (tool as Record<string, unknown>).type;
      return typeof type === 'string' && UNPRICED_TOOL_TYPES.has(type);
    })
  )
    return true;
  return false;
}

const SCHEMA_DEFINITION_FIELDS = [
  'tools',
  'functions',
  'tool_choice',
  'response_format',
];

/** Tool and format schemas describe shapes; their property names are not media inputs. */
export function withoutSchemaDefinitions(value: unknown): unknown {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return value;
  const node = { ...(value as Record<string, unknown>) };
  for (const field of SCHEMA_DEFINITION_FIELDS) delete node[field];
  return node;
}

/** Detect modalities whose provider cost cannot be bounded by the text estimator. */
export function containsUnsupportedMultimodalPayload(
  value: unknown,
  imageAnalysis = analyzeAiImageInputs(value),
): boolean {
  if (imageAnalysis.invalid) return true;
  if (typeof value === 'string') return rawTextLooksMultimodal(value);
  return containsUnsupportedNonImagePayload(value);
}

/** Unparseable bodies have no structure, so only raw text patterns can reveal media. */
function rawTextLooksMultimodal(text: string): boolean {
  return (
    MULTIMODAL_DATA_URI_PATTERN.test(text) ||
    MULTIMODAL_FIELD_PATTERN.test(text)
  );
}

/** Parsed strings are text (prompts, tool results, source code) and never media by content. */
function isModalityDataUri(text: string): boolean {
  return MULTIMODAL_DATA_URI_PATTERN.test(text);
}

function containsUnsupportedNonImagePayload(value: unknown): boolean {
  if (typeof value === 'string') return isModalityDataUri(value);
  if (Array.isArray(value))
    return value.some(containsUnsupportedNonImagePayload);
  if (!value || typeof value !== 'object') return false;
  const node = value as Record<string, unknown>;
  if (imagePayloadIntent(node)) return false;
  if (
    Object.hasOwn(node, 'modalities') &&
    (!Array.isArray(node.modalities) ||
      node.modalities.some(
        (modality) =>
          typeof modality !== 'string' || modality.toLowerCase() !== 'text',
      ))
  )
    return true;
  if (Object.keys(node).some((key) => MULTIMODAL_KEY_PATTERN.test(key)))
    return true;
  if (typeof node.type === 'string' && MULTIMODAL_TYPE_PATTERN.test(node.type))
    return true;
  for (const field of ['mimeType', 'mime_type', 'media_type', 'content_type']) {
    const value = node[field];
    if (
      typeof value === 'string' &&
      (MULTIMODAL_MIME_PATTERN.test(value) || DOCUMENT_MIME_PATTERN.test(value))
    )
      return true;
  }
  return Object.values(node).some(containsUnsupportedNonImagePayload);
}

export function estimateInputTokensFromValue(value: unknown): number {
  const estimate: InputEstimate = { textCharacters: 0, images: 0 };
  collectInputEstimate(value, estimate);
  return (
    Math.ceil(estimate.textCharacters / CHARS_PER_TOKEN) +
    estimate.images * IMAGE_INPUT_TOKENS_ESTIMATE_PER_IMAGE
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

export function supportsBoundedImageInput(
  provider: string,
  modelId: string,
  inputModalities: string[] | undefined,
): boolean {
  return Boolean(
    inputModalities?.includes('image') &&
      hasReviewedImageBilling(provider, modelId),
  );
}

export function conservativeImageRequestInputTokenCeiling(
  descriptor: {
    contextWindow: number;
    maxInputTokens?: number;
    maxTokens: number;
  },
  requestedOutputTokens: number | undefined,
  estimatedInputTokens: number,
): number | undefined {
  const maximumOutputTokens = Math.min(
    requestedOutputTokens ?? descriptor.maxTokens,
    descriptor.maxTokens,
  );
  const contextInputCeiling =
    descriptor.contextWindow -
    maximumOutputTokens -
    OUTPUT_CONTEXT_SAFETY_TOKENS;
  const inputCeiling = Math.min(
    descriptor.maxInputTokens ?? descriptor.contextWindow,
    contextInputCeiling,
  );
  if (
    !Number.isSafeInteger(inputCeiling) ||
    inputCeiling < estimatedInputTokens ||
    estimatedInputTokens < 0
  )
    return undefined;
  return inputCeiling;
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

/** Read the OpenAI-compatible `n` field, rejecting malformed or excessive counts. */
export function requestedCompletionCount(body: unknown): number | undefined {
  if (!body || typeof body !== 'object' || Array.isArray(body)) return 1;
  const request = body as Record<string, unknown>;
  if (!Object.hasOwn(request, 'n')) return 1;
  const count = request.n;
  return typeof count === 'number' &&
    Number.isSafeInteger(count) &&
    count >= 1 &&
    count <= MAX_CHAT_COMPLETION_CHOICES
    ? count
    : undefined;
}

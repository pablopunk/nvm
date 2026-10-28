import type {
  ExtensionDecisionAnswer,
  ExtensionDecisionQuestion,
  ExtensionDecisionResult,
} from '../../resources/nevermind-extension-api';

export const DECISION_IMAGE_MAX_COUNT = 4;
export const DECISION_IMAGE_MAX_DECODED_BYTES = 2 * 1024 * 1024;
export const DECISION_IMAGE_MEDIA_TYPES = [
  'image/png',
  'image/jpeg',
  'image/webp',
] as const;

export type ExtensionAiDecisionInputErrorCode =
  | 'decision_too_many_images'
  | 'decision_image_type_unsupported'
  | 'decision_image_too_large'
  | 'decision_attachment_unreadable';

export class ExtensionAiDecisionInputError extends Error {
  readonly code: ExtensionAiDecisionInputErrorCode;

  constructor(code: ExtensionAiDecisionInputErrorCode, message: string) {
    super(message);
    this.name = 'ExtensionAiDecisionInputError';
    this.code = code;
  }
}

export type DecisionImageInput = {
  type: 'image';
  data: string;
  mimeType: string;
};

export type DecisionWireRequest = {
  input: string;
  images: Array<{ mediaType: string; data: string }>;
  questions: ExtensionDecisionQuestion[];
};

function decodedBase64ByteLength(data: string): number {
  const padding = data.endsWith('==') ? 2 : data.endsWith('=') ? 1 : 0;
  return Math.floor((data.length * 3) / 4) - padding;
}

function assertDecisionImageWithinBounds(
  image: DecisionImageInput,
  index: number,
) {
  if (
    !(DECISION_IMAGE_MEDIA_TYPES as readonly string[]).includes(image.mimeType)
  )
    throw new ExtensionAiDecisionInputError(
      'decision_image_type_unsupported',
      `Decision image ${index + 1} is ${image.mimeType}; use PNG, JPEG or WebP.`,
    );
  if (decodedBase64ByteLength(image.data) > DECISION_IMAGE_MAX_DECODED_BYTES)
    throw new ExtensionAiDecisionInputError(
      'decision_image_too_large',
      `Decision image ${index + 1} is larger than 2 MiB. Resize it before deciding.`,
    );
}

/** Images are bounded and rejected loudly; they are never silently dropped. */
export function buildDecisionWireRequest(input: {
  input: string;
  contextText: string;
  images: DecisionImageInput[];
  questions: ExtensionDecisionQuestion[];
}): DecisionWireRequest {
  if (input.images.length > DECISION_IMAGE_MAX_COUNT)
    throw new ExtensionAiDecisionInputError(
      'decision_too_many_images',
      `Decisions accept at most ${DECISION_IMAGE_MAX_COUNT} images; ${input.images.length} were attached.`,
    );
  input.images.forEach(assertDecisionImageWithinBounds);
  return {
    input: [input.input, input.contextText].filter(Boolean).join('\n\n'),
    images: input.images.map((image) => ({
      mediaType: image.mimeType,
      data: image.data,
    })),
    questions: input.questions,
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function isAnswer(value: unknown): value is ExtensionDecisionAnswer {
  if (!isRecord(value)) return false;
  if (value.type === 'predicate')
    return (
      typeof value.value === 'boolean' && typeof value.probability === 'number'
    );
  if (value.type === 'choice') return typeof value.value === 'string';
  if (value.type === 'score') return typeof value.value === 'number';
  return false;
}

/** Desktop trusts the backend's typed answers only after this shape check. */
export function parseDecisionWireResponse(
  body: unknown,
): ExtensionDecisionResult {
  if (
    !isRecord(body) ||
    typeof body.id !== 'string' ||
    typeof body.model !== 'string' ||
    !isRecord(body.answers) ||
    !isRecord(body.usage) ||
    !Object.values(body.answers).every(isAnswer)
  )
    throw new Error('Decision response has an unexpected shape');
  return {
    id: body.id,
    model: body.model,
    answers: body.answers as Record<string, ExtensionDecisionAnswer>,
    usage: {
      inputTokens: Number(body.usage.inputTokens) || 0,
      outputTokens: Number(body.usage.outputTokens) || 0,
    },
  };
}

export function decisionRequestFailure(
  status: number,
  body: unknown,
): Error & { code: string; status: number } {
  const error = (body as { error?: { type?: unknown; message?: unknown } })
    ?.error;
  const code = typeof error?.type === 'string' ? error.type : 'decision_failed';
  const message =
    typeof error?.message === 'string'
      ? error.message
      : `Decision request failed with ${status}`;
  return Object.assign(new Error(message), { code, status });
}

import {
  AI_CHAT_IMAGE_LIMIT,
  AI_CHAT_IMAGE_MAX_BYTES,
  AI_CHAT_IMAGE_MAX_DIMENSION,
  AI_CHAT_IMAGE_MIME_TYPES,
  AI_CHAT_IMAGE_TOTAL_MAX_BYTES,
} from '../../../app/shared/ai-chat-images';
import { imageDimensions } from '../../../app/shared/ai-image-dimensions';

type ImageInput = {
  mimeType: string;
  data: string;
};

type ImagePayloadCandidate = {
  intent: boolean;
  image?: ImageInput;
};

export type AiImageInputAnalysis = {
  imageCount: number;
  imageBytes: number;
  invalid: boolean;
};

function record(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function hasOnlyKeys(value: Record<string, unknown>, keys: string[]): boolean {
  return Object.keys(value).every((key) => keys.includes(key));
}

function imageMimeType(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const mimeType = value.toLowerCase();
  return AI_CHAT_IMAGE_MIME_TYPES.includes(
    mimeType as (typeof AI_CHAT_IMAGE_MIME_TYPES)[number],
  )
    ? mimeType
    : null;
}

function isImageIntent(node: Record<string, unknown>): boolean {
  const type = typeof node.type === 'string' ? node.type.toLowerCase() : '';
  if (type.includes('image')) return true;
  if (
    ['image', 'image_url', 'imageUrl', 'input_image'].some((key) =>
      Object.hasOwn(node, key),
    )
  )
    return true;
  const source = record(node.source);
  const inlineData = record(node.inlineData) ?? record(node.inline_data);
  return Boolean(
    imageMimeType(source?.media_type) ||
      imageMimeType(inlineData?.mimeType ?? inlineData?.mime_type),
  );
}

function imagePayloadCandidate(
  node: Record<string, unknown>,
): ImagePayloadCandidate {
  const type = typeof node.type === 'string' ? node.type.toLowerCase() : '';
  if (type === 'image_url' || type === 'input_image') {
    const imageUrl = node.image_url ?? node.imageUrl;
    const imageUrlRecord = record(imageUrl);
    const url =
      typeof imageUrl === 'string'
        ? imageUrl
        : typeof imageUrlRecord?.url === 'string'
          ? imageUrlRecord.url
          : undefined;
    const validImageUrlKeys =
      imageUrlRecord === null || hasOnlyKeys(imageUrlRecord, ['url', 'detail']);
    const validPartKeys = hasOnlyKeys(node, [
      'type',
      'image_url',
      'imageUrl',
      'detail',
    ]);
    const mimeType =
      /^data:(image\/[^;,]+);base64,/i.exec(url ?? '')?.[1] ?? '';
    return {
      intent: true,
      ...(url && validImageUrlKeys && validPartKeys
        ? { image: { mimeType, data: url } }
        : {}),
    };
  }

  if (type === 'image') {
    const source = record(node.source);
    const validSource = Boolean(
      source &&
        hasOnlyKeys(source, ['type', 'media_type', 'data']) &&
        source.type === 'base64',
    );
    const validPart = hasOnlyKeys(node, ['type', 'source', 'cache_control']);
    return {
      intent: true,
      ...(validSource && validPart && source
        ? {
            image: {
              mimeType: String(source.media_type ?? ''),
              data: String(source.data ?? ''),
            },
          }
        : {}),
    };
  }

  for (const key of ['inlineData', 'inline_data']) {
    if (!Object.hasOwn(node, key)) continue;
    const inlineData = record(node[key]);
    const mimeType = inlineData?.mimeType ?? inlineData?.mime_type;
    if (!imageMimeType(mimeType)) continue;
    const data = inlineData?.data;
    const validInlineData = Boolean(
      inlineData &&
        hasOnlyKeys(inlineData, ['mimeType', 'mime_type', 'data']) &&
        typeof data === 'string',
    );
    const type = typeof node.type === 'string' ? node.type.toLowerCase() : '';
    const validPart =
      hasOnlyKeys(node, [key, 'type']) && (!type || type.includes('image'));
    return {
      intent: true,
      ...(validInlineData && validPart
        ? { image: { mimeType: String(mimeType ?? ''), data: data as string } }
        : {}),
    };
  }

  return { intent: isImageIntent(node) };
}

function parseInlineImage(image: ImageInput) {
  const mimeType = imageMimeType(image.mimeType);
  const dataUrl =
    /^data:(image\/(?:png|jpeg|webp|gif));base64,([A-Za-z0-9+/]*={0,2})$/i.exec(
      image.data,
    );
  const resolvedMimeType = mimeType ?? imageMimeType(dataUrl?.[1]);
  if (
    !resolvedMimeType ||
    (mimeType && dataUrl && dataUrl[1].toLowerCase() !== mimeType)
  )
    return null;
  const base64 = dataUrl ? dataUrl[2] : image.data;
  if (!base64 || base64.length % 4 !== 0) return null;
  const bytes = Buffer.from(base64, 'base64');
  if (
    !bytes.length ||
    bytes.length > AI_CHAT_IMAGE_MAX_BYTES ||
    bytes.toString('base64') !== base64
  )
    return null;
  const dimensions = imageDimensions(bytes, resolvedMimeType);
  if (
    !dimensions ||
    dimensions.width < 1 ||
    dimensions.height < 1 ||
    dimensions.width > AI_CHAT_IMAGE_MAX_DIMENSION ||
    dimensions.height > AI_CHAT_IMAGE_MAX_DIMENSION
  )
    return null;
  return { byteLength: bytes.length };
}

export function analyzeAiImageInputs(value: unknown): AiImageInputAnalysis {
  const result: AiImageInputAnalysis = {
    imageCount: 0,
    imageBytes: 0,
    invalid: false,
  };

  function visit(node: unknown) {
    if (Array.isArray(node)) {
      for (const value of node) visit(value);
      return;
    }
    const nodeRecord = record(node);
    if (!nodeRecord) return;
    const candidate = imagePayloadCandidate(nodeRecord);
    if (candidate.intent) {
      const image = candidate.image && parseInlineImage(candidate.image);
      if (!image) {
        result.invalid = true;
        return;
      }
      result.imageCount += 1;
      result.imageBytes += image.byteLength;
      return;
    }
    for (const value of Object.values(nodeRecord)) visit(value);
  }

  visit(value);
  if (
    result.imageCount > AI_CHAT_IMAGE_LIMIT ||
    result.imageBytes > AI_CHAT_IMAGE_TOTAL_MAX_BYTES
  )
    result.invalid = true;
  return result;
}

export function imagePayloadIntent(node: Record<string, unknown>) {
  return isImageIntent(node);
}

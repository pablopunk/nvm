import {
  AI_CHAT_IMAGE_LIMIT,
  AI_CHAT_IMAGE_MAX_BYTES,
  AI_CHAT_IMAGE_MAX_DIMENSION,
  AI_CHAT_IMAGE_MIME_TYPES,
  AI_CHAT_IMAGE_TOTAL_MAX_BYTES,
  type AiChatImageInput,
} from '../../shared/ai-chat-images';
import { imageDimensions } from '../../shared/ai-image-dimensions';

const IMAGE_FILE_EXTENSIONS: Record<string, string> = {
  'image/png': 'png',
  'image/jpeg': 'jpg',
  'image/webp': 'webp',
  'image/gif': 'gif',
};
const BASE64_BYTES_PER_GROUP = 3;
const BASE64_CHARACTERS_PER_GROUP = 4;
const AI_CHAT_IMAGE_MAX_BASE64_LENGTH =
  Math.ceil(AI_CHAT_IMAGE_MAX_BYTES / BASE64_BYTES_PER_GROUP) *
  BASE64_CHARACTERS_PER_GROUP;

export type NormalizedAiChatImage = AiChatImageInput & {
  bytes: Buffer;
  extension: string;
};

export function normalizeAiChatImages(input: unknown): NormalizedAiChatImage[] {
  if (input == null) {
    return [];
  }
  if (!Array.isArray(input)) {
    throw new Error('AI chat images must be an array');
  }
  if (input.length > AI_CHAT_IMAGE_LIMIT)
    throw new Error(`Attach no more than ${AI_CHAT_IMAGE_LIMIT} images`);

  let totalBytes = 0;
  return input.map((value) => {
    if (!value || typeof value !== 'object') {
      throw new Error('AI chat image is invalid');
    }
    const image = value as Record<string, unknown>;
    const mimeType = String(image.mimeType || '').toLowerCase();
    if (!AI_CHAT_IMAGE_MIME_TYPES.includes(mimeType as never))
      throw new Error('AI chat image must be PNG, JPEG, WebP, or GIF');
    const data = image.data;
    if (
      typeof data !== 'string' ||
      !data ||
      data.length % 4 !== 0 ||
      !/^[A-Za-z0-9+/]*={0,2}$/.test(data)
    )
      throw new Error('AI chat image data is invalid');
    if (data.length > AI_CHAT_IMAGE_MAX_BASE64_LENGTH)
      throw new Error('Each AI chat image must be 2 MB or smaller');
    const bytes = Buffer.from(data, 'base64');
    if (!bytes.length || bytes.length > AI_CHAT_IMAGE_MAX_BYTES)
      throw new Error('Each AI chat image must be 2 MB or smaller');
    if (bytes.toString('base64') !== data)
      throw new Error('AI chat image data is invalid');
    const dimensions = imageDimensions(bytes, mimeType);
    if (
      !dimensions ||
      dimensions.width < 1 ||
      dimensions.height < 1 ||
      dimensions.width > AI_CHAT_IMAGE_MAX_DIMENSION ||
      dimensions.height > AI_CHAT_IMAGE_MAX_DIMENSION
    )
      throw new Error(
        'AI chat images must be valid and 2048 pixels or smaller',
      );
    totalBytes += bytes.length;
    if (totalBytes > AI_CHAT_IMAGE_TOTAL_MAX_BYTES)
      throw new Error('AI chat images must total 2 MB or less');
    return {
      data,
      mimeType,
      bytes,
      extension: IMAGE_FILE_EXTENSIONS[mimeType],
      ...(typeof image.name === 'string' && image.name.trim()
        ? { name: image.name.trim().slice(0, 200) }
        : {}),
    };
  });
}

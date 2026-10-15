export const AI_CHAT_IMAGE_LIMIT = 8;
export const AI_CHAT_IMAGE_MAX_DIMENSION = 2048;
export const AI_CHAT_IMAGE_MAX_BYTES = 2_097_152;
export const AI_CHAT_IMAGE_TOTAL_MAX_BYTES = AI_CHAT_IMAGE_MAX_BYTES;

export const AI_CHAT_IMAGE_MIME_TYPES = [
  'image/png',
  'image/jpeg',
  'image/webp',
  'image/gif',
] as const;

export type AiChatImageInput = {
  data: string;
  mimeType: string;
  name?: string;
};

export type AiChatMessageImage = {
  url: string;
  alt?: string;
};

export type AiChatMessage = {
  role: 'user' | 'assistant' | 'system';
  content: string;
  images?: AiChatMessageImage[];
};

export function canSendAiChatMessage(message: string, imageCount: number) {
  return Boolean(message.trim() || imageCount > 0);
}

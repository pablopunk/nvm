export const AI_CHAT_IMAGE_LIMIT = 8;
export const AI_CHAT_IMAGE_MAX_BYTES = 8 * 1024 * 1024;
export const AI_CHAT_IMAGE_TOTAL_MAX_BYTES = 24 * 1024 * 1024;

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

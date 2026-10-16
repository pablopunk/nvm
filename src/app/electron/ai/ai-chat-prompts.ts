// biome-ignore-all lint: AI chat messages and images are dynamic persisted payloads.
import fs from 'node:fs/promises';
import {
  joinAiChatTranscript,
  truncateAiChatTranscriptContent,
} from '../../shared/ai-chat-context';
import {
  AI_CHAT_IMAGE_LIMIT,
  AI_CHAT_IMAGE_MIME_TYPES,
  AI_CHAT_IMAGE_TOTAL_MAX_BYTES,
} from '../../shared/ai-chat-images';
import type { UserStateStore } from '../state/user-state-store';
import type { NormalizedAiChatImage } from './ai-chat-images';

export const CONVERSATION_SYSTEM_PROMPT =
  'You are Nevermind AI, a helpful conversational assistant. Answer the user directly and concisely. You cannot create, modify, validate, install, or remove Nevermind extensions.';

export function aiChatTranscriptContent(message) {
  const imageCount = Array.isArray(message.images) ? message.images.length : 0;
  const imageNotice =
    imageCount === 1 ? '[Attached image]' : `[Attached ${imageCount} images]`;
  return truncateAiChatTranscriptContent(
    [message.content, imageCount ? imageNotice : ''].filter(Boolean).join('\n'),
  );
}

export function aiChatPromptMessage(message, imageCount) {
  const trimmed = String(message || '').trim();
  if (trimmed) return trimmed;
  return imageCount === 1
    ? 'Analyze the attached image.'
    : 'Analyze the attached images.';
}

export function piImages(images: NormalizedAiChatImage[]) {
  return images.map(({ data, mimeType }) => ({
    type: 'image' as const,
    data,
    mimeType,
  }));
}

export type AiChatPromptsDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  relevantLearningContext: (message: string, chatId: string) => string;
  storedAiChatImagePath: (image: any) => string | null | undefined;
};

export function createAiChatPrompts(deps: AiChatPromptsDeps) {
  const { userStateStore, relevantLearningContext, storedAiChatImagePath } =
    deps;

  function aiChatPromptWithContext(message, chatId) {
    const chat = userStateStore.get().aiChats[chatId];
    const focused = chat?.contextExtensionFile
      ? `\n\nFocused extension file: ${chat.contextExtensionFile}. Use read_current_extension before editing it. You may list/read other extensions if needed.`
      : '';
    const learnings = relevantLearningContext(message, chatId);
    const messages = (chat?.messages || [])
      .filter((item) => item.role === 'user' || item.role === 'assistant')
      .slice(-12);

    if (!messages.length)
      return `Use this Nevermind AI chat transcript as context. If the user has provided enough details, proceed by calling read_extension_api immediately; do not merely say you will.${focused}${learnings}\n\nNew user message:\n${message}`;

    const transcript = joinAiChatTranscript(
      messages.map(
        (item) =>
          `${item.role === 'user' ? 'User' : 'Assistant'}: ${aiChatTranscriptContent(item)}`,
      ),
    );

    return `Use this Nevermind AI chat transcript as context. Do not ask questions that the user already answered. If the user has now provided enough details, proceed by calling read_extension_api immediately; do not merely say you will.${focused}${learnings}\n\n${transcript}\n\nNew user message:\n${message}`;
  }

  function conversationPromptWithContext(message, chatId) {
    const chat = userStateStore.get().aiChats[chatId];
    const transcript = joinAiChatTranscript(
      (chat?.messages || [])
        .filter((item) => item.role === 'user' || item.role === 'assistant')
        .slice(-20)
        .map(
          (item) =>
            `${item.role === 'user' ? 'User' : 'Assistant'}: ${aiChatTranscriptContent(item)}`,
        ),
    );
    return transcript ? `${transcript}\n\nUser: ${message}` : message;
  }

  async function restoredConversationImages(
    chat,
    currentImages: NormalizedAiChatImage[],
  ) {
    let remainingCount = AI_CHAT_IMAGE_LIMIT - currentImages.length;
    let remainingBytes =
      AI_CHAT_IMAGE_TOTAL_MAX_BYTES -
      currentImages.reduce((total, image) => total + image.bytes.length, 0);
    if (remainingCount <= 0 || remainingBytes <= 0) return [];
    const candidates = (chat.messages || [])
      .slice(-20)
      .flatMap((message) =>
        Array.isArray(message.images) ? message.images : [],
      )
      .reverse();
    const restored: Array<{
      type: 'image';
      data: string;
      mimeType: string;
    }> = [];
    for (const image of candidates) {
      if (remainingCount <= 0 || remainingBytes <= 0) break;
      const imagePath = storedAiChatImagePath(image);
      const mimeType = String(image?.mimeType || '').toLowerCase();
      if (!imagePath || !AI_CHAT_IMAGE_MIME_TYPES.includes(mimeType as never))
        continue;
      const bytes = await fs.readFile(imagePath).catch(() => null);
      if (!(bytes && bytes.length <= remainingBytes)) continue;
      restored.push({
        type: 'image',
        data: bytes.toString('base64'),
        mimeType,
      });
      remainingCount -= 1;
      remainingBytes -= bytes.length;
    }
    return restored.reverse();
  }

  return {
    aiChatPromptWithContext,
    conversationPromptWithContext,
    restoredConversationImages,
  };
}

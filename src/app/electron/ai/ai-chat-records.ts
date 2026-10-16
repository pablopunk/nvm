// biome-ignore-all lint: AI chat records are dynamic persisted payloads.
import path from 'node:path';
import { AUTOMATE_AI_CHAT_MODEL } from '../../shared/ai-chat-model';

type AnyRecord = Record<string, any>;

export function isConversationAiChat(chat: AnyRecord | null | undefined) {
  return chat?.kind === 'conversation';
}

export function isBuilderAiChat(chat: AnyRecord | null | undefined) {
  return Boolean(chat) && !isConversationAiChat(chat);
}

export function chatTouchedExtensionFiles(chat) {
  if (!isBuilderAiChat(chat)) return [];
  return Array.from(
    new Set(
      [
        ...(chat?.touchedExtensionFiles || []),
        chat?.generatedExtensionFile,
        chat?.contextExtensionFile,
      ]
        .filter(Boolean)
        .map((item) => path.basename(item)),
    ),
  );
}

export function aiChatItem(id, query) {
  return {
    id,
    kind: 'builder',
    model: AUTOMATE_AI_CHAT_MODEL,
    query,
    title: query,
    status: 'running',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    messages: [
      {
        role: 'assistant',
        content: `What should "${query}" do? Tell me the exact behavior, inputs, and what UI you want, then I'll build it.`,
      },
    ],
  };
}

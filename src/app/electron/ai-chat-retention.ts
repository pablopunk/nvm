export const AI_CONVERSATION_RETENTION_MS = 24 * 60 * 60 * 1000;

type AiChat = {
  kind?: unknown;
  createdAt?: unknown;
  updatedAt?: unknown;
};

function timestamp(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

export function expiredConversationAiChatIds(
  chats: Record<string, AiChat>,
  now = Date.now(),
) {
  const cutoff = now - AI_CONVERSATION_RETENTION_MS;
  return Object.entries(chats).flatMap(([id, chat]) => {
    if (chat?.kind !== 'conversation') return [];
    const lastActivity = timestamp(chat.updatedAt) ?? timestamp(chat.createdAt);
    return lastActivity !== null && lastActivity <= cutoff ? [id] : [];
  });
}

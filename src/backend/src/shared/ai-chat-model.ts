export type AiChatModel = 'fast' | 'smart';

export const DEFAULT_AI_CHAT_MODEL: AiChatModel = 'fast';
export const AUTOMATE_AI_CHAT_MODEL: AiChatModel = 'smart';

export function isAiChatModel(value: unknown): value is AiChatModel {
  return value === 'fast' || value === 'smart';
}

export function normalizeAiChatModel(
  value: unknown,
  fallback: AiChatModel = DEFAULT_AI_CHAT_MODEL,
) {
  return isAiChatModel(value) ? value : fallback;
}

export function aiChatModelForChat(
  chat:
    | {
        kind?: unknown;
        model?: unknown;
      }
    | null
    | undefined,
) {
  return chat?.kind === 'conversation'
    ? normalizeAiChatModel(chat.model)
    : AUTOMATE_AI_CHAT_MODEL;
}

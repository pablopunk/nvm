export type ShortcutActionRecord = { aiChatId?: string };

export function buildShortcutByAiChatIdMap(
  shortcutActions: Record<string, ShortcutActionRecord | null | undefined>,
  shortcuts: Record<string, string | null | undefined>,
  aiChats: Record<string, unknown>,
  touchedFilesForChat: (chat: unknown) => string[],
) {
  const shortcutsByChat = new Map<string, string[]>();
  for (const [actionId, storedAction] of Object.entries(
    shortcutActions || {},
  )) {
    if (storedAction?.aiChatId && shortcuts?.[actionId]) {
      shortcutsByChat.set(storedAction.aiChatId, [
        ...(shortcutsByChat.get(storedAction.aiChatId) || []),
        String(shortcuts[actionId]),
      ]);
    }
  }

  const map = new Map<string, string>();
  for (const [chatId, chatShortcuts] of shortcutsByChat) {
    if (
      chatShortcuts.length === 1 &&
      touchedFilesForChat(aiChats?.[chatId]).length === 1
    )
      map.set(chatId, chatShortcuts[0]);
  }
  return map;
}

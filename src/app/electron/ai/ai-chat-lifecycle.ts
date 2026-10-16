// biome-ignore-all lint: AI chat records and list items are dynamic host-side payloads.
import { wrapWithConfirmation } from '../extension-host/extension-action-builders';
import { warn as logWarn } from '../platform/logger';
import type { UserStateStore } from '../state/user-state-store';
import { isConversationAiChat } from './ai-chat-records';
import { expiredConversationAiChatIds } from './ai-chat-retention';
import {
  AI_BUILDER_EXTENSION_ID,
  buildAiBuilderAction,
} from './extension-ai-builder';

type AnyRecord = Record<string, any>;

export type PendingAiChatSend = {
  aborted: boolean;
  started: boolean;
  traceId?: string;
  settled: Promise<void>;
  resolveSettled: () => void;
};

export type ActiveConversationRequest = {
  abort: () => void;
  result: Promise<unknown>;
};

export type AiChatLifecycleDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  draftAiChats: Map<string, AnyRecord>;
  pendingAiChatSends: Map<string, PendingAiChatSend>;
  activeConversationRequests: Map<string, ActiveConversationRequest>;
  activeConversationSessionIds: Set<string>;
  activeBuilderSessionIds: Set<string>;
  extensionActionRegistry: Map<string, any>;
  getActiveAiChatId: () => string | undefined;
  getNevermindAi: () => any;
  isPaletteVisible: () => boolean | undefined;
  broadcastAiChatEvent: (event: any) => void;
  scheduleSaveState: () => void;
  invalidateExtensionRootItems: () => void;
  patchOpenView: (viewId: string, patch: any) => void;
  normalizeViewItems: (items: any[], entry: any) => any[];
  createAiBuilderExtension: () => any;
  aiChatView: (item: any, options?: any) => Promise<unknown>;
  removeAiChatImageDirectory: (chatId: string) => Promise<void>;
};

export function createAiChatLifecycle(deps: AiChatLifecycleDeps) {
  const {
    userStateStore,
    draftAiChats,
    pendingAiChatSends,
    activeConversationRequests,
    activeConversationSessionIds,
    activeBuilderSessionIds,
    extensionActionRegistry,
    getActiveAiChatId,
    getNevermindAi,
    isPaletteVisible,
    broadcastAiChatEvent,
    scheduleSaveState,
    invalidateExtensionRootItems,
    patchOpenView,
    normalizeViewItems,
    createAiBuilderExtension,
    aiChatView,
    removeAiChatImageDirectory,
  } = deps;

  function aiChatOpenAction(chatId) {
    return buildAiBuilderAction('Open Chat', async () => {
      const item =
        userStateStore.get().aiChats[chatId] || draftAiChats.get(chatId);
      if (!item)
        return { toast: { message: 'AI chat not found', tone: 'error' } };
      return { view: await aiChatView(item) };
    });
  }

  function aiChatRemoveAction(chat) {
    return wrapWithConfirmation(
      buildAiBuilderAction('Remove Chat', () => removeAiChat(chat.id), {
        style: 'destructive',
      }),
      {
        message: isConversationAiChat(chat)
          ? `Remove "${chat.title || chat.query || 'AI chat'}" and its history?`
          : `Remove "${chat.title || chat.query || 'AI chat'}" and its history? Generated extension files stay.`,
        confirmLabel: 'Remove Chat',
        destructive: true,
      },
    );
  }

  function aiChatListItem(chat: any) {
    const conversation = isConversationAiChat(chat);
    return {
      id: `ai-chat:${chat.id}`,
      title: chat.title || chat.query || 'AI Chat',
      subtitle: conversation
        ? 'AI conversation'
        : chat.contextExtensionFile ||
          (chat.touchedExtensionFiles || [])[0] ||
          chat.status ||
          'Builder chat',
      icon: conversation ? 'message-circle' : 'sparkles',
      primaryAction: aiChatOpenAction(chat.id),
      actions: [aiChatRemoveAction(chat)],
    };
  }

  function aiChatListItems() {
    const chats = Object.values(userStateStore.get().aiChats || {}) as any[];
    return chats
      .sort(
        (a, b) =>
          (b.updatedAt || b.createdAt || 0) - (a.updatedAt || a.createdAt || 0),
      )
      .map(aiChatListItem);
  }

  function aiChatsView() {
    return {
      type: 'list',
      id: 'ai-chats',
      title: 'AI Chats',
      searchBarPlaceholder: 'Search AI Chats',
      items: aiChatListItems(),
    };
  }

  function aiBuilderRegistryEntry() {
    return (
      extensionActionRegistry.get(`${AI_BUILDER_EXTENSION_ID}:ai-chats`) || {
        extension: createAiBuilderExtension(),
        command: { id: 'ai-chats', title: 'AI Chats' },
      }
    );
  }

  function normalizedAiChatListItem(chat: any) {
    return normalizeViewItems(
      [aiChatListItem(chat)],
      aiBuilderRegistryEntry(),
    )[0];
  }

  function patchAiChatsItem(chatId: string) {
    const chat = userStateStore.get().aiChats[chatId];
    if (!chat) return;
    patchOpenView('ai-chats', {
      mode: 'patch',
      items: [normalizedAiChatListItem(chat)],
    });
  }

  function patchAiChatsPrepend(chatId: string) {
    const chat = userStateStore.get().aiChats[chatId];
    if (!chat) return;
    patchOpenView('ai-chats', {
      mode: 'prepend',
      items: [normalizedAiChatListItem(chat)],
    });
  }

  function patchAiChatsRemove(chatId: string) {
    patchOpenView('ai-chats', { removeItemIds: [`ai-chat:${chatId}`] });
  }

  async function abortAiChat(chatId) {
    const targetChatId = chatId || getActiveAiChatId();
    if (!targetChatId) return;
    const chat =
      userStateStore.get().aiChats[targetChatId] ||
      draftAiChats.get(targetChatId);
    const pending = pendingAiChatSends.get(targetChatId);
    if (pending && !pending.started) {
      pending.aborted = true;
      const current = userStateStore.get().aiChats[targetChatId];
      if (current) {
        current.status = 'aborted';
        current.updatedAt = Date.now();
        patchAiChatsItem(targetChatId);
        scheduleSaveState();
      }
      broadcastAiChatEvent({
        type: 'aborted',
        chatId: targetChatId,
        traceId: pending.traceId,
      });
      return;
    }
    if (pending) pending.aborted = true;
    if (isConversationAiChat(chat))
      return activeConversationRequests.get(targetChatId)?.abort();
    return getNevermindAi()?.abort(targetChatId);
  }

  async function resetAiChat(chatId) {
    const targetChatId = chatId || getActiveAiChatId();
    if (!targetChatId) return;
    const chat =
      userStateStore.get().aiChats[targetChatId] ||
      draftAiChats.get(targetChatId);
    const pending = pendingAiChatSends.get(targetChatId);
    const conversationRequest = activeConversationRequests.get(targetChatId);
    await abortAiChat(targetChatId);
    await pending?.settled;
    if (isConversationAiChat(chat)) {
      await conversationRequest?.result.catch(() => {});
      activeConversationSessionIds.delete(targetChatId);
      return getNevermindAi()
        ?.session(targetChatId, { toolMode: 'conversation' })
        .reset();
    }
    activeBuilderSessionIds.delete(targetChatId);
    return getNevermindAi()?.reset(targetChatId);
  }

  async function removeAiChat(chatId) {
    if (!(chatId && userStateStore.get().aiChats[chatId]))
      return { toast: { message: 'AI chat not found', tone: 'error' } };
    await resetAiChat(chatId);
    const chat = userStateStore.get().aiChats[chatId];
    // INVARIANT: removing a chat deletes only conversation history and AI session state.
    // It must NEVER unlink generated extension files. Generated extensions are durable
    // artifacts owned by chats via touchedExtensionFiles; chat removal preserves them so
    // the user can keep the extension after discarding the conversation that built it.
    delete userStateStore.get().aiChats[chatId];
    await removeAiChatImageDirectory(chatId).catch((error) =>
      logWarn(
        'aiChat.images.remove.failed',
        {
          chatId,
          error: error instanceof Error ? error.message : String(error),
        },
        { source: 'host', scope: 'ai' },
      ),
    );
    for (const actionId of Object.keys(userStateStore.get().recents || {})) {
      if (actionId === `ai-chat:${chatId}`)
        delete userStateStore.get().recents[actionId];
    }
    scheduleSaveState();
    invalidateExtensionRootItems();
    patchAiChatsRemove(chatId);
    return {
      toast: { message: `Removed ${chat.title || chat.query || 'AI chat'}` },
    };
  }

  async function cleanupExpiredAiConversations() {
    if (isPaletteVisible()) return 0;
    const expiredChatIds = expiredConversationAiChatIds(
      userStateStore.get().aiChats || {},
    );
    const removableChatIds = expiredChatIds.filter(
      (chatId) =>
        chatId !== getActiveAiChatId() &&
        !pendingAiChatSends.has(chatId) &&
        !activeConversationRequests.has(chatId),
    );
    if (!removableChatIds.length) return 0;

    const removedChatIds: string[] = [];
    for (const chatId of removableChatIds) {
      try {
        await getNevermindAi()
          ?.session(chatId, { toolMode: 'conversation' })
          .reset();
      } catch (error) {
        logWarn(
          'aiChat.session.reset.failed',
          {
            chatId,
            error: error instanceof Error ? error.message : String(error),
          },
          { source: 'host', scope: 'ai' },
        );
        continue;
      }
      try {
        await removeAiChatImageDirectory(chatId);
      } catch (error) {
        logWarn(
          'aiChat.images.remove.failed',
          {
            chatId,
            error: error instanceof Error ? error.message : String(error),
          },
          { source: 'host', scope: 'ai' },
        );
        continue;
      }
      delete userStateStore.get().aiChats[chatId];
      activeConversationSessionIds.delete(chatId);
      activeBuilderSessionIds.delete(chatId);
      delete userStateStore.get().recents?.[`ai-chat:${chatId}`];
      patchAiChatsRemove(chatId);
      removedChatIds.push(chatId);
    }
    if (!removedChatIds.length) return 0;
    scheduleSaveState();
    invalidateExtensionRootItems();
    return removedChatIds.length;
  }

  return {
    aiChatsView,
    patchAiChatsItem,
    patchAiChatsPrepend,
    abortAiChat,
    resetAiChat,
    removeAiChat,
    cleanupExpiredAiConversations,
  };
}

// biome-ignore-all lint: AI chat records are dynamic persisted payloads.
import type { UserStateStore } from '../state/user-state-store';
import type {
  ActiveConversationRequest,
  PendingAiChatSend,
} from './ai-chat-lifecycle';
import { isConversationAiChat } from './ai-chat-records';

export type AiChatSessionTeardownDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  pendingAiChatSends: Map<string, PendingAiChatSend>;
  activeConversationRequests: Map<string, ActiveConversationRequest>;
  activeConversationSessionIds: Set<string>;
  activeBuilderSessionIds: Set<string>;
  getNevermindAi: () => any;
  getActiveAiChatId: () => string | undefined;
  setActiveAiChatId: (chatId: string | undefined) => void;
  abortAiChat: (chatId: string) => Promise<unknown>;
  recordLearningReview: (chatId: string) => void;
};

export function createAiChatSessionTeardown(deps: AiChatSessionTeardownDeps) {
  const {
    userStateStore,
    pendingAiChatSends,
    activeConversationRequests,
    activeConversationSessionIds,
    activeBuilderSessionIds,
    getNevermindAi,
    getActiveAiChatId,
    setActiveAiChatId,
    abortAiChat,
    recordLearningReview,
  } = deps;

  async function disposeAiSessions() {
    const pending = Array.from(pendingAiChatSends.entries());
    const conversations = Array.from(activeConversationRequests.values());
    await Promise.allSettled(pending.map(([chatId]) => abortAiChat(chatId)));
    for (const request of conversations) request.abort();
    await Promise.allSettled(pending.map(([, request]) => request.settled));
    await Promise.allSettled(conversations.map((request) => request.result));
    activeConversationRequests.clear();
    activeConversationSessionIds.clear();
    activeBuilderSessionIds.clear();
    await getNevermindAi()?.disposeAllSessions?.();
  }

  async function noteAiChatExited(chatId) {
    if (!chatId) return;
    if (isConversationAiChat(userStateStore.get().aiChats[chatId])) {
      if (getActiveAiChatId() === chatId) setActiveAiChatId(undefined);
      return;
    }
    recordLearningReview(chatId);
  }

  return { disposeAiSessions, noteAiChatExited };
}

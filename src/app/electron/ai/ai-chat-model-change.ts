// biome-ignore-all lint: AI chat records are dynamic persisted payloads.
import { aiChatModelForChat, isAiChatModel } from '../../shared/ai-chat-model';
import { getByoKey } from '../backend/byo-key';
import type { UserStateStore } from '../state/user-state-store';
import { isConversationAiChat } from './ai-chat-records';
import type {
  ActiveConversationRequest,
  PendingAiChatSend,
} from './ai-chat-lifecycle';

type AnyRecord = Record<string, any>;

export type AiChatModelChangeDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  draftAiChats: Map<string, AnyRecord>;
  pendingAiChatSends: Map<string, PendingAiChatSend>;
  activeConversationRequests: Map<string, ActiveConversationRequest>;
  activeConversationSessionIds: Set<string>;
  pendingAiChatModelChanges: Set<string>;
  getNevermindAi: () => any;
  scheduleSaveState: () => void;
  patchAiChatsItem: (chatId: string) => void;
  broadcastAiChatEvent: (event: any) => void;
};

export function createAiChatModelChange(deps: AiChatModelChangeDeps) {
  const {
    userStateStore,
    draftAiChats,
    pendingAiChatSends,
    activeConversationRequests,
    activeConversationSessionIds,
    pendingAiChatModelChanges,
    getNevermindAi,
    scheduleSaveState,
    patchAiChatsItem,
    broadcastAiChatEvent,
  } = deps;

  let aiChatModelChangeQueue = Promise.resolve();

  async function setAiChatModel(chatId, model: unknown) {
    const targetChatId = typeof chatId === 'string' ? chatId : '';
    if (!targetChatId) throw new Error('AI chat not found');
    if (!isAiChatModel(model)) throw new Error('Unsupported AI chat model');
    const chat =
      userStateStore.get().aiChats[targetChatId] ||
      draftAiChats.get(targetChatId);
    if (!isConversationAiChat(chat))
      throw new Error('Only Tab conversations can change models');
    if (await getByoKey())
      throw new Error('Fast and Smart are unavailable with a BYO model');
    if (
      pendingAiChatSends.has(targetChatId) ||
      activeConversationRequests.has(targetChatId)
    )
      throw new Error('Finish the current response before changing models');

    if (pendingAiChatModelChanges.has(targetChatId))
      throw new Error('The AI chat model is already changing');
    const previousModel = aiChatModelForChat(chat);
    if (previousModel === model) return;
    pendingAiChatModelChanges.add(targetChatId);
    const previousChange = aiChatModelChangeQueue;
    let releaseChange = () => {};
    aiChatModelChangeQueue = new Promise<void>((resolve) => {
      releaseChange = () => resolve();
    });
    try {
      await previousChange;
      await getNevermindAi()
        ?.session(targetChatId, {
          toolMode: 'conversation',
        })
        .reset();
      activeConversationSessionIds.delete(targetChatId);
      chat.model = model;
      chat.updatedAt = Date.now();
      userStateStore.get().aiChatDefaultModel = model;
      scheduleSaveState();
      if (userStateStore.get().aiChats[targetChatId])
        patchAiChatsItem(targetChatId);
      broadcastAiChatEvent({
        type: 'model_changed',
        chatId: targetChatId,
        data: { model },
      });
    } finally {
      releaseChange();
      pendingAiChatModelChanges.delete(targetChatId);
    }
  }

  return { setAiChatModel };
}

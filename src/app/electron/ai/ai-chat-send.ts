// biome-ignore-all lint: AI chat records and streamed events are dynamic payloads.
import { aiChatModelForChat } from '../../shared/ai-chat-model';
import { performanceTraces } from '../diagnostics/debug-performance';
import { operationOutcome } from '../diagnostics/observability';
import type { UserStateStore } from '../state/user-state-store';
import {
  type NormalizedAiChatImage,
  normalizeAiChatImages,
} from './ai-chat-images';
import type {
  ActiveConversationRequest,
  PendingAiChatSend,
} from './ai-chat-lifecycle';
import {
  aiChatPromptMessage,
  CONVERSATION_SYSTEM_PROMPT,
  piImages,
} from './ai-chat-prompts';
import { isBuilderAiChat, isConversationAiChat } from './ai-chat-records';

export type AiChatSendDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  pendingAiChatSends: Map<string, PendingAiChatSend>;
  activeConversationRequests: Map<string, ActiveConversationRequest>;
  activeConversationSessionIds: Set<string>;
  activeBuilderSessionIds: Set<string>;
  pendingAiChatModelChanges: Set<string>;
  getNevermindAi: () => any;
  setActiveAiChatId: (chatId: string | undefined) => void;
  initNevermindAi: () => Promise<void>;
  promoteDraftAiChat: (chatId: string) => unknown;
  appendAiChatMessage: (
    chatId: string,
    role: string,
    content: string,
    images?: any[],
  ) => unknown;
  appendAiChatDelta: (chatId: string, text: string) => void;
  patchAiChatsItem: (chatId: string) => void;
  persistAiChatImages: (
    chatId: string,
    images: NormalizedAiChatImage[],
  ) => Promise<any[]>;
  aiChatMessageForRenderer: (message: any) => unknown;
  aiChatPromptWithContext: (message: string, chatId: string) => string;
  conversationPromptWithContext: (message: string, chatId: string) => string;
  restoredConversationImages: (
    chat: any,
    currentImages: NormalizedAiChatImage[],
  ) => Promise<Array<{ type: 'image'; data: string; mimeType: string }>>;
  registerAiEventActionForRenderer: (event: any) => any;
  broadcastAiChatEvent: (event: any) => void;
  scheduleSaveState: () => void;
};

export function createAiChatSend(deps: AiChatSendDeps) {
  const {
    userStateStore,
    pendingAiChatSends,
    activeConversationRequests,
    activeConversationSessionIds,
    activeBuilderSessionIds,
    pendingAiChatModelChanges,
    getNevermindAi,
    setActiveAiChatId,
    initNevermindAi,
    promoteDraftAiChat,
    appendAiChatMessage,
    appendAiChatDelta,
    patchAiChatsItem,
    persistAiChatImages,
    aiChatMessageForRenderer,
    aiChatPromptWithContext,
    conversationPromptWithContext,
    restoredConversationImages,
    registerAiEventActionForRenderer,
    broadcastAiChatEvent,
    scheduleSaveState,
  } = deps;

  let aiChatSendSuspensionCount = 0;

  function beginAiChatSendSuspension() {
    aiChatSendSuspensionCount += 1;
  }

  function endAiChatSendSuspension() {
    aiChatSendSuspensionCount -= 1;
  }

  async function sendAiChatMessage(message, chatId, traceId, images) {
    const normalizedImages = normalizeAiChatImages(images);
    const userMessage = typeof message === 'string' ? message.trim() : '';
    const requestTraceId = typeof traceId === 'string' ? traceId : undefined;
    if (!(userMessage || normalizedImages.length))
      throw new Error('Enter a message or attach an image');
    const promptMessage = aiChatPromptMessage(
      userMessage,
      normalizedImages.length,
    );
    return performanceTraces.run(
      'ai.chat',
      {
        messageLength: userMessage.length,
        imageCount: normalizedImages.length,
      },
      async () => {
        if (aiChatSendSuspensionCount > 0)
          throw new Error('AI is unavailable while the backend is switching');
        const requestedChatId = typeof chatId === 'string' ? chatId : '';
        if (!requestedChatId) throw new Error('AI chat not found');
        if (requestedChatId.startsWith('draft:'))
          promoteDraftAiChat(requestedChatId);
        const targetChatId = requestedChatId;
        const chat = userStateStore.get().aiChats[targetChatId];
        if (!(isBuilderAiChat(chat) || isConversationAiChat(chat)))
          throw new Error('AI chat not found');
        if (pendingAiChatModelChanges.has(targetChatId))
          throw new Error('The AI chat model is changing');
        setActiveAiChatId(targetChatId);
        if (
          pendingAiChatSends.has(targetChatId) ||
          (isConversationAiChat(chat) &&
            activeConversationRequests.has(targetChatId))
        )
          throw new Error('This AI chat is already responding');
        let resolveSettled = () => {};
        const settled = new Promise<void>((resolve) => {
          resolveSettled = resolve;
        });
        const pending = {
          aborted: false,
          started: false,
          ...(requestTraceId ? { traceId: requestTraceId } : {}),
          settled,
          resolveSettled,
        };
        pendingAiChatSends.set(targetChatId, pending);
        const conversation = isConversationAiChat(chat);
        let prompt = conversation
          ? activeConversationSessionIds.has(targetChatId)
            ? promptMessage
            : conversationPromptWithContext(promptMessage, targetChatId)
          : activeBuilderSessionIds.has(targetChatId)
            ? promptMessage
            : aiChatPromptWithContext(promptMessage, targetChatId);
        try {
          if (!getNevermindAi()) await initNevermindAi();
          let modelImages = piImages(normalizedImages);
          if (conversation && !activeConversationSessionIds.has(targetChatId)) {
            const restoredImages = await restoredConversationImages(
              chat,
              normalizedImages,
            );
            if (restoredImages.length) {
              prompt = `Recent earlier attached images are included first, in transcript order. Images for the new message are last.\n\n${prompt}`;
              modelImages = [...restoredImages, ...modelImages];
            }
          }
          const storedImages = await persistAiChatImages(
            targetChatId,
            normalizedImages,
          );
          const storedMessage = appendAiChatMessage(
            targetChatId,
            'user',
            userMessage,
            storedImages,
          );
          if (!storedMessage) throw new Error('AI chat not found');
          broadcastAiChatEvent({
            type: 'user_message',
            chatId: targetChatId,
            traceId: requestTraceId,
            data: { message: aiChatMessageForRenderer(storedMessage) },
          });
          if (pending.aborted) {
            operationOutcome('cancelled');
            return;
          }
          pending.started = true;
          if (conversation)
            return await sendConversationMessage(
              prompt,
              targetChatId,
              requestTraceId,
              modelImages,
            );
          activeBuilderSessionIds.add(targetChatId);
          return await getNevermindAi().send(
            prompt,
            targetChatId,
            requestTraceId,
            modelImages,
          );
        } finally {
          if (pendingAiChatSends.get(targetChatId) === pending)
            pendingAiChatSends.delete(targetChatId);
          pending.resolveSettled();
        }
      },
      requestTraceId ? { traceId: requestTraceId } : undefined,
    );
  }

  async function sendConversationMessage(
    prompt,
    chatId,
    traceId,
    images: Array<{ type: 'image'; data: string; mimeType: string }> = [],
  ) {
    const chat = userStateStore.get().aiChats[chatId];
    if (!isConversationAiChat(chat)) throw new Error('AI chat not found');
    if (activeConversationRequests.has(chatId))
      throw new Error('This AI conversation is already responding');
    let terminalEvent: string | undefined;
    let responseActivity = false;
    const session = getNevermindAi().session(chatId, {
      system: CONVERSATION_SYSTEM_PROMPT,
      model: aiChatModelForChat(chat),
      toolMode: 'conversation',
    });
    const request = session.stream(prompt, {
      images,
      onEvent(event: any) {
        const terminal = ['done', 'error', 'aborted'].includes(event.type);
        if (terminal && terminalEvent) return;
        if (terminal) terminalEvent = event.type;
        if (event.type === 'delta' || event.type === 'tool_start')
          responseActivity = true;
        if (event.type === 'delta' && event.text)
          appendAiChatDelta(chatId, event.text);
        if (event.type === 'error' && event.message)
          appendAiChatMessage(chatId, 'system', event.message);
        const current = userStateStore.get().aiChats[chatId];
        if (current && event.type === 'start') current.status = 'running';
        if (current && terminal) {
          current.status = event.type === 'done' ? 'done' : event.type;
          current.updatedAt = Date.now();
          patchAiChatsItem(chatId);
          scheduleSaveState();
        }
        broadcastAiChatEvent(
          registerAiEventActionForRenderer({
            ...event,
            chatId,
            traceId,
          }),
        );
      },
    });
    activeConversationSessionIds.add(chatId);
    activeConversationRequests.set(chatId, request);
    try {
      await request.result;
    } catch {
      // The stream reports errors and cancellation through onEvent.
    } finally {
      if (activeConversationRequests.get(chatId) === request)
        activeConversationRequests.delete(chatId);
      if (
        !responseActivity &&
        (terminalEvent === 'error' || terminalEvent === 'aborted')
      ) {
        activeConversationSessionIds.delete(chatId);
        await session.reset().catch(() => {});
      }
    }
  }

  return {
    sendAiChatMessage,
    beginAiChatSendSuspension,
    endAiChatSendSuspension,
  };
}

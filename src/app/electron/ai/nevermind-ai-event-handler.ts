// biome-ignore-all lint: AI events and chat records are dynamic host-side payloads.
import path from 'node:path';
import type { LocalLearningStore } from '../state/learning-store';
import type { UserStateStore } from '../state/user-state-store';
import { isBuilderAiChat } from './ai-chat-records';

type AnyRecord = Record<string, any>;

export type NevermindAiEvent = {
  type: string;
  chatId?: string;
  text?: string;
  message?: string;
  name?: string;
  label?: string;
  data?: unknown;
  isError?: boolean;
  traceId?: string;
};

export type NevermindAiEventHandlerDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  draftAiChats: Map<string, AnyRecord>;
  activeBuilderSessionIds: Set<string>;
  getActiveAiChatId: () => string | undefined;
  getNevermindAi: () => any;
  getLearningStore: () => LocalLearningStore | null;
  aiLearningMetadata: (chatId: string) => any;
  appendAiChatDelta: (chatId: string, text: string) => void;
  appendAiChatMessage: (
    chatId: string,
    role: string,
    content: string,
  ) => unknown;
  patchAiChatsItem: (chatId: string) => void;
  scheduleSaveState: () => void;
  registerAiEventActionForRenderer: (event: any) => any;
  broadcastAiChatEvent: (event: any) => void;
};

export function createNevermindAiEventHandler(
  deps: NevermindAiEventHandlerDeps,
) {
  const {
    userStateStore,
    draftAiChats,
    activeBuilderSessionIds,
    getActiveAiChatId,
    getNevermindAi,
    getLearningStore,
    aiLearningMetadata,
    appendAiChatDelta,
    appendAiChatMessage,
    patchAiChatsItem,
    scheduleSaveState,
    registerAiEventActionForRenderer,
    broadcastAiChatEvent,
  } = deps;

  function recordStreamProgress(
    chatId: string | undefined,
    event: NevermindAiEvent,
    metadata: any,
  ) {
    if (chatId && event.type === 'start')
      getLearningStore()?.recordStatus(chatId, 'start', metadata);
    if (chatId && event.type === 'delta' && event.text)
      appendAiChatDelta(chatId, event.text);
  }

  function recordToolTrace(
    chatId: string | undefined,
    event: NevermindAiEvent,
    metadata: any,
  ) {
    if (chatId && event.type === 'tool_trace_start' && event.name)
      getLearningStore()?.recordToolStart(
        chatId,
        event.name,
        (event.data as any)?.input,
        metadata,
        (event.data as any)?.toolCallId,
      );
    if (chatId && event.type === 'tool_trace_end' && event.name) {
      const toolData = event.data as any;
      getLearningStore()?.recordToolEnd(
        chatId,
        event.name,
        {
          ok: !event.isError,
          outputSummary: toolData?.output,
          error: toolData?.error,
          toolCallId: toolData?.toolCallId,
        },
        metadata,
      );
      if (
        [
          'write_extension',
          'validate_extension',
          'remove_extension',
          'install_extension',
        ].includes(event.name)
      ) {
        getLearningStore()?.recordExtensionEvent(
          chatId,
          {
            kind: event.name as
              | 'write_extension'
              | 'validate_extension'
              | 'remove_extension'
              | 'install_extension',
            filename: toolData?.output?.filePath || toolData?.input?.filename,
            extensionId: toolData?.output?.extensionId,
            commandIds: Array.isArray(toolData?.output?.commandIds)
              ? toolData.output.commandIds
              : undefined,
            ok: !event.isError,
            error: toolData?.error,
            details: toolData?.output,
          },
          metadata,
        );
      }
    }
  }

  function recordStreamError(
    chatId: string | undefined,
    event: NevermindAiEvent,
  ) {
    if (chatId && event.type === 'error' && event.message)
      appendAiChatMessage(chatId, 'system', event.message);
    if (
      chatId &&
      event.type === 'error' &&
      /prompt[_ -]?too[_ -]?large|prompt exceeds|413\b/i.test(
        String(event.message || ''),
      )
    ) {
      activeBuilderSessionIds.delete(chatId);
      void getNevermindAi()
        ?.reset(chatId)
        .catch(() => {});
    }
  }

  function recordExtensionActivated(
    chatId: string | undefined,
    event: NevermindAiEvent,
  ) {
    if (chatId && event.type === 'extension_activated' && event.data) {
      const preview = event.data as any;
      if (preview.filename && preview.preview) {
        const chat =
          userStateStore.get().aiChats[chatId] || draftAiChats.get(chatId);
        if (chat) {
          chat.builderPreviewFiles = Array.from(
            new Set([
              ...(chat.builderPreviewFiles || []),
              path.basename(preview.filename),
            ]),
          );
          chat.selectedBuilderPreviewFilename = path.basename(preview.filename);
          scheduleSaveState();
        }
      }
    }
  }

  function recordTerminalStatus(
    chatId: string | undefined,
    event: NevermindAiEvent,
    metadata: any,
  ) {
    if (
      chatId &&
      event.type === 'done' &&
      userStateStore.get().aiChats[chatId]
    ) {
      if (userStateStore.get().aiChats[chatId].status !== 'ready')
        userStateStore.get().aiChats[chatId].status = 'done';
      userStateStore.get().aiChats[chatId].updatedAt = Date.now();
      getLearningStore()?.recordStatus(chatId, 'done', metadata);
      patchAiChatsItem(chatId);
      scheduleSaveState();
    }
    if (chatId && event.type === 'error' && event.message) {
      getLearningStore()?.recordStatus(
        chatId,
        'error',
        metadata,
        event.message,
      );
    }
    if (chatId && event.type === 'aborted') {
      getLearningStore()?.recordStatus(chatId, 'aborted', metadata);
      const current = userStateStore.get().aiChats[chatId];
      if (current) {
        current.status = 'aborted';
        current.updatedAt = Date.now();
        patchAiChatsItem(chatId);
        scheduleSaveState();
      }
    }
  }

  function handleNevermindAiEvent(event: NevermindAiEvent) {
    const chatId = event.chatId || getActiveAiChatId();
    const chat = chatId
      ? userStateStore.get().aiChats[chatId] || draftAiChats.get(chatId)
      : null;
    if (!isBuilderAiChat(chat)) return;
    const metadata = chatId ? aiLearningMetadata(chatId) : undefined;
    recordStreamProgress(chatId, event, metadata);
    recordToolTrace(chatId, event, metadata);
    recordStreamError(chatId, event);
    recordExtensionActivated(chatId, event);
    recordTerminalStatus(chatId, event, metadata);
    broadcastAiChatEvent(
      registerAiEventActionForRenderer({
        ...event,
        chatId,
      }),
    );
  }

  return { handleNevermindAiEvent };
}

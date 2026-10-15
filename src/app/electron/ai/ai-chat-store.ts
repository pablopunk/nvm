// biome-ignore-all lint: AI chat records are dynamic persisted payloads.
import crypto from 'node:crypto';
import path from 'node:path';
import {
  AUTOMATE_AI_CHAT_MODEL,
  normalizeAiChatModel,
} from '../../shared/ai-chat-model';
import { hashValue } from '../search/search-utils';
import type { LocalLearningStore } from '../state/learning-store';
import type { UserStateStore } from '../state/user-state-store';
import {
  aiChatItem,
  chatTouchedExtensionFiles,
  isBuilderAiChat,
} from './ai-chat-records';

type AnyRecord = Record<string, any>;

export type AiChatLearningTrace = Pick<
  LocalLearningStore,
  'upsertTraceMetadata' | 'appendMessage' | 'appendAssistantDelta'
>;

export type AiChatStoreDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  draftAiChats: Map<string, AnyRecord>;
  getLearningStore: () => AiChatLearningTrace | null;
  scheduleSaveState: () => void;
  invalidateExtensionRootItems: () => void;
  patchAiChatsItem: (chatId: string) => void;
  patchAiChatsPrepend: (chatId: string) => void;
  removeStoredAiChatMessageImages: (messages: any[]) => void;
};

export function createAiChatStore(deps: AiChatStoreDeps) {
  const {
    userStateStore,
    draftAiChats,
    getLearningStore,
    scheduleSaveState,
    invalidateExtensionRootItems,
    patchAiChatsItem,
    patchAiChatsPrepend,
    removeStoredAiChatMessageImages,
  } = deps;

  function defaultAiChatModel() {
    return normalizeAiChatModel(userStateStore.get().aiChatDefaultModel);
  }

  function aiLearningMetadata(chatId: string) {
    const chat =
      userStateStore.get().aiChats[chatId] || draftAiChats.get(chatId);
    return {
      query: chat?.query,
      title: chat?.title,
      contextExtensionFile: chat?.contextExtensionFile,
      extensionFiles: chatTouchedExtensionFiles(chat),
    };
  }

  function getOrCreateAiChat(query, options: any = {}) {
    const trimmed = query.trim();
    const baseId = hashValue(trimmed);
    const current = userStateStore.get().aiChats[baseId];
    if (current && !options.fresh) return current;
    const id =
      current && options.fresh
        ? hashValue(`${trimmed}:${Date.now()}:${crypto.randomUUID()}`)
        : baseId;
    const item = aiChatItem(id, trimmed);
    userStateStore.get().aiChats[id] = item;
    getLearningStore()?.upsertTraceMetadata(id, aiLearningMetadata(id));
    scheduleSaveState();
    invalidateExtensionRootItems();
    return item;
  }

  function conversationChatItem(id, query) {
    return {
      id,
      kind: 'conversation',
      model: defaultAiChatModel(),
      query,
      title: query,
      status: 'running',
      createdAt: Date.now(),
      updatedAt: Date.now(),
      messages: [],
    };
  }

  function createDraftAiChat(query) {
    const trimmed = query.trim();
    const id = `draft:${hashValue(`${trimmed}:${Date.now()}:${crypto.randomUUID()}`)}`;
    const item = aiChatItem(id, trimmed);
    draftAiChats.set(id, item);
    return item;
  }

  function createDraftConversationChat(query) {
    const trimmed = query.trim();
    const id = `draft:conversation:${crypto.randomUUID()}`;
    const item = conversationChatItem(id, trimmed);
    draftAiChats.set(id, item);
    return item;
  }

  function promoteDraftAiChat(chatId) {
    if (userStateStore.get().aiChats[chatId])
      return userStateStore.get().aiChats[chatId];
    const draft = draftAiChats.get(chatId);
    if (!draft) return null;
    draftAiChats.delete(chatId);
    draft.createdAt = Date.now();
    draft.updatedAt = Date.now();
    userStateStore.get().aiChats[chatId] = draft;
    if (isBuilderAiChat(draft))
      getLearningStore()?.upsertTraceMetadata(
        chatId,
        aiLearningMetadata(chatId),
      );
    scheduleSaveState();
    invalidateExtensionRootItems();
    return draft;
  }

  function appendAiChatMessage(chatId, role, content, images: any[] = []) {
    const chat = userStateStore.get().aiChats[chatId];
    if (!(chat && (content || images.length))) return null;
    const message = { role, content, ...(images.length ? { images } : {}) };
    const messages = [...(chat.messages || []), message];
    removeStoredAiChatMessageImages(messages.slice(0, -100));
    chat.messages = messages.slice(-100);
    chat.updatedAt = Date.now();
    if (isBuilderAiChat(chat) && content)
      getLearningStore()?.appendMessage(
        chatId,
        role,
        content,
        aiLearningMetadata(chatId),
      );
    scheduleSaveState();
    patchAiChatsItem(chatId);
    return message;
  }

  function appendAiChatDelta(chatId, text) {
    const chat = userStateStore.get().aiChats[chatId];
    if (!(chat && text)) return;
    const messages = chat.messages || [];
    const last = messages[messages.length - 1];
    if (last?.role === 'assistant') last.content = `${last.content}${text}`;
    else messages.push({ role: 'assistant', content: text });
    removeStoredAiChatMessageImages(messages.slice(0, -100));
    chat.messages = messages.slice(-100);
    chat.updatedAt = Date.now();
    if (isBuilderAiChat(chat))
      getLearningStore()?.appendAssistantDelta(
        chatId,
        text,
        aiLearningMetadata(chatId),
      );
    scheduleSaveState();
  }

  function getOrCreateExtensionChat(extensionFile, title = extensionFile) {
    const filename = path.basename(extensionFile || '');
    const existing = (
      Object.values(userStateStore.get().aiChats || {}) as any[]
    )
      .filter(
        (chat) =>
          isBuilderAiChat(chat) &&
          (chat.contextExtensionFile === filename ||
            chatTouchedExtensionFiles(chat).includes(filename)),
      )
      .sort(
        (a, b) =>
          (b.updatedAt || b.createdAt || 0) - (a.updatedAt || a.createdAt || 0),
      )[0];
    if (existing) {
      existing.contextExtensionFile = filename;
      existing.updatedAt = Date.now();
      getLearningStore()?.upsertTraceMetadata(
        existing.id,
        aiLearningMetadata(existing.id),
      );
      scheduleSaveState();
      patchAiChatsItem(existing.id);
      return existing;
    }
    const id = hashValue(`extension-chat:${filename}:${Date.now()}`);
    const item = {
      id,
      kind: 'builder',
      model: AUTOMATE_AI_CHAT_MODEL,
      query: `Tweak ${title || filename}`,
      title: `Tweak ${title || filename}`,
      status: 'ready',
      contextExtensionFile: filename,
      touchedExtensionFiles: [filename],
      createdAt: Date.now(),
      updatedAt: Date.now(),
      messages: [
        {
          role: 'assistant',
          content: `I can tweak "${title || filename}". I can read this extension as context and inspect any other extension if needed.`,
        },
      ],
    };
    userStateStore.get().aiChats[id] = item;
    getLearningStore()?.upsertTraceMetadata(id, aiLearningMetadata(id));
    scheduleSaveState();
    invalidateExtensionRootItems();
    patchAiChatsPrepend(id);
    return item;
  }

  return {
    aiLearningMetadata,
    getOrCreateAiChat,
    createDraftAiChat,
    createDraftConversationChat,
    promoteDraftAiChat,
    appendAiChatMessage,
    appendAiChatDelta,
    getOrCreateExtensionChat,
  };
}

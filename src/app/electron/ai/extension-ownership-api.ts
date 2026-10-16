// biome-ignore-all lint: AI chat ownership records are dynamic host-side payloads.
import { AI_BUILDER_EXTENSION_ID } from './extension-ai-builder';

export type ExtensionOwnershipApiDeps = {
  aiChatIdForExtensionFile: (extensionFile: string) => string | null;
  chatCanWriteExtension: (extensionFile: string, chatId?: string) => boolean;
  chatTouchedExtensionFiles: (chat: any) => string[];
  claimExtensionFileForChat: (chat: any, extensionFile: string) => boolean;
  draftAiChats: Map<string, any>;
  getAiChats: () => Record<string, any>;
  loadExtensions: () => Promise<void>;
  registerActionShortcuts: () => void;
  removeGeneratedExtensionForChat: (
    extensionFile: string,
    chatId?: string,
  ) => Promise<{ removed: boolean }>;
  scheduleSaveState: () => void;
};

export function createExtensionOwnershipApiFactory(
  deps: ExtensionOwnershipApiDeps,
) {
  const {
    aiChatIdForExtensionFile,
    chatCanWriteExtension,
    chatTouchedExtensionFiles,
    claimExtensionFileForChat,
    draftAiChats,
    getAiChats,
    loadExtensions,
    registerActionShortcuts,
    removeGeneratedExtensionForChat,
    scheduleSaveState,
  } = deps;

  function createExtensionOwnershipApi(extension) {
    const privileged = extension?.id === AI_BUILDER_EXTENSION_ID;
    const readOnly = {
      ownerOf: (extensionFile) => aiChatIdForExtensionFile(extensionFile),
      filesForChat: (chatId) => {
        const chat = getAiChats()[chatId] || draftAiChats.get(chatId);
        return chat ? chatTouchedExtensionFiles(chat) : [];
      },
      canWrite: (extensionFile, chatId) =>
        chatCanWriteExtension(extensionFile, chatId),
    };
    if (!privileged) return readOnly;
    return {
      ...readOnly,
      claim: (extensionFile, chatId) => {
        const chat = chatId
          ? getAiChats()[chatId] || draftAiChats.get(chatId)
          : null;
        if (!claimExtensionFileForChat(chat, extensionFile)) return false;
        scheduleSaveState();
        return true;
      },
      reload: async () => {
        await loadExtensions();
        registerActionShortcuts();
      },
      remove: async (extensionFile, chatId) => {
        if (!chatCanWriteExtension(extensionFile, chatId)) return false;
        const { removed } = await removeGeneratedExtensionForChat(
          extensionFile,
          chatId,
        );
        return removed;
      },
    };
  }

  return createExtensionOwnershipApi;
}

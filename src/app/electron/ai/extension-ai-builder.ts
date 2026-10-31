// biome-ignore-all lint: AI builder actions and chat records are dynamic host-side payloads.
export const AI_BUILDER_EXTENSION_ID = 'nevermind.ai-builder';

export function assertAiBuilderPrivilege(extension) {
  if (extension?.id !== AI_BUILDER_EXTENSION_ID) {
    throw new Error(
      'ctx.aiBuilder is only available to the built-in AI Builder extension',
    );
  }
}

export function buildAiBuilderAction(title, handler, options: any = {}) {
  return { ...options, type: 'runExtensionAction', title, __handler: handler };
}

export type AiBuilderApiDeps = {
  aiChatView: (item: any, options?: any) => Promise<unknown>;
  aiChatsView: () => unknown;
  chatTouchedExtensionFiles: (chat: any) => string[];
  cleanupExpiredAiConversations: () => Promise<number>;
  createDraftAiChat: (query: string) => any;
  draftAiChats: Map<string, any>;
  getAiChats: () => Record<string, any>;
  getOrCreateExtensionChat: (extensionFile: string, title?: string) => any;
  removeAiChat: (chatId: string) => Promise<unknown>;
};

export function createAiBuilderApiFactory(deps: AiBuilderApiDeps) {
  const {
    aiChatView,
    aiChatsView,
    chatTouchedExtensionFiles,
    cleanupExpiredAiConversations,
    createDraftAiChat,
    draftAiChats,
    getAiChats,
    getOrCreateExtensionChat,
    removeAiChat,
  } = deps;

  function createAiBuilderApi(extension) {
    const privileged = extension?.id === AI_BUILDER_EXTENSION_ID;
    if (!privileged) return;
    return {
      startChat: (input: any = {}) => {
        const prompt = String(input.prompt || input.query || '');
        return buildAiBuilderAction(
          input.title || `Automate "${prompt}"`,
          async () => {
            const item = createDraftAiChat(prompt);
            return {
              view: await aiChatView(item, {
                initialPrompt: prompt || undefined,
              }),
            };
          },
          input.options,
        );
      },
      openChat: (chatId, input: any = {}) =>
        buildAiBuilderAction(
          input.title || 'Open Chat',
          async () => {
            const item = getAiChats()[chatId] || draftAiChats.get(chatId);
            if (!item)
              return { toast: { message: 'AI chat not found', tone: 'error' } };
            return { view: await aiChatView(item) };
          },
          input.options,
        ),
      removeChat: (chatId, input: any = {}) =>
        buildAiBuilderAction(
          input.title || 'Remove Chat',
          async () => removeAiChat(chatId),
          { style: 'destructive', ...(input.options || {}) },
        ),
      tweakExtension: (input: any = {}) =>
        buildAiBuilderAction(
          input.title || 'Tweak with AI',
          async () => {
            const file = input.extensionFile || input.extensionId;
            if (!file)
              return {
                toast: { message: 'No extension specified', tone: 'error' },
              };
            const item = getOrCreateExtensionChat(file, input.title || file);
            return {
              view: await aiChatView(item, { initialPrompt: input.prompt }),
            };
          },
          input.options,
        ),
      openChatsList: (input: any = {}) =>
        buildAiBuilderAction(
          input.title || 'AI Chats',
          async () => ({ view: aiChatsView() }),
          input.options,
        ),
      cleanupExpiredConversations: cleanupExpiredAiConversations,
      listChats: () =>
        Object.values(getAiChats() || {}).map((chat: any) => ({
          id: chat.id,
          title: chat.title || chat.query,
          query: chat.query,
          status: chat.status,
          createdAt: chat.createdAt,
          updatedAt: chat.updatedAt,
          extensionFiles: chatTouchedExtensionFiles(chat),
        })),
      getChat: (chatId) => {
        const chat = getAiChats()[chatId] || draftAiChats.get(chatId);
        if (!chat) return null;
        return {
          id: chat.id,
          title: chat.title || chat.query,
          query: chat.query,
          status: chat.status,
          messages: chat.messages,
          extensionFiles: chatTouchedExtensionFiles(chat),
        };
      },
    };
  }

  return createAiBuilderApi;
}

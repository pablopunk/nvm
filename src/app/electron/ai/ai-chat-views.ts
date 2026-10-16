// biome-ignore-all lint: AI chat views and extension previews are dynamic host-side payloads.
import path from 'node:path';
import { aiChatModelForChat } from '../../shared/ai-chat-model';
import { getByoKey } from '../backend/byo-key';
import { error as logError } from '../platform/logger';
import { aiChatPreviewFiles, prepareAiChatPreview } from './ai-chat-previews';
import { isConversationAiChat } from './ai-chat-records';

export type AiChatViewsDeps = {
  extensionModules: Map<string, any>;
  extensionActionRegistry: Map<string, any>;
  extensionActionFromContribution: (entry: any) => any;
  prepareRootActionForRenderer: (action: any) => any;
  validCachedRootActions: (extension: any) => any[] | null;
  refreshExtensionRootActions: (
    extension: any,
    cacheKey: string,
  ) => Promise<any[]>;
  aiChatMessagesForRenderer: (messages?: any[]) => any[];
};

export function createAiChatViews(deps: AiChatViewsDeps) {
  const {
    extensionModules,
    extensionActionRegistry,
    extensionActionFromContribution,
    prepareRootActionForRenderer,
    validCachedRootActions,
    refreshExtensionRootActions,
    aiChatMessagesForRenderer,
  } = deps;

  async function aiChatView(item, options: any = {}) {
    if (isConversationAiChat(item)) {
      const byo = await getByoKey();
      return {
        type: 'chat',
        title: item.title || item.query || 'Ask AI',
        aiChat: true,
        chatId: item.id,
        aiModel: aiChatModelForChat(item),
        aiModelSelectable: !byo,
        initialPrompt: options.initialPrompt,
        messages: aiChatMessagesForRenderer(item.messages),
      };
    }
    const { files: previewFiles, selectedBuilderPreviewFilename } =
      aiChatPreviewFiles(
        item,
        Array.from(
          extensionModules.values(),
          (extension: any) => extension.__filePath,
        ),
      );
    const builderPreviews = (
      await Promise.all(
        previewFiles.map(async (filename) => {
          try {
            return prepareAiChatPreview(
              await activatedExtensionPreview(filename),
              prepareRootActionForRenderer,
            );
          } catch (error) {
            logError('aiChat.builderPreview.rehydrate.failed', error, {
              source: 'host',
              scope: 'ai',
              extensionId: filename,
            });
            return null;
          }
        }),
      )
    ).filter(Boolean);
    return {
      type: 'chat',
      title: `Automate "${item.query}"`,
      aiChat: true,
      chatId: item.id,
      aiModel: aiChatModelForChat(item),
      initialPrompt: options.initialPrompt,
      messages: aiChatMessagesForRenderer(item.messages),
      builderPreviews,
      selectedBuilderPreviewFilename,
    };
  }

  async function activatedExtensionPreview(filename) {
    const safeName = path.basename(filename);
    const extension = Array.from(extensionModules.values()).find(
      (item: any) => path.basename(item.__filePath || '') === safeName,
    );
    if (!extension)
      throw new Error(`Activated extension ${safeName} is not loaded.`);
    const actionEntries = Array.from(extensionActionRegistry.values()).filter(
      (entry: any) =>
        path.basename(entry.extension?.__filePath || '') === safeName,
    );
    const previewAction = (action) => {
      const prepared = prepareRootActionForRenderer(action);
      return structuredClone(prepared);
    };
    const entry = {
      extension,
      command: { id: 'root', title: extension.title || extension.id },
    };
    const rootItems =
      validCachedRootActions(extension) ||
      (typeof extension.rootItems === 'function'
        ? await refreshExtensionRootActions(
            extension,
            extension.__filePath || extension.id,
          )
        : []);
    const contributions = actionEntries
      .map((item) => ({
        action: extensionActionFromContribution(item),
        source: item.source,
      }))
      .filter((item) => item.action);
    return {
      filename: safeName,
      preview: {
        extensionId: extension.id,
        rootItems: rootItems.map(previewAction).filter(Boolean),
        commands: contributions
          .filter((item) => item.source !== 'action')
          .map((item) => previewAction(item.action)),
        actions: contributions
          .filter((item) => item.source === 'action')
          .map((item) => previewAction(item.action)),
      },
    };
  }

  function extensionRuntimeStateForFile(filename) {
    const base = path.basename(filename || '');
    const matches = Array.from(extensionActionRegistry.values()).filter(
      (entry) => path.basename(entry.extension?.__filePath || '') === base,
    );
    return {
      loaded: matches.length > 0,
      extensionId: matches[0]?.extension?.id,
      commandIds: matches.map((entry) => entry.command?.id).filter(Boolean),
    };
  }

  return {
    aiChatView,
    activatedExtensionPreview,
    extensionRuntimeStateForFile,
  };
}

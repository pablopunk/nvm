// biome-ignore-all lint: Created action records and extension registry entries are dynamic host-side payloads.
import fs from 'node:fs/promises';
import path from 'node:path';
import { AUTOMATE_AI_CHAT_MODEL } from '../../shared/ai-chat-model';
import { createStandaloneExtensionFork } from '../extension-host/extension-manifest';
import { extensionSourceBasename } from '../extension-host/extension-storage-service';
import { hashValue } from '../search/search-utils';
import type { UserStateStore } from '../state/user-state-store';
import { chatTouchedExtensionFiles } from './ai-chat-records';

export type AiCreatedActionsDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  extensionActionRegistry: Map<string, any>;
  scheduleSaveState: () => void;
  invalidateExtensionRootItems: () => void;
  actionAliases: (actionId: any) => string[];
  extensionActionFromContribution: (entry: any) => any;
  extensionModuleForAction: (action: any) => any;
  stageExtensionProposal: (
    filename: string,
    source: string,
  ) => Promise<unknown>;
  activateManagedExtension: (filename: string) => Promise<unknown>;
  loadExtensions: () => Promise<void>;
  registerActionShortcuts: () => void;
  removeAiChat: (chatId: string) => Promise<unknown>;
  removeAiChatReferencesToExtensionFile: (
    extensionFile: string,
  ) => Promise<void>;
};

export function createAiCreatedActions(deps: AiCreatedActionsDeps) {
  const {
    userStateStore,
    extensionActionRegistry,
    scheduleSaveState,
    invalidateExtensionRootItems,
    actionAliases,
    extensionActionFromContribution,
    extensionModuleForAction,
    stageExtensionProposal,
    activateManagedExtension,
    loadExtensions,
    registerActionShortcuts,
    removeAiChat,
    removeAiChatReferencesToExtensionFile,
  } = deps;

  function addAliasForGeneratedAction(chatId) {
    const chat = userStateStore.get().aiChats[chatId];
    if (!chat?.query) return;
    const files = chatTouchedExtensionFiles(chat);
    const entry = Array.from(extensionActionRegistry.values()).find((e) =>
      files.includes(path.basename(e.extension?.__filePath || '')),
    );
    if (!entry) return;
    const action = extensionActionFromContribution(entry);
    if (!action?.id) return;
    const aliases = new Set(actionAliases(action.id));
    aliases.add(chat.query.trim());
    userStateStore.get().aliases[action.id] = Array.from(aliases);
    scheduleSaveState();
  }

  async function duplicateCreatedAction(action) {
    if (
      !(
        ['extension-root-item', 'extension-action'].includes(action?.kind) &&
        action.removable
      )
    )
      return {
        ok: false,
        message: 'Only generated extensions can be duplicated',
      };
    const extension = extensionModuleForAction(action);
    const filePath = extension?.__filePath;
    if (!filePath)
      return { ok: false, message: 'Generated extension not found' };

    const duplicateId = hashValue(`${filePath}:${Date.now()}`);
    const duplicateExtensionId = `${extension.id}-copy-${duplicateId.slice(0, 8)}`;
    const duplicateTitle = `Copy of ${extension.title || action.title}`;
    const duplicateFile = `${extensionSourceBasename(filePath)}-copy-${duplicateId.slice(0, 8)}.ts`;
    const sourceCode = createStandaloneExtensionFork(
      await fs.readFile(filePath, 'utf8'),
      { id: duplicateExtensionId, title: duplicateTitle },
    );
    await stageExtensionProposal(duplicateFile, sourceCode);
    await activateManagedExtension(duplicateFile);
    userStateStore.get().aiChats[duplicateId] = {
      id: duplicateId,
      kind: 'builder',
      model: AUTOMATE_AI_CHAT_MODEL,
      query: `Tweak ${duplicateTitle}`,
      title: `Tweak ${duplicateTitle}`,
      status: 'ready',
      createdAt: Date.now(),
      updatedAt: Date.now(),
      contextExtensionFile: duplicateFile,
      touchedExtensionFiles: [duplicateFile],
      messages: [
        {
          role: 'system',
          content: `Duplicated from "${extension.title || action.title}". Tweak this copy without changing the original.`,
        },
      ],
    };
    scheduleSaveState();
    invalidateExtensionRootItems();
    await loadExtensions();
    registerActionShortcuts();
    const targetRegisteredActionId =
      action.registeredActionId || action.commandId;
    const duplicateEntry = targetRegisteredActionId
      ? extensionActionRegistry.get(
          `${duplicateExtensionId}:${targetRegisteredActionId}`,
        )
      : Array.from(extensionActionRegistry.values()).find(
          (candidate) => candidate.extension?.id === duplicateExtensionId,
        );
    return {
      ok: true,
      message: 'Action duplicated',
      action: duplicateEntry
        ? extensionActionFromContribution(duplicateEntry)
        : {
            id: `ai-tweak-extension:${duplicateFile}`,
            kind: 'ai-tweak-extension',
            extensionFile: duplicateFile,
            title: duplicateTitle,
            subtitle: 'Tweak extension with AI',
            icon: 'sparkles',
            score: 0,
          },
    };
  }

  async function removeCreatedAction(action) {
    if (action?.kind === 'ai-chat' && action.aiChatId) {
      await removeAiChat(action.aiChatId);
      delete userStateStore.get().recents[action.id];
      return { ok: true, message: 'AI chat removed' };
    }

    if (
      ['extension-root-item', 'extension-action'].includes(action?.kind) &&
      action.removable
    ) {
      const extension = extensionModuleForAction(action);
      const filePath = extension?.__filePath;
      if (!filePath)
        return { ok: false, message: 'This extension cannot be removed' };
      await fs.unlink(filePath).catch((error) => {
        if (error?.code !== 'ENOENT') throw error;
      });
      await removeAiChatReferencesToExtensionFile(path.basename(filePath));
      delete userStateStore.get().recents[action.id];
      scheduleSaveState();
      await loadExtensions();
      registerActionShortcuts();
      return { ok: true, message: 'Generated extension removed' };
    }

    return { ok: false, message: 'This action cannot be removed' };
  }

  return {
    addAliasForGeneratedAction,
    duplicateCreatedAction,
    removeCreatedAction,
  };
}

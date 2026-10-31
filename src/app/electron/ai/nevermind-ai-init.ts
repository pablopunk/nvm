// biome-ignore-all lint: AI chat records and extension runtime state are dynamic host-side payloads.
import path from 'node:path';
import { warn as logWarn } from '../platform/logger';
import type { UserStateStore } from '../state/user-state-store';
import { createNevermindAi } from './ai';
import { CONVERSATION_SYSTEM_PROMPT } from './ai-chat-prompts';
import { isBuilderAiChat } from './ai-chat-records';
import type { createPreviewRunObserver } from './ai-preview-run-observer';
import type { NevermindAiEvent } from './nevermind-ai-event-handler';

type AnyRecord = Record<string, any>;

type NevermindAiOptions = Parameters<typeof createNevermindAi>[0];

export type NevermindAiInitializerDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  draftAiChats: Map<string, AnyRecord>;
  EXTENSION_TYPES_FILENAME: string;
  internalExtensionFiles: readonly string[];
  bundledResourcePath: (...relativePath: string[]) => Promise<string>;
  getUserDataPath: () => string;
  getAppPath: () => string;
  getExtensionsDir: () => string;
  extensionDraftsDir: () => string;
  loadExtensions: () => Promise<unknown> | unknown;
  extensionShortcutRecords: () => any;
  getPaletteHotkey: () => string;
  extensionRuntimeStateForFile: (filename: string) => any;
  getActiveAiChatId: () => string | undefined;
  setNevermindAi: (nevermindAi: any) => void;
  getNevermindAi: () => any;
  markGeneratedExtensionForActiveChat: (
    filePath: string,
    chatId?: string,
  ) => void;
  chatCanWriteExtension: (filename: string, chatId?: string) => boolean;
  claimExtensionFileForChat: (chat: any, filename: string) => boolean;
  stageExtensionProposal: (
    filename: string,
    source: string,
  ) => Promise<{ draftFile: string }>;
  discardExtensionProposal: (filename: string) => Promise<unknown>;
  activateManagedExtension: (filename: string) => Promise<unknown>;
  activatedExtensionPreview: (filename: string) => Promise<any>;
  previewRunObserver: ReturnType<typeof createPreviewRunObserver>;
  scheduleSaveState: () => void;
  withLiveExtensionWriteLock: NonNullable<
    NevermindAiOptions['withLiveExtensionWriteLock']
  >;
  removeGeneratedExtensionForChat: NonNullable<
    NevermindAiOptions['removeExtension']
  >;
  addAliasForGeneratedAction: (chatId: string) => void;
  handleNevermindAiEvent: (event: NevermindAiEvent) => void;
};

export function createNevermindAiInitializer(deps: NevermindAiInitializerDeps) {
  const {
    userStateStore,
    draftAiChats,
    EXTENSION_TYPES_FILENAME,
    internalExtensionFiles,
    bundledResourcePath,
    getUserDataPath,
    getAppPath,
    getExtensionsDir,
    extensionDraftsDir,
    loadExtensions,
    extensionShortcutRecords,
    getPaletteHotkey,
    extensionRuntimeStateForFile,
    getActiveAiChatId,
    setNevermindAi,
    getNevermindAi,
    markGeneratedExtensionForActiveChat,
    chatCanWriteExtension,
    claimExtensionFileForChat,
    stageExtensionProposal,
    discardExtensionProposal,
    activateManagedExtension,
    activatedExtensionPreview,
    previewRunObserver,
    scheduleSaveState,
    withLiveExtensionWriteLock,
    removeGeneratedExtensionForChat,
    addAliasForGeneratedAction,
    handleNevermindAiEvent,
  } = deps;

  function getActiveBuilderChat() {
    const activeAiChatId = getActiveAiChatId();
    const chat = activeAiChatId
      ? userStateStore.get().aiChats[activeAiChatId] ||
        draftAiChats.get(activeAiChatId)
      : null;
    return isBuilderAiChat(chat) ? chat : null;
  }

  function getBuilderChat(chatId: string) {
    const chat =
      userStateStore.get().aiChats[chatId] || draftAiChats.get(chatId);
    return isBuilderAiChat(chat) ? chat : null;
  }

  async function activateGeneratedExtension(filename, source, chatId) {
    const chat = chatId
      ? userStateStore.get().aiChats[chatId] || draftAiChats.get(chatId)
      : null;
    if (!(chat && chatCanWriteExtension(filename, chatId)))
      throw new Error(
        `Refusing to write ${path.basename(filename)}: this AI chat does not own that extension.`,
      );
    await stageExtensionProposal(filename, source);
    try {
      await activateManagedExtension(filename);
    } catch (error) {
      await discardExtensionProposal(filename);
      throw error;
    }
    if (!claimExtensionFileForChat(chat, filename))
      throw new Error(
        `Refusing to write ${path.basename(filename)}: this AI chat does not own that extension.`,
      );
    scheduleSaveState();
    const activation = await activatedExtensionPreview(filename);
    const firstCommand = activation.preview.commands?.[0];
    if (firstCommand)
      previewRunObserver.expectRun(
        path.basename(filename),
        String(firstCommand.id),
      );
    return activation;
  }

  async function builderResourcePaths() {
    const [extensionTypesPath, skillPath] = await Promise.all([
      bundledResourcePath(EXTENSION_TYPES_FILENAME),
      bundledResourcePath('skills', 'nevermind-extension-builder', 'SKILL.md'),
    ]);
    return { extensionTypesPath, skillPath };
  }

  function nevermindAiOptions(resourcePaths: {
    extensionTypesPath: string;
    skillPath: string;
  }): NevermindAiOptions {
    const { extensionTypesPath, skillPath } = resourcePaths;
    return {
      agentDir: path.join(getUserDataPath(), 'pi-agent'),
      workspaceDir: path.join(getUserDataPath(), 'ai-workspace'),
      extensionsDir: getExtensionsDir(),
      extensionDraftsDir: extensionDraftsDir(),
      internalExtensionsDir: path.join(
        getAppPath(),
        'src',
        'app',
        'extensions',
      ),
      internalExtensionFiles,
      extensionApiPath: extensionTypesPath,
      extensionTypesPath,
      skillPath,
      reloadExtensions: loadExtensions,
      getShortcuts: () => extensionShortcutRecords(),
      getPaletteShortcut: () => ({
        title: 'Open Nevermind',
        accelerator: String(getPaletteHotkey()),
        scope: 'palette' as const,
      }),
      getExtensionRuntimeState: (filename) =>
        extensionRuntimeStateForFile(filename),
      getActiveChat: getActiveBuilderChat,
      getChat: getBuilderChat,
      markGeneratedExtension: (filePath, chatId) =>
        markGeneratedExtensionForActiveChat(filePath, chatId),
      activateGeneratedExtension,
      readPreviewRun: previewRunObserver.outcomeFor,
      withLiveExtensionWriteLock,
      canWriteExtension: (filename, chatId) =>
        chatCanWriteExtension(filename, chatId),
      removeExtension: (filename, chatId) =>
        removeGeneratedExtensionForChat(filename, chatId),
      addAliasForChat: (chatId) => addAliasForGeneratedAction(chatId),
      onEvent: handleNevermindAiEvent,
    };
  }

  function warmNevermindAiOnStartup() {
    void Promise.all([
      getNevermindAi().prepare({ model: 'fast' }),
      getNevermindAi().prepare({ model: 'smart' }),
      getNevermindAi().prepare({
        model: 'fast',
        system: CONVERSATION_SYSTEM_PROMPT,
        toolMode: 'conversation',
      }),
    ]).catch((error) =>
      logWarn('ai.one-shot.startup-warm.failed', error, {
        source: 'host',
        scope: 'ai',
      }),
    );
  }

  async function initNevermindAi() {
    const resourcePaths = await builderResourcePaths();
    setNevermindAi(createNevermindAi(nevermindAiOptions(resourcePaths)));
    warmNevermindAiOnStartup();
  }

  return { initNevermindAi };
}

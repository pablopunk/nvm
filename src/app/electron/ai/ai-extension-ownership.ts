// biome-ignore-all lint: AI chat ownership records and extension manager state are dynamic host-side payloads.
import fs from 'node:fs/promises';
import path from 'node:path';
import { isExtensionSourceFile } from '../extension-host/extension-source-files';
import { atomicWriteFile } from '../platform/atomic-file';
import type { UserStateStore } from '../state/user-state-store';
import { chatTouchedExtensionFiles, isBuilderAiChat } from './ai-chat-records';
import type { AiChatLearningTrace } from './ai-chat-store';

type AnyRecord = Record<string, any>;

export type AiExtensionOwnershipDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  draftAiChats: Map<string, AnyRecord>;
  extensionModules: Map<string, any>;
  getActiveAiChatId: () => string | undefined;
  getExtensionsDir: () => string;
  scheduleSaveState: () => void;
  persistUserState: () => Promise<void>;
  aiLearningMetadata: (chatId: string) => any;
  getLearningStore: () => Pick<
    AiChatLearningTrace,
    'upsertTraceMetadata'
  > | null;
  patchAiChatsItem: (chatId: string) => void;
  extensionManagerState: () => any;
  captureManagedExtensionRuntime: (sourceFile: string) => any;
  detachManagedExtensionRuntime: (sourceFile: string) => unknown;
  restoreManagedExtensionRuntime: (snapshot: any, candidate: any) => void;
  syncFrontmostAppPolling: () => void;
  registerActionShortcuts: () => void;
  invalidateExtensionRootItems: () => void;
};

export function createAiExtensionOwnership(deps: AiExtensionOwnershipDeps) {
  const {
    userStateStore,
    draftAiChats,
    extensionModules,
    getActiveAiChatId,
    getExtensionsDir,
    scheduleSaveState,
    persistUserState,
    aiLearningMetadata,
    getLearningStore,
    patchAiChatsItem,
    extensionManagerState,
    captureManagedExtensionRuntime,
    detachManagedExtensionRuntime,
    restoreManagedExtensionRuntime,
    syncFrontmostAppPolling,
    registerActionShortcuts,
    invalidateExtensionRootItems,
  } = deps;

  const liveExtensionWriteLocks = new Map<string, Promise<void>>();

  function aiChatIdForExtensionFile(filename) {
    const base = path.basename(filename || '');
    if (!base) return null;
    const matches = Object.values(userStateStore.get().aiChats).filter(
      (chat: any) => chatTouchedExtensionFiles(chat).includes(base),
    );
    return matches.length === 1 ? (matches[0] as any).id : null;
  }

  function extensionFileOwner(filename) {
    const ownerId = aiChatIdForExtensionFile(filename);
    return ownerId
      ? userStateStore.get().aiChats[ownerId] ||
          draftAiChats.get(ownerId) ||
          null
      : null;
  }

  function claimExtensionFileForChat(chat, filename) {
    if (!(isBuilderAiChat(chat) && filename)) return false;
    const safeName = path.basename(filename);
    const owner = extensionFileOwner(safeName);
    if (owner && owner.id !== chat.id) return false;
    touchExtensionFileForChat(chat, safeName);
    return true;
  }

  function touchExtensionFileForChat(chat, filename) {
    if (!(isBuilderAiChat(chat) && filename)) return;
    const safeName = path.basename(filename);
    chat.touchedExtensionFiles = Array.from(
      new Set([...chatTouchedExtensionFiles(chat), safeName]),
    );
    chat.contextExtensionFile = safeName;
    chat.generatedExtensionFile = safeName;
    chat.status = 'ready';
    chat.updatedAt = Date.now();
    getLearningStore()?.upsertTraceMetadata(
      chat.id,
      aiLearningMetadata(chat.id),
    );
    patchAiChatsItem(chat.id);
  }

  function markGeneratedExtensionForActiveChat(
    filePath,
    chatId = getActiveAiChatId(),
  ) {
    const chat = chatId ? userStateStore.get().aiChats[chatId] : null;
    if (!isBuilderAiChat(chat)) return;
    touchExtensionFileForChat(chat, path.basename(filePath));
    scheduleSaveState();
  }

  function chatCanWriteExtension(filename, chatId = getActiveAiChatId()) {
    const base = path.basename(filename || '');
    if (!base) return false;
    const chat = chatId
      ? userStateStore.get().aiChats[chatId] || draftAiChats.get(chatId)
      : null;
    if (!isBuilderAiChat(chat)) return false;
    const owner = extensionFileOwner(base);
    if (owner) return owner.id === chat.id;
    return !Array.from(extensionModules.values()).some(
      (extension) => path.basename(extension.__filePath || '') === base,
    );
  }

  function withLiveExtensionWriteLock<T>(
    filename: string,
    operation: () => Promise<T>,
  ) {
    const safeName = path.basename(filename);
    const previous = liveExtensionWriteLocks.get(safeName) || Promise.resolve();
    const current = previous.catch(() => {}).then(operation);
    const settled = current.then(
      () => undefined,
      () => undefined,
    );
    liveExtensionWriteLocks.set(safeName, settled);
    return current.finally(() => {
      if (liveExtensionWriteLocks.get(safeName) === settled)
        liveExtensionWriteLocks.delete(safeName);
    });
  }

  async function removeGeneratedExtensionForChat(
    filename,
    chatId = getActiveAiChatId(),
  ) {
    const base = path.basename(filename || '');
    if (!(base && isExtensionSourceFile(base))) return { removed: false };
    return withLiveExtensionWriteLock(base, async () => {
      if (!chatCanWriteExtension(base, chatId))
        throw new Error(
          `Refusing to remove ${base}: this AI chat does not own that extension.`,
        );
      const filePath = path.join(getExtensionsDir(), base);
      const existed = await fs
        .stat(filePath)
        .then(() => true)
        .catch(() => false);
      if (!existed) return { removed: false, filePath };
      const manager = extensionManagerState();
      const proposal = manager.proposals?.[base];
      const fileState = manager.files?.[base];
      const source = await fs.readFile(filePath);
      const proposalSource = proposal?.draftFile
        ? await fs.readFile(proposal.draftFile).catch(() => null)
        : null;
      const runtime = captureManagedExtensionRuntime(filePath);
      detachManagedExtensionRuntime(filePath);
      delete manager.files[base];
      delete manager.proposals[base];
      try {
        await fs.unlink(filePath);
        if (proposal?.draftFile)
          await fs.unlink(proposal.draftFile).catch(() => {});
        await persistUserState();
      } catch (error) {
        if (fileState) manager.files[base] = fileState;
        if (proposal) manager.proposals[base] = proposal;
        await atomicWriteFile(filePath, source);
        if (proposal?.draftFile && proposalSource)
          await atomicWriteFile(proposal.draftFile, proposalSource);
        restoreManagedExtensionRuntime(runtime, {
          extensionIds: new Set(),
        });
        throw error;
      }
      try {
        await removeAiChatReferencesToExtensionFile(base);
      } catch (error) {
        await atomicWriteFile(filePath, source);
        if (fileState) manager.files[base] = fileState;
        if (proposal) manager.proposals[base] = proposal;
        if (proposal?.draftFile && proposalSource)
          await atomicWriteFile(proposal.draftFile, proposalSource);
        restoreManagedExtensionRuntime(runtime, {
          extensionIds: new Set(),
        });
        await persistUserState();
        throw error;
      }
      syncFrontmostAppPolling();
      registerActionShortcuts();
      invalidateExtensionRootItems();
      return { removed: true, filePath };
    });
  }

  async function removeAiChatReferencesToExtensionFile(extensionFile) {
    const removedFile = path.basename(extensionFile || '');
    if (!removedFile) return;
    for (const chat of Object.values(
      userStateStore.get().aiChats || {},
    ) as any[]) {
      const touchedFiles = chatTouchedExtensionFiles(chat);
      if (!touchedFiles.includes(removedFile)) continue;
      const remainingFiles = [] as string[];
      for (const filename of touchedFiles.filter(
        (item) => item !== removedFile,
      )) {
        const exists = await fs
          .stat(path.join(getExtensionsDir(), filename))
          .then(() => true)
          .catch(() => false);
        if (exists) remainingFiles.push(filename);
      }
      chat.touchedExtensionFiles = remainingFiles;
      chat.builderPreviewFiles = (chat.builderPreviewFiles || []).filter(
        (filename) => path.basename(filename) !== removedFile,
      );
      if (chat.selectedBuilderPreviewFilename === removedFile)
        chat.selectedBuilderPreviewFilename = chat.builderPreviewFiles[0];
      if (remainingFiles.length === 0) {
        delete chat.contextExtensionFile;
        delete chat.generatedExtensionFile;
        delete chat.builderPreviewFiles;
        delete chat.selectedBuilderPreviewFilename;
      }
      if (chat.contextExtensionFile === removedFile)
        chat.contextExtensionFile = remainingFiles[0];
      if (chat.generatedExtensionFile === removedFile)
        chat.generatedExtensionFile = remainingFiles[0];
      chat.updatedAt = Date.now();
      scheduleSaveState();
    }
  }

  return {
    aiChatIdForExtensionFile,
    claimExtensionFileForChat,
    markGeneratedExtensionForActiveChat,
    chatCanWriteExtension,
    withLiveExtensionWriteLock,
    removeGeneratedExtensionForChat,
    removeAiChatReferencesToExtensionFile,
  };
}

// biome-ignore-all lint: AI chat records are dynamic persisted payloads.
import fs from 'node:fs/promises';
import {
  AUTOMATE_AI_CHAT_MODEL,
  normalizeAiChatModel,
} from '../../shared/ai-chat-model';
import { withoutAiToolCallHistory } from '../../shared/ai-chat-tool-history';
import type { UserStateStore } from '../state/user-state-store';
import { aiChatPreviewFiles } from './ai-chat-previews';
import { isConversationAiChat } from './ai-chat-records';

export type AiChatMigrationDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  getExtensionsDir: () => string;
};

export function createAiChatMigration(deps: AiChatMigrationDeps) {
  const { userStateStore, getExtensionsDir } = deps;

  async function migrateAiChats() {
    const existingFiles = new Set(
      await fs
        .readdir(getExtensionsDir(), { withFileTypes: true })
        .then((entries) =>
          entries.filter((entry) => entry.isFile()).map((entry) => entry.name),
        )
        .catch(() => []),
    );
    let chatsChanged = false;
    for (const chat of Object.values(
      userStateStore.get().aiChats || {},
    ) as any[]) {
      const expectedModel = isConversationAiChat(chat)
        ? normalizeAiChatModel(chat.model)
        : AUTOMATE_AI_CHAT_MODEL;
      if (chat.model !== expectedModel) {
        chat.model = expectedModel;
        chatsChanged = true;
      }
      if (isConversationAiChat(chat)) continue;
      if (Array.isArray(chat.messages)) {
        const messages = withoutAiToolCallHistory(chat.messages);
        if (messages.length !== chat.messages.length) {
          chat.messages = messages;
          chatsChanged = true;
        }
      }
      if (chat.generatedExtensionFile && !chat.touchedExtensionFiles)
        chat.touchedExtensionFiles = [chat.generatedExtensionFile];
      if (chat.generatedExtensionFile && !chat.contextExtensionFile)
        chat.contextExtensionFile = chat.generatedExtensionFile;
      if (Array.isArray(chat.builderPreviews)) {
        chat.builderPreviewFiles = chat.builderPreviews
          .map((preview) => preview?.filename)
          .filter((filename) => typeof filename === 'string');
        delete chat.builderPreviews;
      }
      const previewState = aiChatPreviewFiles(chat, existingFiles);
      if (previewState.files.length) {
        chat.builderPreviewFiles = previewState.files;
        chat.selectedBuilderPreviewFilename =
          previewState.selectedBuilderPreviewFilename;
      } else {
        delete chat.builderPreviewFiles;
        delete chat.selectedBuilderPreviewFilename;
      }
    }
    return chatsChanged;
  }

  return { migrateAiChats };
}

import path from 'node:path';
import type { UserStateStore } from '../state/user-state-store';

export type ExtensionActionLookupDeps = {
  extensionActionRegistry: Map<string, any>;
  extensionModules: Map<string, any>;
  userStateStore: Pick<UserStateStore, 'get'>;
  chatTouchedExtensionFiles: (chat: any) => string[];
  extensionActionFromContribution: (entry: any) => any;
};

export function createExtensionActionLookup(deps: ExtensionActionLookupDeps) {
  const {
    extensionActionRegistry,
    extensionModules,
    userStateStore,
    chatTouchedExtensionFiles,
    extensionActionFromContribution,
  } = deps;

  function extensionActionEntryForAction(action) {
    const registeredActionId = action?.registeredActionId || action?.commandId;
    const direct = extensionActionRegistry.get(
      `${action.extensionId}:${registeredActionId}`,
    );
    if (direct) return direct;

    if (action?.extensionFile) {
      const fileMatches = Array.from(extensionActionRegistry.values()).filter(
        (entry) =>
          path.basename(entry.extension.__filePath || '') ===
          action.extensionFile,
      );
      if (fileMatches.length === 1) return fileMatches[0];
    }

    if (action?.aiChatId) {
      const files = chatTouchedExtensionFiles(
        userStateStore.get().aiChats[action.aiChatId],
      );
      const chatMatches = Array.from(extensionActionRegistry.values()).filter(
        (entry) =>
          files.includes(path.basename(entry.extension.__filePath || '')),
      );
      if (chatMatches.length === 1) return chatMatches[0];
    }

    const matches = Array.from(extensionActionRegistry.values()).filter(
      (entry) => entry.extension.id === action?.extensionId,
    );
    return matches.length === 1 ? matches[0] : null;
  }

  function extensionEntryForAction(action) {
    return extensionActionEntryForAction(action);
  }

  function extensionModuleForAction(action) {
    const entry = extensionActionEntryForAction(action);
    if (entry?.extension) return entry.extension;
    if (!action?.extensionFile) return null;
    return (
      Array.from(extensionModules.values()).find(
        (extension) =>
          path.basename(extension.__filePath || '') === action.extensionFile,
      ) || null
    );
  }

  function currentActionForStoredShortcut(action) {
    if (action?.kind === 'extension-root-item') {
      const persistentAction = action.persistentAction;
      if (persistentAction)
        return currentActionForStoredShortcut(persistentAction);
      const reference = action.rootAction;
      if (reference?.type === 'runExtensionRegisteredAction') {
        const extensionId = reference.extensionId || action.extensionId;
        const registeredActionId =
          reference.registeredActionId || reference.actionId;
        const entry = extensionActionRegistry.get(
          `${extensionId}:${registeredActionId}`,
        );
        if (entry) return extensionActionFromContribution(entry);
      }
    }
    if (action?.kind === 'extension-command') {
      const entry = extensionActionEntryForAction(action);
      return entry ? extensionActionFromContribution(entry) : null;
    }
    if (action?.kind === 'extension-action') {
      const entry = extensionActionEntryForAction(action);
      return entry ? extensionActionFromContribution(entry) : null;
    }
    return action;
  }

  function resolveRegisteredActionRef(action) {
    let current = action;
    const visited = new Set<string>();
    while (current?.type === 'runExtensionRegisteredAction') {
      const extensionId = current.extensionId;
      const registeredActionId = current.registeredActionId || current.actionId;
      const key = `${extensionId}:${registeredActionId}`;
      if (visited.has(key)) return { error: 'Action reference cycle detected' };
      visited.add(key);
      const entry = extensionActionRegistry.get(key);
      const rootAction = entry ? extensionActionFromContribution(entry) : null;
      if (!rootAction) return { error: 'Action unavailable' };
      if (rootAction.rootAction?.type !== 'runExtensionRegisteredAction')
        return { rootAction };
      current = rootAction.rootAction;
    }
    return { error: 'Action unavailable' };
  }

  return {
    extensionActionEntryForAction,
    extensionEntryForAction,
    extensionModuleForAction,
    currentActionForStoredShortcut,
    resolveRegisteredActionRef,
  };
}

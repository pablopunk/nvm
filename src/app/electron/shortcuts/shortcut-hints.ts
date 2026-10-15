import type { UserStateStore } from '../state/user-state-store';
import { defaultActionIdFor } from './action-customization';
import { buildShortcutByAiChatIdMap } from './shortcut-ownership';

export type ShortcutHintsDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  chatTouchedExtensionFiles: (chat: any) => string[];
};

export function createShortcutHints(deps: ShortcutHintsDeps) {
  const { userStateStore, chatTouchedExtensionFiles } = deps;

  let shortcutByAiChatIdCache: Map<string, string> | null = null;
  function shortcutByAiChatIdMap() {
    if (shortcutByAiChatIdCache) return shortcutByAiChatIdCache;
    shortcutByAiChatIdCache = buildShortcutByAiChatIdMap(
      userStateStore.get().shortcutActions,
      userStateStore.get().shortcuts,
      userStateStore.get().aiChats,
      chatTouchedExtensionFiles,
    );
    return shortcutByAiChatIdCache;
  }

  function invalidateShortcutCaches() {
    shortcutByAiChatIdCache = null;
  }

  function shortcutForAction(action: any) {
    if (userStateStore.get().shortcuts[action.id])
      return userStateStore.get().shortcuts[action.id];
    if (userStateStore.get().removedShortcuts?.[action.id]) return null;
    if (!action.aiChatId) return null;
    return shortcutByAiChatIdMap().get(action.aiChatId) || null;
  }

  function withShortcutHint(action: any) {
    const shortcut = shortcutForAction(action);
    return shortcut ? { ...action, shortcut } : action;
  }

  function withDefaultOverride(action: any) {
    const defaultActionId = defaultActionIdFor(action);
    if (!defaultActionId) return withShortcutHint(action);
    const override = userStateStore.get().overrides[defaultActionId];
    return {
      ...action,
      defaultActionId,
      isOverridden: Boolean(override),
      overrideSummary: override?.instruction,
      shortcut: shortcutForAction(action),
    };
  }

  return {
    shortcutForAction,
    withShortcutHint,
    withDefaultOverride,
    invalidateShortcutCaches,
  };
}

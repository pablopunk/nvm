import type { UserStateStore } from '../state/user-state-store';
import { normalizeAccelerator } from './shortcut-utils';

const PALETTE_HOTKEY_ACTION_ID = '__palette-hotkey__';

export type ShortcutCatalogDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  visibleExtensionActionEntries: () => any[];
  extensionActionFromContribution: (entry: any) => any;
  resolveStoredShortcutAction: (actionId: string) => any;
  patchOpenView: (viewId: string, patch: any) => void;
  activationFailure: (key: string) => string | null;
};

export function createShortcutCatalog(deps: ShortcutCatalogDeps) {
  const {
    userStateStore,
    visibleExtensionActionEntries,
    extensionActionFromContribution,
    resolveStoredShortcutAction,
    patchOpenView,
    activationFailure,
  } = deps;

  function inactiveSubtitle(item) {
    const failure = activationFailure(`shortcut:${item.actionId}`);
    return failure ? `Not active: ${failure}` : item.action.subtitle;
  }

  function declaredGlobalShortcuts() {
    return visibleExtensionActionEntries()
      .map((entry) => {
        const accelerator =
          entry.item.globalShortcut ||
          (entry.item.shortcutScope === 'global' ? entry.item.shortcut : null);
        if (!accelerator) return null;
        const action = extensionActionFromContribution(entry);
        return action
          ? {
              actionId: action.id,
              accelerator: normalizeAccelerator(accelerator),
              action,
            }
          : null;
      })
      .filter(Boolean);
  }

  function getShortcuts() {
    const configured = Object.entries(userStateStore.get().shortcuts)
      .map(([actionId, accelerator]) => ({
        actionId,
        accelerator: String(accelerator),
        scope: 'global',
        source: 'user' as const,
        action: resolveStoredShortcutAction(actionId),
      }))
      .filter((item) => item.action);
    const declared = declaredGlobalShortcuts()
      .filter(
        (item) =>
          !(
            item.actionId in userStateStore.get().shortcuts ||
            userStateStore.get().removedShortcuts?.[item.actionId]
          ),
      )
      .map((item) => ({
        ...item,
        accelerator: String(item.accelerator),
        scope: 'global' as const,
        source: 'extension' as const,
      }));
    return [...configured, ...declared].sort((a, b) =>
      a.action.title.localeCompare(b.action.title),
    );
  }

  function extensionShortcutRecords() {
    return getShortcuts().map((item) => ({
      actionId: item.actionId,
      title: item.action.title,
      subtitle: inactiveSubtitle(item),
      accelerator: item.accelerator,
      scope: 'global' as const,
      source: item.source,
    }));
  }

  function resolveShortcutTargetAction(input: any) {
    if (input?.action) return input.action;
    const actionId = input?.actionId;
    if (!actionId) return null;
    if (actionId === PALETTE_HOTKEY_ACTION_ID || input?.scope === 'palette')
      return { id: PALETTE_HOTKEY_ACTION_ID };
    const record = getShortcuts().find((item) => item.actionId === actionId);
    return record?.action || { id: actionId };
  }

  function buildRecordShortcutAction(input: any, options: any) {
    const scope =
      input?.scope === 'palette' || input?.actionId === PALETTE_HOTKEY_ACTION_ID
        ? 'palette'
        : 'action';
    const targetAction =
      scope === 'palette'
        ? { id: PALETTE_HOTKEY_ACTION_ID }
        : resolveShortcutTargetAction(input);
    const title =
      input?.title ||
      options?.title ||
      (scope === 'palette' ? 'Change Shortcut' : 'Record shortcut');
    const shortcut =
      input?.shortcut === undefined ? undefined : String(input.shortcut);
    return {
      ...options,
      type: 'recordShortcut',
      title,
      action: targetAction,
      ...(shortcut === undefined ? {} : { shortcut }),
    };
  }

  function buildRemoveShortcutAction(input: any, options: any) {
    const actionId = input?.actionId || input?.action?.id;
    const title = input?.title || options?.title || 'Remove shortcut';
    return {
      style: 'destructive',
      ...options,
      type: 'removeShortcut',
      title,
      actionId,
    };
  }

  function keyboardShortcutItem(record: any) {
    const changeAction = buildRecordShortcutAction(
      { actionId: record.actionId, title: 'Change shortcut' },
      {},
    );
    const removeAction =
      record.source === 'user'
        ? buildRemoveShortcutAction(
            { actionId: record.actionId, title: 'Remove shortcut' },
            {},
          )
        : null;
    return {
      id: `shortcut:${record.actionId}`,
      title: record.title,
      subtitle: record.subtitle,
      shortcut: record.accelerator,
      shortcutScope: record.scope,
      icon: 'keyboard',
      primaryAction: changeAction,
      actionPanel: {
        sections: [{ actions: [changeAction, removeAction].filter(Boolean) }],
      },
    };
  }

  function keyboardShortcutItems() {
    return extensionShortcutRecords().map(keyboardShortcutItem);
  }

  function patchKeyboardShortcutsView() {
    patchOpenView('keyboard-shortcuts', {
      mode: 'replace',
      items: keyboardShortcutItems(),
    });
  }

  return {
    declaredGlobalShortcuts,
    getShortcuts,
    extensionShortcutRecords,
    buildRecordShortcutAction,
    buildRemoveShortcutAction,
    patchKeyboardShortcutsView,
  };
}

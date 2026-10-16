import { canCustomizeCommandAction } from '../../palette/model';
import { executeSystemBuiltin, reservedPaletteShortcutName } from '../os/os';
import type { UserStateStore } from '../state/user-state-store';
import { isSpotlightAccelerator, normalizeAccelerator } from './shortcut-utils';

export type ActionCustomizationDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  scheduleSaveState: () => void;
  globalShortcut: { unregister: (accelerator: string) => void };
  paletteWindow: {
    isHotkeyReserved: (accelerator: string) => boolean;
    registerHotkey: (accelerator?: string) => { registered: boolean };
    unregisterHotkey: () => void;
  };
  actionAliases: (actionId: string) => string[];
  chatTouchedExtensionFiles: (chat: any) => string[];
  getPaletteHotkey: () => string;
  setSetting: (id: string, value: unknown) => unknown;
  settingDefinition: (id: string) => any;
  declaredGlobalShortcuts: () => any[];
  invalidateShortcutCaches: () => void;
  patchKeyboardShortcutsView: () => void;
  unregisterShortcutForAction: (actionId: string) => void;
  registerActionShortcut: (
    actionId: string,
    accelerator: string,
    action: any,
  ) => boolean;
};

export function defaultActionIdFor(action: any) {
  if (action.defaultActionId) return action.defaultActionId;
  if (action.kind === 'builtin') return action.id;
  if (action.kind === 'calculate') return 'default:calculator';
  return null;
}

export function createActionCustomization(deps: ActionCustomizationDeps) {
  const {
    userStateStore,
    scheduleSaveState,
    globalShortcut,
    paletteWindow,
    actionAliases,
    chatTouchedExtensionFiles,
    getPaletteHotkey,
    setSetting,
    settingDefinition,
    declaredGlobalShortcuts,
    invalidateShortcutCaches,
    patchKeyboardShortcutsView,
    unregisterShortcutForAction,
    registerActionShortcut,
  } = deps;

  function canCustomizeAction(action) {
    return canCustomizeCommandAction(action);
  }

  async function removeShortcut(actionId) {
    if (!actionId) return { ok: false, message: 'Shortcut not found' };
    if (userStateStore.get().shortcuts[actionId]) {
      globalShortcut.unregister(userStateStore.get().shortcuts[actionId]);
      delete userStateStore.get().shortcuts[actionId];
      delete userStateStore.get().shortcutActions[actionId];
    } else {
      const declared = declaredGlobalShortcuts().find(
        (item) => item.actionId === actionId,
      );
      if (!declared) return { ok: false, message: 'Shortcut not found' };
      globalShortcut.unregister(declared.accelerator);
      if (!userStateStore.get().removedShortcuts)
        userStateStore.get().removedShortcuts = {};
      userStateStore.get().removedShortcuts[actionId] = declared.accelerator;
    }
    invalidateShortcutCaches();
    scheduleSaveState();
    patchKeyboardShortcutsView();
    return { ok: true, message: 'Shortcut removed' };
  }

  async function setAlias(action, alias) {
    if (!canCustomizeAction(action))
      return {
        ok: false,
        message: 'Aliases are only available for persistent actions',
      };
    if (!(action?.id && alias.trim()))
      return { ok: false, message: 'Missing alias' };
    const aliases = new Set(actionAliases(action.id));
    aliases.add(alias.trim());
    userStateStore.get().aliases[action.id] = Array.from(aliases);
    scheduleSaveState();
    return { ok: true, message: `Alias set: ${alias.trim()}` };
  }

  async function removeAlias(action, alias) {
    if (!(action?.id && alias)) return { ok: false, message: 'Missing alias' };
    const current = actionAliases(action.id).filter((value) => value !== alias);
    if (current.length) userStateStore.get().aliases[action.id] = current;
    else delete userStateStore.get().aliases[action.id];
    scheduleSaveState();
    return { ok: true, message: `Alias removed: ${alias}` };
  }

  async function setShortcut(action, shortcut) {
    if (!canCustomizeAction(action))
      return {
        ok: false,
        message: 'Shortcuts are only available for persistent actions',
      };
    if (!(action?.id && shortcut.trim()))
      return { ok: false, message: 'Missing shortcut' };
    const accelerator = normalizeAccelerator(shortcut);
    if (paletteWindow.isHotkeyReserved(accelerator))
      return {
        ok: false,
        message: `${accelerator} is reserved for opening Nevermind`,
      };
    const conflictingActionId = Object.entries(
      userStateStore.get().shortcuts,
    ).find(
      ([actionId, current]) =>
        actionId !== action.id && current === accelerator,
    )?.[0];
    if (conflictingActionId) {
      const conflictingAction =
        userStateStore.get().shortcutActions[conflictingActionId];
      if (
        action.aiChatId &&
        conflictingAction?.aiChatId === action.aiChatId &&
        chatTouchedExtensionFiles(userStateStore.get().aiChats[action.aiChatId])
          .length === 1
      ) {
        await removeShortcut(conflictingActionId);
      } else {
        const title = conflictingAction?.title || 'another action';
        return {
          ok: false,
          message: `${accelerator} is already used by ${title}`,
        };
      }
    }
    unregisterShortcutForAction(action.id);
    delete userStateStore.get().removedShortcuts?.[action.id];
    const ok = registerActionShortcut(action.id, accelerator, action);
    if (!ok) return { ok: false, message: `Could not register ${accelerator}` };
    scheduleSaveState();
    patchKeyboardShortcutsView();
    return { ok: true, message: `Shortcut set: ${accelerator}` };
  }

  function setShortcutSetting(id, accelerator) {
    const definition = settingDefinition(id);
    if (!definition || definition.type !== 'shortcut')
      return { ok: false, message: 'Setting not found' };
    if (id === 'paletteHotkey')
      return { ok: false, message: 'Use palette hotkey registration' };
    if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' };
    const normalized = normalizeAccelerator(accelerator);
    setSetting(id, normalized);
    return { ok: true, message: `${definition.title} set: ${normalized}` };
  }

  async function setPaletteHotkey(accelerator) {
    if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' };
    const normalized = normalizeAccelerator(accelerator);
    const current = String(getPaletteHotkey());
    if (normalized === current)
      return {
        ok: true,
        message: `Shortcut unchanged: ${normalized}`,
        spotlightConflict: isSpotlightAccelerator(normalized),
      };
    const conflictingActionId = Object.entries(
      userStateStore.get().shortcuts,
    ).find(([, value]) => value === normalized)?.[0];
    if (conflictingActionId) {
      const title =
        userStateStore.get().shortcutActions[conflictingActionId]?.title ||
        'another action';
      return {
        ok: false,
        message: `${normalized} is already used by ${title}`,
      };
    }
    paletteWindow.unregisterHotkey();
    const registration = paletteWindow.registerHotkey(normalized);
    if (!registration.registered) {
      paletteWindow.registerHotkey(current);
      const spotlightConflict = isSpotlightAccelerator(normalized);
      return {
        ok: false,
        message: spotlightConflict
          ? `${normalized} is used by ${reservedPaletteShortcutName()}`
          : `Could not register ${normalized}`,
        spotlightConflict,
      };
    }
    setSetting('paletteHotkey', normalized);
    return {
      ok: true,
      message: `Shortcut set: ${normalized}`,
      spotlightConflict: isSpotlightAccelerator(normalized),
    };
  }

  function suspendPaletteHotkey() {
    paletteWindow.unregisterHotkey();
  }

  function resumePaletteHotkey() {
    paletteWindow.registerHotkey();
  }

  async function openSystemKeyboardSettings() {
    await executeSystemBuiltin({ builtin: 'open-keyboard-settings' }, () => {});
    return { ok: true };
  }

  async function setOverride(action, instruction) {
    const defaultActionId = defaultActionIdFor(action);
    if (!defaultActionId)
      return { ok: false, message: 'This action cannot be overridden yet' };
    if (!instruction.trim())
      return { ok: false, message: 'Missing override instructions' };
    userStateStore.get().overrides[defaultActionId] = {
      instruction: instruction.trim(),
      updatedAt: Date.now(),
      originalTitle: action.title,
    };
    scheduleSaveState();
    return { ok: true, message: 'Override saved' };
  }

  async function clearOverride(action) {
    const defaultActionId = defaultActionIdFor(action);
    if (!defaultActionId)
      return { ok: false, message: 'This action has no original to restore' };
    delete userStateStore.get().overrides[defaultActionId];
    scheduleSaveState();
    return { ok: true, message: 'Original restored' };
  }

  return {
    removeShortcut,
    setAlias,
    removeAlias,
    setShortcut,
    setShortcutSetting,
    setPaletteHotkey,
    suspendPaletteHotkey,
    resumePaletteHotkey,
    openSystemKeyboardSettings,
    setOverride,
    clearOverride,
  };
}

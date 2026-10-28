import { canCustomizeCommandAction } from '../../palette/model';
import { executeSystemBuiltin, reservedPaletteShortcutName } from '../os/os';
import type { UserStateStore } from '../state/user-state-store';
import { isSpotlightAccelerator, normalizeAccelerator } from './shortcut-utils';

type CommitInput = {
  label: string;
  actionId?: string;
  actionIsGenerated?: boolean;
  mutate: (draft: any) => string | void;
  beforeApply?: () => () => void;
  afterCommit?: () => void;
};

type CommitOutcome = { ok: true } | { ok: false; message: string };

export type ActionCustomizationDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  scheduleSaveState: () => void;
  commitPortable: (input: CommitInput) => Promise<CommitOutcome>;
  paletteWindow: {
    isHotkeyReserved: (accelerator: string) => boolean;
    registerHotkey: (accelerator?: string) => unknown;
    unregisterHotkey: () => void;
  };
  chatTouchedExtensionFiles: (chat: any) => string[];
  getPaletteHotkey: () => string;
  setSetting: (id: string, value: unknown) => Promise<unknown>;
  settingDefinition: (id: string) => any;
  declaredGlobalShortcuts: () => any[];
};

export function defaultActionIdFor(action: any) {
  if (action.defaultActionId) return action.defaultActionId;
  if (action.kind === 'builtin') return action.id;
  if (action.kind === 'calculate') return 'default:calculator';
  return null;
}

function outcomeWithMessage(outcome: CommitOutcome, message: string) {
  return outcome.ok === true
    ? { ok: true, message }
    : { ok: false, message: outcome.message };
}

function actionClassification(action: any) {
  return {
    actionId: action?.id,
    actionIsGenerated: Boolean(action?.removable),
  };
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: Each customization writer is a short commit; the factory groups them with their shared dependencies.
export function createActionCustomization(deps: ActionCustomizationDeps) {
  const {
    userStateStore,
    scheduleSaveState,
    commitPortable,
    paletteWindow,
    chatTouchedExtensionFiles,
    getPaletteHotkey,
    setSetting,
    settingDefinition,
    declaredGlobalShortcuts,
  } = deps;

  function canCustomizeAction(action) {
    return canCustomizeCommandAction(action);
  }

  function canReplaceShortcut(action, conflicting) {
    return Boolean(
      action.aiChatId &&
        conflicting?.aiChatId === action.aiChatId &&
        chatTouchedExtensionFiles(userStateStore.get().aiChats[action.aiChatId])
          .length === 1,
    );
  }

  async function removeShortcut(actionId) {
    if (!actionId) return { ok: false, message: 'Shortcut not found' };
    const wasUserShortcut =
      typeof userStateStore.get().shortcuts[actionId] === 'string';
    const hasDeclaredShortcut = declaredGlobalShortcuts().some(
      (item) => item.actionId === actionId,
    );
    if (!(wasUserShortcut || hasDeclaredShortcut))
      return { ok: false, message: 'Shortcut not found' };
    const outcome = await commitPortable({
      label: 'shortcut:remove',
      actionId,
      mutate: (draft) => {
        if (typeof draft.shortcuts[actionId] === 'string')
          delete draft.shortcuts[actionId];
        else draft.shortcuts[actionId] = null;
      },
      afterCommit: () => {
        if (wasUserShortcut)
          delete userStateStore.get().shortcutActions[actionId];
      },
    });
    return outcomeWithMessage(outcome, 'Shortcut removed');
  }

  async function setAlias(action, alias) {
    if (!canCustomizeAction(action))
      return {
        ok: false,
        message: 'Aliases are only available for persistent actions',
      };
    if (!(action?.id && alias.trim()))
      return { ok: false, message: 'Missing alias' };
    const value = alias.trim();
    const outcome = await commitPortable({
      label: 'alias:add',
      ...actionClassification(action),
      mutate: (draft) => {
        draft.aliases[action.id] = [
          ...new Set([...(draft.aliases[action.id] ?? []), value]),
        ];
      },
    });
    return outcomeWithMessage(outcome, `Alias set: ${value}`);
  }

  async function removeAlias(action, alias) {
    if (!(action?.id && alias)) return { ok: false, message: 'Missing alias' };
    const outcome = await commitPortable({
      label: 'alias:remove',
      ...actionClassification(action),
      mutate: (draft) => {
        const remaining = (draft.aliases[action.id] ?? []).filter(
          (value: string) => value !== alias,
        );
        if (remaining.length) draft.aliases[action.id] = remaining;
        else delete draft.aliases[action.id];
      },
    });
    return outcomeWithMessage(outcome, `Alias removed: ${alias}`);
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
    let replacedActionId: string | null = null;
    const outcome = await commitPortable({
      label: 'shortcut:set',
      ...actionClassification(action),
      mutate: (draft) => {
        const conflictingId = Object.entries(draft.shortcuts).find(
          ([id, current]) => id !== action.id && current === accelerator,
        )?.[0];
        if (conflictingId) {
          const conflicting =
            userStateStore.get().shortcutActions[conflictingId];
          if (!canReplaceShortcut(action, conflicting))
            return `${accelerator} is already used by ${conflicting?.title || 'another action'}`;
          replacedActionId = conflictingId;
          delete draft.shortcuts[conflictingId];
        }
        draft.shortcuts[action.id] = accelerator;
      },
      beforeApply: () => {
        const actions = userStateStore.get().shortcutActions;
        const previous = actions[action.id];
        actions[action.id] = action;
        return () => {
          if (previous === undefined) delete actions[action.id];
          else actions[action.id] = previous;
        };
      },
      afterCommit: () => {
        if (replacedActionId)
          delete userStateStore.get().shortcutActions[replacedActionId];
        userStateStore.get().shortcutActions[action.id] = action;
      },
    });
    return outcomeWithMessage(outcome, `Shortcut set: ${accelerator}`);
  }

  async function setShortcutSetting(id, accelerator) {
    const definition = settingDefinition(id);
    if (!definition || definition.type !== 'shortcut')
      return { ok: false, message: 'Setting not found' };
    if (id === 'paletteHotkey')
      return { ok: false, message: 'Use palette hotkey registration' };
    if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' };
    const normalized = normalizeAccelerator(accelerator);
    const result: any = await setSetting(id, normalized);
    if (!result.ok) return result;
    return { ok: true, message: `${definition.title} set: ${normalized}` };
  }

  async function setPaletteHotkey(accelerator) {
    if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' };
    const normalized = normalizeAccelerator(accelerator);
    const current = String(getPaletteHotkey());
    const spotlightConflict = isSpotlightAccelerator(normalized);
    if (normalized === current)
      return {
        ok: true,
        message: `Shortcut unchanged: ${normalized}`,
        spotlightConflict,
      };
    const outcome = await commitPortable({
      label: 'palette-hotkey',
      mutate: (draft) => {
        const conflictingId = Object.entries(draft.shortcuts).find(
          ([, value]) => value === normalized,
        )?.[0];
        if (conflictingId) {
          const title =
            userStateStore.get().shortcutActions[conflictingId]?.title ||
            'another action';
          return `${normalized} is already used by ${title}`;
        }
        draft.settings.paletteHotkey = normalized;
      },
    });
    if (outcome.ok === false)
      return {
        ok: false,
        message: outcome.message,
        spotlightConflict,
      };
    return {
      ok: true,
      message: `Shortcut set: ${normalized}`,
      spotlightConflict,
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

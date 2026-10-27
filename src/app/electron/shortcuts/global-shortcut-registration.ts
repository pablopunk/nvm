import { warn as logWarn } from '../platform/logger';
import type { UserStateStore } from '../state/user-state-store';

export type GlobalShortcutRegistrationDeps = {
  globalShortcut: {
    register: (accelerator: string, listener: () => void) => boolean;
    unregister: (accelerator: string) => void;
    isRegistered: (accelerator: string) => boolean;
  };
  paletteWindow: {
    isHotkeyReserved: (accelerator: string) => boolean;
    debugLog: (message: string, data?: unknown) => void;
  };
  registeredActionAccelerators: Set<string>;
  userStateStore: Pick<UserStateStore, 'get'>;
  invalidateShortcutCaches: () => void;
  executeShortcutAction: (action: any) => unknown;
  resolveStoredShortcutAction: (actionId: string) => any;
  declaredGlobalShortcuts: () => any[];
};

export function createGlobalShortcutRegistration(
  deps: GlobalShortcutRegistrationDeps,
) {
  const {
    globalShortcut,
    paletteWindow,
    registeredActionAccelerators,
    userStateStore,
    invalidateShortcutCaches,
    executeShortcutAction,
    resolveStoredShortcutAction,
    declaredGlobalShortcuts,
  } = deps;

  function unregisterShortcutForAction(actionId) {
    const current = userStateStore.get().shortcuts[actionId];
    if (current) globalShortcut.unregister(current);
  }

  function bindGlobalActionShortcut(actionId, accelerator, action) {
    if (paletteWindow.isHotkeyReserved(accelerator)) return false;
    globalShortcut.unregister(accelerator);
    registeredActionAccelerators.add(accelerator);
    const ok = globalShortcut.register(accelerator, () =>
      executeShortcutAction(action),
    );
    paletteWindow.debugLog('registerActionShortcut', {
      actionId,
      accelerator,
      title: action?.title,
      ok,
      isRegistered: globalShortcut.isRegistered(accelerator),
    });
    return ok;
  }

  function registerActionShortcut(actionId, accelerator, action) {
    const ok = bindGlobalActionShortcut(actionId, accelerator, action);
    if (!ok) return false;
    userStateStore.get().shortcuts[actionId] = accelerator;
    userStateStore.get().shortcutActions[actionId] = action;
    invalidateShortcutCaches();
    return true;
  }

  function unregisterActionShortcuts() {
    for (const accelerator of registeredActionAccelerators)
      globalShortcut.unregister(accelerator);
    registeredActionAccelerators.clear();
  }

  function registerActionShortcuts() {
    unregisterActionShortcuts();
    const bound = new Set();
    for (const [actionId, accelerator] of Object.entries(
      userStateStore.get().shortcuts,
    )) {
      const action = resolveStoredShortcutAction(actionId);
      if (!action) continue;
      const ok = bindGlobalActionShortcut(actionId, accelerator, action);
      if (ok) bound.add(accelerator);
      else
        logWarn(
          'actionShortcut.register.failed',
          { actionId, accelerator },
          { source: 'host', scope: 'shortcuts' },
        );
    }
    for (const { actionId, accelerator, action } of declaredGlobalShortcuts()) {
      if (
        userStateStore.get().shortcuts[actionId] ||
        userStateStore.get().removedShortcuts?.[actionId] ||
        bound.has(accelerator)
      )
        continue;
      const ok = bindGlobalActionShortcut(actionId, accelerator, action);
      if (ok) bound.add(accelerator);
      else
        logWarn(
          'declaredActionShortcut.register.failed',
          { actionId, accelerator },
          { source: 'host', scope: 'shortcuts' },
        );
    }
  }

  return {
    unregisterShortcutForAction,
    registerActionShortcut,
    unregisterActionShortcuts,
    registerActionShortcuts,
  };
}

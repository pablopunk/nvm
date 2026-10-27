import type { PortableConfig } from '../state/portable-config-schema';
import {
  type DeclaredShortcut,
  desiredShortcutBindings,
} from './shortcut-binding-plan';
import { createShortcutBindings } from './shortcut-bindings';

export type GlobalShortcutRegistrationDeps = {
  globalShortcut: {
    register: (accelerator: string, listener: () => void) => boolean;
    unregister: (accelerator: string) => void;
  };
  isPaletteHotkeyReserved: (accelerator: string) => boolean;
  resolveStoredShortcutAction: (actionId: string) => any;
  declaredGlobalShortcuts: () => DeclaredShortcut[];
  executeShortcutAction: (action: any) => unknown;
};

export function createGlobalShortcutRegistration(
  deps: GlobalShortcutRegistrationDeps,
) {
  const bindings = createShortcutBindings({
    globalShortcut: deps.globalShortcut,
    runAction: (actionId) => {
      const action = deps.resolveStoredShortcutAction(actionId);
      return action ? deps.executeShortcutAction(action) : undefined;
    },
  });

  function applyShortcutView(view: PortableConfig) {
    const desired = desiredShortcutBindings(
      view,
      deps.resolveStoredShortcutAction,
      deps.declaredGlobalShortcuts(),
    );
    const paletteHotkey = String(view.settings.paletteHotkey ?? '');
    return bindings.apply(
      desired,
      (accelerator) =>
        accelerator === paletteHotkey ||
        deps.isPaletteHotkeyReserved(accelerator),
    );
  }

  return { applyShortcutView, releaseAllShortcuts: bindings.releaseAll };
}

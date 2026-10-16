import { isNvmTestMode } from '../platform/test-mode';
import { runInBackground } from './run-in-background';

export type RootActionExecutorDeps = {
  paletteWindow: { hidePalette: () => Promise<void> | void };
  recordRecent: (action: any) => void;
  currentActionForStoredShortcut: (action: any) => any;
  openSystemKeyboardSettings: () => unknown;
  executeExtensionRootItem: (action: any) => Promise<any>;
  presentActionResultFeedback: (result: any, action: any) => any;
};

export function createRootActionExecutor(deps: RootActionExecutorDeps) {
  const {
    paletteWindow,
    recordRecent,
    currentActionForStoredShortcut,
    openSystemKeyboardSettings,
    executeExtensionRootItem,
    presentActionResultFeedback,
  } = deps;

  async function executeActionWithoutFeedback(action, options: any = {}) {
    if (!action) return;
    recordRecent(action);
    let result;

    switch (action.kind) {
      case 'test-action':
        if (isNvmTestMode) {
          result = { toast: { message: 'Safe action invoked' } };
        }
        break;
      case 'open-keyboard-settings':
        runInBackground(openSystemKeyboardSettings, action.traceId);
        break;
      case 'extension-root-item':
      case 'extension-action': {
        const result = await executeExtensionRootItem(action);
        if (result) return result;
        break;
      }
      case 'extension-command': {
        const upgraded = currentActionForStoredShortcut(action);
        if (upgraded !== action)
          return executeActionWithoutFeedback(upgraded, options);
        break;
      }
    }

    if (!options.keepPaletteOpen) {
      if (isNvmTestMode && action.kind === 'test-action')
        setTimeout(() => paletteWindow.hidePalette(), 0).unref?.();
      else await paletteWindow.hidePalette();
    }
    return result;
  }

  async function executeAction(action, options: any = {}) {
    return presentActionResultFeedback(
      await executeActionWithoutFeedback(action, options),
      action,
    );
  }

  return { executeAction };
}

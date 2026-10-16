// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic action shapes by design.
import { quickLookTitle, revealPathTitle, settingsTitle } from '../os/os';
import {
  denyShortcut,
  trustedContextCapabilities,
  trustedExtensionApiUnavailable,
} from './extension-context-capabilities';

export type ContextActionsDeps = {
  buildRecordShortcutAction: (input: any, options: any) => unknown;
  buildRemoveShortcutAction: (input: any, options: any) => unknown;
};

function createActionsSystem() {
  const { canUseSystem } = trustedContextCapabilities;
  return canUseSystem
    ? {
        lockScreen: (title = 'Lock Screen', options: any = {}) => ({
          dismissAfterRun: 'auto',
          ...options,
          type: 'lockScreen',
          title,
        }),
        sleep: (title = 'Sleep', options: any = {}) => ({
          dismissAfterRun: 'auto',
          ...options,
          type: 'sleepSystem',
          title,
        }),
        restart: (title = 'Restart Computer', options: any = {}) => ({
          dismissAfterRun: 'auto',
          ...options,
          type: 'restartSystem',
          title,
        }),
        openSystemSettings: (title = settingsTitle(), options: any = {}) => ({
          dismissAfterRun: 'auto',
          ...options,
          type: 'openSystemSettings',
          title,
        }),
        openKeyboardSettings: (
          title = 'Keyboard Settings',
          options: any = {},
        ) => ({
          dismissAfterRun: 'auto',
          ...options,
          type: 'openKeyboardSettings',
          title,
        }),
        quit: (title = 'Quit Nevermind', options: any = {}) => ({
          ...options,
          type: 'quitApp',
          title,
        }),
      }
    : denyShortcut('system');
}

function createActionsUpdates() {
  const { canUseUpdates } = trustedContextCapabilities;
  return canUseUpdates
    ? {
        check: (title = 'Check for Updates', options: any = {}) => ({
          ...options,
          type: 'checkForUpdates',
          title,
        }),
        download: (title = 'Download Update', options: any = {}) => ({
          ...options,
          type: 'downloadUpdate',
          title,
        }),
        install: (title = 'Install and Restart', options: any = {}) => ({
          ...options,
          type: 'installUpdate',
          title,
        }),
      }
    : denyShortcut('updates');
}

function createActionsCamera() {
  return {
    switchDevice: (title = 'Switch Camera', options: any = {}) => ({
      ...options,
      type: 'nativeAction',
      title,
      nativeAction: { kind: 'camera.switchDevice' },
    }),
    nextDevice: (title = 'Next Camera', options: any = {}) => ({
      ...options,
      type: 'nativeAction',
      title,
      nativeAction: { kind: 'camera.nextDevice' },
    }),
    previousDevice: (title = 'Previous Camera', options: any = {}) => ({
      ...options,
      type: 'nativeAction',
      title,
      nativeAction: { kind: 'camera.previousDevice' },
    }),
    toggleMuted: (title = 'Toggle Camera Audio', options: any = {}) => ({
      ...options,
      type: 'nativeAction',
      title,
      nativeAction: { kind: 'camera.toggleMuted' },
    }),
    toggleControls: (title = 'Toggle Camera Controls', options: any = {}) => ({
      ...options,
      type: 'nativeAction',
      title,
      nativeAction: { kind: 'camera.toggleControls' },
    }),
  };
}

export function createContextActions(extension, deps: ContextActionsDeps) {
  const { canUseSystem, canWriteSettings, canUseShortcuts } =
    trustedContextCapabilities;
  const { buildRecordShortcutAction, buildRemoveShortcutAction } = deps;
  return {
    openPath: (filePath, title = 'Open', options: any = {}) => ({
      dismissAfterRun: 'auto',
      ...options,
      type: 'openPath',
      title,
      path: filePath,
    }),
    revealPath: (filePath, title = revealPathTitle(), options: any = {}) => ({
      dismissAfterRun: 'auto',
      ...options,
      type: 'revealPath',
      title,
      path: filePath,
    }),
    quickLook: (filePath, title = quickLookTitle(), options: any = {}) => ({
      ...options,
      type: 'quickLook',
      title,
      path: filePath,
    }),
    openWith: (filePath, app, title, options: any = {}) => ({
      dismissAfterRun: 'auto',
      ...options,
      type: 'openWith',
      title: title || `Open with ${app?.name || 'App'}`,
      path: filePath,
      app,
      appPath: app?.path || app,
    }),
    openUrl: (url, title = 'Open URL', options: any = {}) => ({
      dismissAfterRun: 'auto',
      ...options,
      type: 'openUrl',
      title,
      url,
    }),
    copyText: (text, title = 'Copy', options: any = {}) => ({
      ...options,
      type: 'copyText',
      title,
      text,
    }),
    pasteText: (text, title = 'Paste', options: any = {}) => ({
      ...options,
      type: 'pasteText',
      title,
      text,
    }),
    paste: (content, title = 'Paste', options: any = {}) => ({
      ...options,
      type: 'pasteClipboard',
      title,
      content,
    }),
    ref: (registeredActionId, title = 'Run Action', options: any = {}) => ({
      ...options,
      type: 'runExtensionRegisteredAction',
      title,
      extensionId: extension.id,
      registeredActionId,
    }),
    typeText: (text, title = 'Type Text', options: any = {}) => ({
      ...options,
      type: 'typeText',
      title,
      text,
    }),
    copyImage: (image, title = 'Copy image', options: any = {}) =>
      String(image || '').startsWith('data:')
        ? { ...options, type: 'copyImage', title, imageDataUrl: image }
        : { ...options, type: 'copyImage', title, path: image },
    trash: (paths, title = 'Move to Trash', options: any = {}) => ({
      ...options,
      type: 'trash',
      title,
      paths: Array.isArray(paths) ? paths : [paths],
      style: options.style || 'destructive',
      requiresConfirmation: options.requiresConfirmation ?? true,
    }),
    root: (title, view, options: any = {}) => ({
      ...options,
      type: 'rootView',
      title,
      view,
    }),
    push: (title, view, options: any = {}) => ({
      ...options,
      type: 'pushView',
      title,
      view,
    }),
    replace: (title, view, options: any = {}) => ({
      ...options,
      type: 'replaceView',
      title,
      view,
    }),
    pop: (title = 'Back', options: any = {}) => ({
      ...options,
      type: 'popView',
      title,
    }),
    run: (title, handler, options: any = {}) => ({
      ...options,
      type: 'runExtensionAction',
      title,
      __handler: handler,
    }),
    background: (title, handler, options: any = {}) => ({
      ...options,
      type: 'runExtensionAction',
      title,
      __handler: handler,
      dismissAfterRun: options.dismissAfterRun || 'auto',
    }),
    shellExec: canUseSystem
      ? (title, command, args = [], options: any = {}) => ({
          ...options,
          type: 'shellExec',
          title,
          command,
          args,
          options,
          requiresConfirmation: options.requiresConfirmation ?? true,
        })
      : () => {
          throw trustedExtensionApiUnavailable('system');
        },
    shellScript: canUseSystem
      ? (title, script, options: any = {}) => ({
          ...options,
          type: 'shellScript',
          title,
          script,
          options,
          requiresConfirmation: options.requiresConfirmation ?? true,
        })
      : () => {
          throw trustedExtensionApiUnavailable('system');
        },
    toggleSetting: canWriteSettings
      ? (settingId, title = 'Toggle', options: any = {}) => ({
          ...options,
          type: 'toggleSetting',
          title,
          settingId,
        })
      : denyShortcut('settings.write'),
    recordShortcut: canUseShortcuts
      ? (input: any = {}, options: any = {}) =>
          buildRecordShortcutAction(input, options)
      : denyShortcut('recordShortcut'),
    removeShortcut: canUseShortcuts
      ? (input: any = {}, options: any = {}) =>
          buildRemoveShortcutAction(input, options)
      : denyShortcut('removeShortcut'),
    setPaletteShortcut: canUseShortcuts
      ? (title = 'Change Shortcut', options: any = {}) =>
          buildRecordShortcutAction({ scope: 'palette', title }, options)
      : denyShortcut('setPaletteShortcut'),
    native: (title, nativeAction, options: any = {}) => ({
      ...options,
      type: 'nativeAction',
      title,
      nativeAction,
    }),
    system: createActionsSystem(),
    updates: createActionsUpdates(),
    camera: createActionsCamera(),
  };
}

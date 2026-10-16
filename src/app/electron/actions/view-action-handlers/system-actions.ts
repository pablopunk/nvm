import {
  executeSystemBuiltin,
  forceQuitApp as forceQuitOsApp,
} from '../../os/os';
import { systemSettingsPaneUrl } from '../../os/system-settings';
import { warn as logWarn } from '../../platform/logger';
import { runInBackground } from '../run-in-background';

export type SystemActionHandlerDeps = {
  shell: { openExternal: (url: string) => Promise<void> };
  openSystemKeyboardSettings: () => unknown;
  requestQuitApp: (reason?: string) => unknown;
  checkForUpdatesView: () => any;
  downloadUpdateView: () => any;
  installDownloadedUpdate: () => any;
};

export function createSystemActionHandlers(deps: SystemActionHandlerDeps) {
  const {
    shell,
    openSystemKeyboardSettings,
    requestQuitApp,
    checkForUpdatesView,
    downloadUpdateView,
    installDownloadedUpdate,
  } = deps;

  function handleLockScreen(action) {
    runInBackground(
      () => executeSystemBuiltin({ builtin: 'lock-screen' }, () => {}),
      action.traceId,
    );
  }

  function handleSleepSystem(action) {
    runInBackground(
      () => executeSystemBuiltin({ builtin: 'sleep' }, () => {}),
      action.traceId,
    );
  }

  function handleRestartSystem(action) {
    runInBackground(
      () => executeSystemBuiltin({ builtin: 'restart' }, () => {}),
      action.traceId,
    );
  }

  function handleOpenSystemSettings(action) {
    runInBackground(() => {
      const paneUrl = systemSettingsPaneUrl(
        action.paneId,
        process.platform,
        action.anchor,
      );
      return paneUrl
        ? shell.openExternal(paneUrl)
        : executeSystemBuiltin({ builtin: 'settings' }, () => {});
    }, action.traceId);
  }

  function handleOpenKeyboardSettings(action) {
    runInBackground(openSystemKeyboardSettings, action.traceId);
  }

  function handleQuitApp(action) {
    return requestQuitApp('view-action');
  }

  async function handleForceQuitApp(action) {
    const appPath = action.path || action.appPath || '';
    const appName =
      action.app?.name || action.title?.replace(/^Force Quit /, '') || '';
    try {
      const result = await forceQuitOsApp(appPath, appName);
      if (result.ok) {
        return { toast: { message: `Force quit ${appName}` } };
      }
      return {
        toast: {
          message: result.error || `Could not force quit ${appName}`,
          tone: 'error',
        },
      };
    } catch (error) {
      logWarn('forceQuitApp.failed', error, {
        source: 'host',
        scope: 'apps',
      });
      return {
        toast: { message: `Could not force quit ${appName}`, tone: 'error' },
      };
    }
  }

  function handleCheckForUpdates(action) {
    return checkForUpdatesView();
  }

  function handleDownloadUpdate(action) {
    return downloadUpdateView();
  }

  function handleInstallUpdate(action) {
    return installDownloadedUpdate();
  }

  return {
    handleLockScreen,
    handleSleepSystem,
    handleRestartSystem,
    handleOpenSystemSettings,
    handleOpenKeyboardSettings,
    handleQuitApp,
    handleForceQuitApp,
    handleCheckForUpdates,
    handleDownloadUpdate,
    handleInstallUpdate,
  };
}

import { app } from 'electron';
import * as logger from './logger';
import { supportsAutoUpdates } from './os';

type UpdateInfo = { version?: string };
type AutoUpdaterLike = {
  autoDownload: boolean;
  autoInstallOnAppQuit: boolean;
  checkForUpdates: () => Promise<
    { updateInfo?: UpdateInfo } | null | undefined
  >;
  downloadUpdate: () => Promise<unknown>;
  quitAndInstall: () => void;
  on: (event: string, listener: (...args: any[]) => void) => void;
};

type UpdateStatus =
  | 'idle'
  | 'unsupported'
  | 'checking'
  | 'available'
  | 'downloading'
  | 'downloaded'
  | 'installing'
  | 'error';

type UpdateState = {
  checkInFlight: boolean;
  downloadInFlight: boolean;
  installInFlight: boolean;
  status: UpdateStatus;
  availableInfo: UpdateInfo | null;
  downloadedInfo: UpdateInfo | null;
  errorMessage: string;
};

const AUTO_UPDATE_STARTUP_DELAY_MS = 15_000;
const AUTO_UPDATE_POLL_INTERVAL_MS = 4 * 60 * 60 * 1000;

export function createUpdateManager(autoUpdater: AutoUpdaterLike) {
  let startupTimer: NodeJS.Timeout | null = null;
  let pollTimer: NodeJS.Timeout | null = null;
  const stateListeners = new Set<() => void>();
  function notifyStateChanged() {
    for (const listener of stateListeners) {
      try {
        listener();
      } catch {}
    }
  }
  function onStateChange(listener: () => void) {
    stateListeners.add(listener);
    return () => stateListeners.delete(listener);
  }
  const state: UpdateState = {
    checkInFlight: false,
    downloadInFlight: false,
    installInFlight: false,
    status: 'idle',
    availableInfo: null,
    downloadedInfo: null,
    errorMessage: '',
  };

  function canUseAutoUpdates() {
    return app.isPackaged && supportsAutoUpdates();
  }

  async function checkForUpdates(
    trigger = 'manual',
    options: { download?: boolean } = {},
  ) {
    if (!canUseAutoUpdates()) {
      state.status = 'unsupported';
      notifyStateChanged();
      return null;
    }
    if (state.downloadedInfo) return state.downloadedInfo;
    if (state.checkInFlight || state.downloadInFlight || state.installInFlight)
      return state.availableInfo || state.downloadedInfo;
    state.checkInFlight = true;
    state.status = 'checking';
    state.errorMessage = '';
    notifyStateChanged();
    try {
      logger.info(
        'updater.checking',
        { trigger },
        { source: 'host', scope: 'updater' },
      );
      const result = await autoUpdater.checkForUpdates();
      if (options.download && result?.updateInfo && !state.downloadedInfo)
        await downloadAvailableUpdate(result.updateInfo);
      return result?.updateInfo || null;
    } catch (error) {
      state.status = 'error';
      state.errorMessage =
        error instanceof Error ? error.message : String(error);
      logger.error('updater.check.failed', error, {
        source: 'host',
        scope: 'updater',
      });
      return null;
    } finally {
      state.checkInFlight = false;
      if (state.status === 'checking')
        state.status = state.availableInfo ? 'available' : 'idle';
      notifyStateChanged();
    }
  }

  async function downloadAvailableUpdate(info = state.availableInfo) {
    if (
      !canUseAutoUpdates() ||
      state.downloadInFlight ||
      state.installInFlight ||
      state.downloadedInfo
    )
      return;
    state.availableInfo = info || state.availableInfo;
    state.downloadInFlight = true;
    state.status = 'downloading';
    state.errorMessage = '';
    notifyStateChanged();
    try {
      await autoUpdater.downloadUpdate();
    } catch (error) {
      state.status = 'error';
      state.errorMessage =
        error instanceof Error ? error.message : String(error);
      logger.error('updater.download.failed', error, {
        source: 'host',
        scope: 'updater',
      });
    } finally {
      state.downloadInFlight = false;
      if (state.status === 'downloading')
        state.status = state.downloadedInfo
          ? 'downloaded'
          : state.availableInfo
            ? 'available'
            : 'idle';
      notifyStateChanged();
    }
  }

  function clearTimers() {
    if (startupTimer) {
      clearTimeout(startupTimer);
      startupTimer = null;
    }
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function configure() {
    if (!canUseAutoUpdates()) {
      logger.info('updater.disabled', undefined, {
        source: 'host',
        scope: 'updater',
      });
      return;
    }

    autoUpdater.autoDownload = false;
    autoUpdater.autoInstallOnAppQuit = false;

    autoUpdater.on('checking-for-update', () => {
      state.status = 'checking';
      logger.info('updater.checking', undefined, {
        source: 'host',
        scope: 'updater',
      });
      notifyStateChanged();
    });

    autoUpdater.on('update-available', (info: UpdateInfo) => {
      state.availableInfo = info;
      state.status = 'available';
      logger.info('updater.available', info, {
        source: 'host',
        scope: 'updater',
      });
      notifyStateChanged();
    });

    autoUpdater.on('update-not-available', () => {
      state.availableInfo = null;
      state.downloadedInfo = null;
      state.status = 'idle';
      logger.info('updater.notAvailable', undefined, {
        source: 'host',
        scope: 'updater',
      });
      notifyStateChanged();
    });

    autoUpdater.on('download-progress', (progress: { percent: number }) => {
      logger.info(
        'updater.download.progress',
        { percent: Math.floor(progress.percent) },
        { source: 'host', scope: 'updater' },
      );
      notifyStateChanged();
    });

    autoUpdater.on('update-downloaded', (info: UpdateInfo) => {
      state.downloadedInfo = info;
      state.availableInfo = info;
      state.status = 'downloaded';
      logger.info('updater.downloaded', info, {
        source: 'host',
        scope: 'updater',
      });
      notifyStateChanged();
    });

    autoUpdater.on('error', (error: Error) => {
      state.status = 'error';
      state.errorMessage = error?.message || String(error);
      logger.error('updater.error', error, {
        source: 'host',
        scope: 'updater',
      });
      notifyStateChanged();
    });

    startupTimer = setTimeout(() => {
      startupTimer = null;
      void checkForUpdates('startup');
    }, AUTO_UPDATE_STARTUP_DELAY_MS);
    startupTimer.unref?.();

    pollTimer = setInterval(() => {
      void checkForUpdates('poll');
    }, AUTO_UPDATE_POLL_INTERVAL_MS);
    pollTimer.unref?.();
  }

  function quitAndInstall() {
    if (!state.downloadedInfo || state.installInFlight) return false;
    state.installInFlight = true;
    state.status = 'installing';
    state.errorMessage = '';
    logger.info('updater.install.requested', state.downloadedInfo, {
      source: 'host',
      scope: 'updater',
    });
    notifyStateChanged();
    try {
      autoUpdater.quitAndInstall();
      return true;
    } catch (error) {
      state.installInFlight = false;
      state.status = 'error';
      state.errorMessage =
        error instanceof Error ? error.message : String(error);
      logger.error('updater.install.failed', error, {
        source: 'host',
        scope: 'updater',
      });
      notifyStateChanged();
      return false;
    }
  }

  return {
    state,
    canUseAutoUpdates,
    checkForUpdates,
    downloadAvailableUpdate,
    clearTimers,
    configure,
    quitAndInstall,
    onStateChange,
  };
}

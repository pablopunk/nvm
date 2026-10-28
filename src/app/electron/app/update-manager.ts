import { app } from 'electron';
import * as logger from '../platform/logger';
import { supportsAutoUpdates } from '../os/os';
import {
  bindOperation,
  operationOutcome,
  operationStage,
  recordOperationFailure,
  runOperation,
} from '../diagnostics/observability';
import {
  DEFAULT_UPDATE_CHANNEL,
  isUpdateInfoForChannel,
  type UpdateChannel,
} from './update-channel';

type UpdateInfo = { version?: string; updateChannel?: UpdateChannel };
type UpdateCheckResult = {
  isUpdateAvailable: boolean;
  updateInfo?: UpdateInfo;
};
type CancellationTokenLike = {
  readonly cancelled: boolean;
  cancel: () => void;
};
type AutoUpdaterLike = {
  autoDownload: boolean;
  autoInstallOnAppQuit: boolean;
  allowDowngrade: boolean;
  checkForUpdates: () => Promise<UpdateCheckResult | null | undefined>;
  downloadUpdate: (
    cancellationToken?: CancellationTokenLike,
  ) => Promise<unknown>;
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
  latestVersion: string | null;
  errorMessage: string;
};

export type UpdateManagerOptions = {
  getUpdateChannel?: () => UpdateChannel;
  prepareUpdateFeed?: () => Promise<unknown>;
  createCancellationToken?: () => CancellationTokenLike;
};

const AUTO_UPDATE_STARTUP_DELAY_MS = 15_000;
const AUTO_UPDATE_POLL_INTERVAL_MS = 4 * 60 * 60 * 1000;

function createLocalCancellationToken(): CancellationTokenLike {
  let cancelled = false;
  return {
    get cancelled() {
      return cancelled;
    },
    cancel() {
      cancelled = true;
    },
  };
}

export function createUpdateManager(
  autoUpdater: AutoUpdaterLike,
  onInstallRequested?: (version?: string) => void,
  managerOptions: UpdateManagerOptions = {},
) {
  const getUpdateChannel =
    managerOptions.getUpdateChannel ?? (() => DEFAULT_UPDATE_CHANNEL);
  const createCancellationToken =
    managerOptions.createCancellationToken ?? createLocalCancellationToken;
  let startupTimer: NodeJS.Timeout | null = null;
  let pollTimer: NodeJS.Timeout | null = null;
  let installStarted = false;
  let captureActiveFailure: ((error: unknown) => void) | undefined;
  let channelFeedReady: Promise<void> | null = null;
  let downloadCancellation: CancellationTokenLike | null = null;
  let recheckWhenIdle = false;
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
    latestVersion: null,
    errorMessage: '',
  };

  function canUseAutoUpdates() {
    return app.isPackaged && supportsAutoUpdates();
  }

  function isCurrentChannelInfo(info: unknown) {
    return isUpdateInfoForChannel(info, getUpdateChannel());
  }

  function logIgnoredChannelResult(event: string, info?: UpdateInfo) {
    logger.info(
      'updater.staleChannelResult',
      { event, version: info?.version, channel: info?.updateChannel },
      { source: 'host', scope: 'updater' },
    );
  }

  function prepareUpdateFeed() {
    const ready = Promise.resolve(managerOptions.prepareUpdateFeed?.()).then(
      () => undefined,
    );
    ready.catch((error) =>
      logger.error('updater.feed.failed', error, {
        source: 'host',
        scope: 'updater',
      }),
    );
    return ready;
  }

  function runDeferredChannelRecheck() {
    if (
      !recheckWhenIdle ||
      state.checkInFlight ||
      state.downloadInFlight ||
      state.installInFlight
    )
      return;
    recheckWhenIdle = false;
    void checkForUpdates('channel');
  }

  function checkForUpdates(
    trigger = 'manual',
    options: { download?: boolean } = {},
  ) {
    return runOperation('update.check', () => checkUpdate(trigger, options));
  }

  async function checkUpdate(
    trigger = 'manual',
    options: { download?: boolean } = {},
  ) {
    if (!canUseAutoUpdates()) {
      operationOutcome('blocked');
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
    const capture = bindOperation((error: unknown) => {
      recordOperationFailure(error, 'request');
    });
    const previousCapture = captureActiveFailure;
    captureActiveFailure = capture;
    operationStage('request');
    try {
      logger.info(
        'updater.checking',
        { trigger },
        { source: 'host', scope: 'updater' },
      );
      await channelFeedReady;
      const result = await autoUpdater.checkForUpdates();
      if (
        options.download &&
        result?.isUpdateAvailable &&
        result.updateInfo &&
        isCurrentChannelInfo(result.updateInfo) &&
        !state.downloadedInfo
      )
        await downloadAvailableUpdate(result.updateInfo);
      return result?.updateInfo || null;
    } catch (error) {
      recordOperationFailure(error, 'request');
      state.status = 'error';
      state.errorMessage =
        error instanceof Error ? error.message : String(error);
      logger.error('updater.check.failed', error, {
        source: 'host',
        scope: 'updater',
      });
      return null;
    } finally {
      if (captureActiveFailure === capture)
        captureActiveFailure = previousCapture;
      state.checkInFlight = false;
      if (state.status === 'checking')
        state.status = state.availableInfo ? 'available' : 'idle';
      notifyStateChanged();
      runDeferredChannelRecheck();
    }
  }

  function downloadAvailableUpdate(info = state.availableInfo) {
    return runOperation('update.download', () => downloadUpdate(info));
  }

  async function downloadUpdate(info = state.availableInfo) {
    const availableInfo = info || state.availableInfo;
    if (
      !canUseAutoUpdates() ||
      state.downloadInFlight ||
      state.installInFlight ||
      state.downloadedInfo ||
      !availableInfo ||
      !isCurrentChannelInfo(availableInfo)
    )
      return;
    state.availableInfo = availableInfo;
    state.downloadInFlight = true;
    const cancellation = createCancellationToken();
    downloadCancellation = cancellation;
    state.status = 'downloading';
    state.errorMessage = '';
    notifyStateChanged();
    const capture = bindOperation((error: unknown) => {
      recordOperationFailure(error, 'save');
    });
    const previousCapture = captureActiveFailure;
    captureActiveFailure = capture;
    operationStage('save');
    try {
      await autoUpdater.downloadUpdate(cancellation);
    } catch (error) {
      if (cancellation.cancelled) {
        logger.info(
          'updater.download.cancelled',
          { version: availableInfo.version },
          { source: 'host', scope: 'updater' },
        );
      } else {
        recordOperationFailure(error, 'save');
        state.status = 'error';
        state.errorMessage =
          error instanceof Error ? error.message : String(error);
        logger.error('updater.download.failed', error, {
          source: 'host',
          scope: 'updater',
        });
      }
    } finally {
      if (captureActiveFailure === capture)
        captureActiveFailure = previousCapture;
      if (downloadCancellation === cancellation) downloadCancellation = null;
      state.downloadInFlight = false;
      if (state.status === 'downloading')
        state.status = state.downloadedInfo
          ? 'downloaded'
          : state.availableInfo
            ? 'available'
            : 'idle';
      notifyStateChanged();
      runDeferredChannelRecheck();
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

  function discardUpdateChannelState() {
    if (!canUseAutoUpdates() || state.installInFlight) return;
    downloadCancellation?.cancel();
    state.availableInfo = null;
    state.downloadedInfo = null;
    state.latestVersion = null;
    state.errorMessage = '';
    if (
      state.status === 'available' ||
      state.status === 'downloaded' ||
      state.status === 'error'
    )
      state.status = 'idle';
    notifyStateChanged();
  }

  function handleUpdateChannelChanged() {
    if (!canUseAutoUpdates() || state.installInFlight) return;
    logger.info(
      'updater.channel.changed',
      { channel: getUpdateChannel() },
      { source: 'host', scope: 'updater' },
    );
    discardUpdateChannelState();
    recheckWhenIdle = true;
    runDeferredChannelRecheck();
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
    autoUpdater.allowDowngrade = false;
    channelFeedReady = prepareUpdateFeed();

    autoUpdater.on('checking-for-update', () => {
      state.status = 'checking';
      logger.info('updater.checking', undefined, {
        source: 'host',
        scope: 'updater',
      });
      notifyStateChanged();
    });

    autoUpdater.on('update-available', (info: UpdateInfo) => {
      if (!isCurrentChannelInfo(info)) {
        logIgnoredChannelResult('update-available', info);
        return;
      }
      state.availableInfo = info;
      state.latestVersion = info.version ?? null;
      state.status = 'available';
      logger.info('updater.available', info, {
        source: 'host',
        scope: 'updater',
      });
      notifyStateChanged();
    });

    autoUpdater.on('update-not-available', (info?: UpdateInfo) => {
      if (!isCurrentChannelInfo(info)) {
        logIgnoredChannelResult('update-not-available', info);
        return;
      }
      state.availableInfo = null;
      state.downloadedInfo = null;
      state.latestVersion = info?.version ?? null;
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
      if (!isCurrentChannelInfo(info)) {
        logIgnoredChannelResult('update-downloaded', info);
        return;
      }
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
      if (captureActiveFailure) captureActiveFailure(error);
      else
        runOperation('update.check', () => {
          recordOperationFailure(error, 'request');
        });
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

  function prepareInstall() {
    if (
      !state.downloadedInfo ||
      state.installInFlight ||
      !isCurrentChannelInfo(state.downloadedInfo)
    )
      return false;
    state.installInFlight = true;
    onInstallRequested?.(state.downloadedInfo.version);
    state.status = 'installing';
    state.errorMessage = '';
    logger.info('updater.install.requested', state.downloadedInfo, {
      source: 'host',
      scope: 'updater',
    });
    notifyStateChanged();
    return true;
  }

  function quitAndInstall() {
    return runOperation('update.install', installUpdate, {
      defaultOutcome: 'unknown',
    });
  }

  function installUpdate() {
    if (!state.downloadedInfo || installStarted) return false;
    if (!state.installInFlight && !prepareInstall()) return false;
    installStarted = true;
    operationStage('install');
    try {
      autoUpdater.quitAndInstall();
      return true;
    } catch (error) {
      recordOperationFailure(error, 'install');
      installStarted = false;
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
    prepareInstall,
    quitAndInstall,
    onStateChange,
    discardUpdateChannelState,
    handleUpdateChannelChanged,
  };
}

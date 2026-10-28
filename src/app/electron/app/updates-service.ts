// biome-ignore-all lint: Update views and prompt actions are dynamic view payloads moved verbatim from main.ts.
import { currentNevermindCompatibilityManifest } from '../backend/nevermind-compatibility';
import { isNewerVersion as isVersionNewerThan } from '../platform/version-utils';
import { type UpdateChannel, updateChannelLabel } from './update-channel';
import type { createUpdateManager } from './update-manager';

export type UpdatesServiceDeps = {
  updateManager: ReturnType<typeof createUpdateManager>;
  getAppVersion: () => string;
  autoUpdatesUnavailableMessage: () => string;
  getActiveNevermindBaseUrl: () => string | null;
  getUpdateChannel: () => UpdateChannel;
  markAppQuitting: () => void;
  requestQuit: (reason: string, quit: () => void) => Promise<void>;
  quitApp: () => void;
  patchOpenView: (viewId: string, patch: any) => void;
};

export function safeExternalUpdateUrl(raw?: string) {
  if (!raw) return null;
  try {
    const parsed = new URL(raw);
    return parsed.protocol === 'https:' || parsed.protocol === 'http:'
      ? parsed.toString()
      : null;
  } catch {
    return null;
  }
}

export function createUpdatesService(deps: UpdatesServiceDeps) {
  const {
    updateManager,
    getAppVersion,
    autoUpdatesUnavailableMessage,
    getActiveNevermindBaseUrl,
    getUpdateChannel,
    markAppQuitting,
    requestQuit,
    quitApp,
    patchOpenView,
  } = deps;

  function isNewerVersion(version) {
    return isVersionNewerThan(version, getAppVersion());
  }

  function updateStatusItems() {
    return updateStatusView().items;
  }

  function updateStatusView(_options: any = {}) {
    const downloadedInfo = isNewerVersion(
      updateManager.state.downloadedInfo?.version,
    )
      ? updateManager.state.downloadedInfo
      : null;
    const availableInfo = isNewerVersion(
      updateManager.state.availableInfo?.version,
    )
      ? updateManager.state.availableInfo
      : null;
    const version = downloadedInfo?.version || availableInfo?.version;
    const unsupported =
      updateManager.state.status === 'unsupported' ||
      !updateManager.canUseAutoUpdates();
    const installing =
      updateManager.state.installInFlight ||
      updateManager.state.status === 'installing';
    const primaryAction = installing
      ? undefined
      : downloadedInfo
        ? { type: 'installUpdate', title: 'Install and Restart' }
        : availableInfo
          ? {
              type: 'downloadUpdate',
              title: updateManager.state.downloadInFlight
                ? 'Downloading...'
                : 'Download Update',
            }
          : {
              type: 'checkForUpdates',
              title: updateManager.state.checkInFlight
                ? 'Checking...'
                : 'Check Again',
            };
    const channelName = updateChannelLabel(getUpdateChannel());
    const currentVersionLine = `Current version: ${getAppVersion()} · ${channelName} channel`;
    const aheadOfLatest =
      !downloadedInfo &&
      !availableInfo &&
      Boolean(updateManager.state.latestVersion) &&
      isVersionNewerThan(getAppVersion(), updateManager.state.latestVersion);
    const title = unsupported
      ? 'Updates unavailable'
      : installing
        ? `Installing Nevermind ${version || ''}`.trim()
        : downloadedInfo
          ? `Nevermind ${version} is ready`
          : availableInfo
            ? `Nevermind ${version} is available`
            : updateManager.state.checkInFlight
              ? 'Checking for updates...'
              : updateManager.state.status === 'error'
                ? 'Update check failed'
                : aheadOfLatest
                  ? `Nevermind ${getAppVersion()} is ahead of the latest ${channelName} release`
                  : 'No versions available';
    const subtitle = unsupported
      ? autoUpdatesUnavailableMessage()
      : installing
        ? 'Restarting Nevermind to finish updating...'
        : downloadedInfo
          ? 'Install the downloaded update and restart Nevermind'
          : availableInfo
            ? 'Download the update before installing it'
            : updateManager.state.checkInFlight
              ? currentVersionLine
              : updateManager.state.status === 'error'
                ? updateManager.state.errorMessage
                : aheadOfLatest
                  ? `Nevermind won't downgrade automatically. ${channelName} updates resume with the next newer ${channelName.toLowerCase()} release.`
                  : currentVersionLine;
    return {
      type: 'list',
      id: 'app-updates',
      title: 'Updates',
      contentSizing: 'fit',
      presentation: 'root',
      searchBarPlaceholder: 'Search Updates',
      isLoading:
        updateManager.state.checkInFlight ||
        updateManager.state.downloadInFlight ||
        updateManager.state.installInFlight,
      items: [
        {
          id: 'update-status',
          title,
          subtitle,
          icon: 'restart',
          accessories: version ? [{ text: version }] : [],
          primaryAction: unsupported ? undefined : primaryAction,
          actionPanel:
            unsupported || !primaryAction
              ? undefined
              : { sections: [{ actions: [primaryAction] }] },
        },
      ],
    };
  }

  function updatesStateSnapshot() {
    const state = updateManager.state;
    const downloadedInfo = isNewerVersion(state.downloadedInfo?.version)
      ? state.downloadedInfo
      : null;
    const availableInfo = isNewerVersion(state.availableInfo?.version)
      ? state.availableInfo
      : null;
    return {
      currentVersion: getAppVersion(),
      channel: getUpdateChannel(),
      status: String(state.status || 'idle'),
      supported:
        updateManager.canUseAutoUpdates() && state.status !== 'unsupported',
      checking: Boolean(state.checkInFlight),
      downloading: Boolean(state.downloadInFlight),
      installing: Boolean(
        state.installInFlight || state.status === 'installing',
      ),
      availableVersion: availableInfo?.version || null,
      downloadedVersion: downloadedInfo?.version || null,
      errorMessage: state.errorMessage || null,
    };
  }

  function checkForUpdatesView() {
    updateManager.checkForUpdates('manual', { download: true }).catch(() => {});
    return { view: updateStatusView(), navigation: 'replace' };
  }

  function downloadUpdateView() {
    updateManager.downloadAvailableUpdate().catch(() => {});
    return { view: updateStatusView(), navigation: 'replace' };
  }

  function installDownloadedUpdate() {
    if (!updateManager.prepareInstall())
      return { view: updateStatusView(), navigation: 'replace' };
    markAppQuitting();
    void requestQuit('updater', () => {
      const didStart = updateManager.quitAndInstall();
      if (!didStart) quitApp();
    });
    return { view: updateStatusView(), navigation: 'replace' };
  }

  function patchUpdatesView() {
    patchOpenView('app-updates', {
      mode: 'patch',
      items: updateStatusItems(),
      isLoading:
        updateManager.state.checkInFlight ||
        updateManager.state.downloadInFlight ||
        updateManager.state.installInFlight,
    });
  }

  function updateActionForCompatibilityPrompt(updateUrl?: string) {
    const safeUrl = safeExternalUpdateUrl(updateUrl);
    const downloadedInfo = isNewerVersion(
      updateManager.state.downloadedInfo?.version,
    )
      ? updateManager.state.downloadedInfo
      : null;
    const availableInfo = isNewerVersion(
      updateManager.state.availableInfo?.version,
    )
      ? updateManager.state.availableInfo
      : null;
    if (downloadedInfo)
      return {
        type: 'installUpdate',
        title: `Install Nevermind ${downloadedInfo.version || ''}`.trim(),
      };
    if (availableInfo)
      return {
        type: 'downloadUpdate',
        title: `Download Nevermind ${availableInfo.version || ''}`.trim(),
      };
    if (updateManager.canUseAutoUpdates())
      return { type: 'checkForUpdates', title: 'Check for Update' };
    return safeUrl
      ? { type: 'openUrl', title: 'Download Update', url: safeUrl }
      : undefined;
  }

  function compatibilityPromptAction() {
    const activeNevermindBaseUrl = getActiveNevermindBaseUrl();
    if (!activeNevermindBaseUrl) return null;
    const manifest = currentNevermindCompatibilityManifest(
      activeNevermindBaseUrl,
    );
    if (manifest?.client?.compatible !== false) return null;
    const version =
      manifest.desktop?.latestVersion ||
      manifest.desktop?.minimumSupportedVersion ||
      '';
    const primaryAction = updateActionForCompatibilityPrompt(
      manifest.desktop?.updateUrl,
    );
    return {
      id: 'updates:compatibility-required',
      title: 'Update Nevermind',
      subtitle: version
        ? `Nevermind ${version} or newer is required for backend compatibility`
        : 'This version is no longer supported by the backend',
      icon: 'restart',
      score: 1100,
      primaryAction,
      actionPanel: primaryAction
        ? { sections: [{ actions: [primaryAction] }] }
        : undefined,
    };
  }

  function updatePromptAction() {
    const downloadedInfo = isNewerVersion(
      updateManager.state.downloadedInfo?.version,
    )
      ? updateManager.state.downloadedInfo
      : null;
    const availableInfo = isNewerVersion(
      updateManager.state.availableInfo?.version,
    )
      ? updateManager.state.availableInfo
      : null;
    const version = downloadedInfo?.version || availableInfo?.version;
    if (
      updateManager.state.installInFlight ||
      updateManager.state.status === 'installing'
    ) {
      return {
        id: 'updates:installing',
        title: `Installing Nevermind ${version || ''}`.trim(),
        subtitle: 'Restarting Nevermind to finish updating...',
        icon: 'restart',
        score: 1000,
      };
    }
    if (downloadedInfo) {
      return {
        id: 'updates:install',
        title: `Install Nevermind ${version}`,
        subtitle: 'Restart Nevermind to finish updating',
        icon: 'restart',
        score: 1000,
        primaryAction: {
          type: 'installUpdate',
          title: `Install Nevermind ${version}`,
        },
      };
    }
    if (availableInfo) {
      return {
        id: 'updates:download',
        title: `Download Nevermind ${version}`,
        subtitle: updateManager.state.downloadInFlight
          ? 'Downloading update...'
          : 'Update available',
        icon: 'restart',
        score: 1000,
        primaryAction: {
          type: 'downloadUpdate',
          title: `Download Nevermind ${version}`,
        },
      };
    }
    return null;
  }

  return {
    isNewerVersion,
    updateStatusItems,
    updateStatusView,
    updatesStateSnapshot,
    checkForUpdatesView,
    downloadUpdateView,
    installDownloadedUpdate,
    patchUpdatesView,
    updateActionForCompatibilityPrompt,
    compatibilityPromptAction,
    updatePromptAction,
  };
}

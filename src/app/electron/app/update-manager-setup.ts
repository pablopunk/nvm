import electronUpdater from 'electron-updater';
import { app } from 'electron';
import { defaultSharedStateRoot } from '../platform/shared-state-root';
import type { UpdateChannel } from './update-channel';
import { installNevermindChannelFeed } from './update-channel-feed';
import { createUpdateManager } from './update-manager';
import { isStateSafeForInstall } from './state-update-safety';
import { portableExecutableFromEnvironment } from '../os/windows-portable-update';
import { WindowsPortableUpdater } from './windows-portable-updater';
import { reportPreviousPortableUpdate } from './portable-update-outcome';

function createPlatformAutoUpdater() {
  const portableExecutable = portableExecutableFromEnvironment();
  return portableExecutable
    ? new WindowsPortableUpdater(portableExecutable)
    : electronUpdater.autoUpdater;
}

export function createAppUpdateManager(input: {
  isTestMode: boolean;
  recordInstallAttempt: (version?: string) => void;
  getUpdateChannel: () => UpdateChannel;
}) {
  if (input.isTestMode) {
    return {
      state: { status: 'unsupported' as const },
      canUseAutoUpdates: () => false,
      configure: () => {},
      onStateChange: () => {},
      checkForUpdates: async () => {},
      downloadAvailableUpdate: async () => {},
      quitAndInstall: () => false,
      clearTimers: () => {},
      discardUpdateChannelState: () => {},
      handleUpdateChannelChanged: () => {},
    };
  }
  const autoUpdater = createPlatformAutoUpdater();
  const manager = createUpdateManager(
    autoUpdater as any,
    input.recordInstallAttempt,
    {
      getUpdateChannel: input.getUpdateChannel,
      getCurrentVersion: () => app.getVersion(),
      canInstallUpdate: (info) =>
        isStateSafeForInstall(app.getVersion(), info, [
          app.getPath('userData'),
          defaultSharedStateRoot(),
        ]),
      prepareUpdateFeed: () =>
        installNevermindChannelFeed(autoUpdater as any, input.getUpdateChannel),
      createCancellationToken: () => new electronUpdater.CancellationToken(),
    },
  );
  if (autoUpdater instanceof WindowsPortableUpdater)
    void reportPreviousPortableUpdate(manager.state).catch(() => {});
  return manager;
}

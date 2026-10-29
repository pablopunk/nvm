import electronUpdater from 'electron-updater';
import { app } from 'electron';
import os from 'node:os';
import path from 'node:path';
import type { UpdateChannel } from './update-channel';
import { installNevermindChannelFeed } from './update-channel-feed';
import { createUpdateManager } from './update-manager';
import { isStateSafeForInstall } from './state-update-safety';

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
  const { autoUpdater } = electronUpdater;
  return createUpdateManager(autoUpdater as any, input.recordInstallAttempt, {
    getUpdateChannel: input.getUpdateChannel,
    canInstallUpdate: (info) =>
      isStateSafeForInstall(app.getVersion(), info, [
        app.getPath('userData'),
        path.join(os.homedir(), '.config', 'nevermind'),
      ]),
    prepareUpdateFeed: () =>
      installNevermindChannelFeed(autoUpdater as any, input.getUpdateChannel),
    createCancellationToken: () => new electronUpdater.CancellationToken(),
  });
}

import { requestMacAppQuit, runningMacAppForUninstall } from '../os/app-quit';
import { findMacAppsWithBundleId } from '../os/app-uninstall-search';
import { warn } from '../platform/logger';
import {
  createProductionAppUninstallService,
  NEVERMIND_BUNDLE_ID,
} from './app-uninstall-service';

function nevermindBundlePath() {
  return process.execPath.match(/^(.*\.app)(?:\/|$)/)?.[1] || null;
}

export function createHostAppUninstallService(input: {
  trashItem: (itemPath: string) => Promise<void>;
}) {
  return createProductionAppUninstallService({
    trashItem: async (itemPath) => {
      try {
        await input.trashItem(itemPath);
      } catch (error) {
        const code =
          error && typeof error === 'object' && 'code' in error
            ? String(error.code)
            : 'unknown';
        const message =
          error instanceof Error
            ? error.message
                .replaceAll(itemPath, '[item]')
                .replaceAll(os.homedir(), '~')
            : 'Unknown error';
        warn('app-uninstall.trash-failed', { code, message });
        throw error;
      }
    },
    requestQuit: requestMacAppQuit,
    findMatchingApps: findMacAppsWithBundleId,
    runningAppPaths: runningMacAppForUninstall,
    nevermindAppPath: nevermindBundlePath(),
    nevermindBundleId: NEVERMIND_BUNDLE_ID,
  });
}
import os from 'node:os';

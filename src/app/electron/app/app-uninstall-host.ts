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
        warn('app-uninstall.trash-failed', { code });
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

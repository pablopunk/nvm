import { requestMacAppQuit, runningMacAppForUninstall } from '../os/app-quit';
import { findMacAppsWithBundleId } from '../os/app-uninstall-search';
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
    trashItem: input.trashItem,
    requestQuit: requestMacAppQuit,
    findMatchingApps: findMacAppsWithBundleId,
    runningAppPaths: runningMacAppForUninstall,
    nevermindAppPath: nevermindBundlePath(),
    nevermindBundleId: NEVERMIND_BUNDLE_ID,
  });
}

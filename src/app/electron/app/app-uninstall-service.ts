import { randomUUID } from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import { createDiscovery } from './app-uninstall-discovery';
import { createAppGuards } from './app-uninstall-guards';
import { createPathInspector } from './app-uninstall-inspect';
import { createOwnershipVerifier } from './app-uninstall-ownership';
import { createProductionPlistReader } from './app-uninstall-plist';
import { createTrashRunner, selectedCandidates } from './app-uninstall-trash';
import type { AppUninstallDependencies, Stat } from './app-uninstall-types';

export {
  NEVERMIND_BUNDLE_ID,
  validateBundleId,
} from './app-uninstall-bundle-id';
export {
  createProductionPlistReader,
  PLUTIL_OPTIONS,
  PLUTIL_PATH,
} from './app-uninstall-plist';
export type {
  AppUninstallCandidate,
  AppUninstallDependencies,
  AppUninstallNote,
  AppUninstallSnapshot,
  DiscoveryResult,
  TrashResult,
} from './app-uninstall-types';

type ProductionInput = Omit<
  Partial<AppUninstallDependencies>,
  | 'lstat'
  | 'realpath'
  | 'access'
  | 'readBundleId'
  | 'listDirectory'
  | 'randomId'
> &
  Pick<
    AppUninstallDependencies,
    'trashItem' | 'runningAppPaths' | 'requestQuit'
  >;

export function createAppUninstallService(deps: AppUninstallDependencies) {
  const randomId = deps.randomId || randomUUID;
  const inspector = createPathInspector(deps);
  const guards = createAppGuards(deps, inspector);
  const discover = createDiscovery({
    deps,
    randomId,
    inspector,
    guards,
    verifyOwnership: createOwnershipVerifier(deps),
  });
  const { trash } = createTrashRunner({ deps, guards });
  return { discover, selected: selectedCandidates, trash };
}

export function createProductionAppUninstallService(input: ProductionInput) {
  return createAppUninstallService({
    platform: input.platform || process.platform,
    homeDirectory: input.homeDirectory || os.homedir(),
    currentUid: input.currentUid ?? process.getuid?.() ?? -1,
    lstat: (value) => fs.lstat(value) as Promise<Stat>,
    realpath: (value) => fs.realpath(value),
    access: (value, mode) => fs.access(value, mode),
    listDirectory: (value) => fs.readdir(value),
    readBundleId: createProductionPlistReader(),
    findMatchingApps: input.findMatchingApps,
    trashItem: input.trashItem,
    requestQuit: input.requestQuit,
    runningAppPaths: input.runningAppPaths,
    nevermindAppPath: input.nevermindAppPath,
    nevermindBundleId: input.nevermindBundleId,
    wait: input.wait,
    quitTimeoutMs: input.quitTimeoutMs,
    quitPollMs: input.quitPollMs,
  });
}

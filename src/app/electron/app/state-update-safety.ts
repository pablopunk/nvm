import { compareSemverVersions } from '../platform/version-utils';
import { stateRootsSupportDowngrade } from '../state/state-compatibility';

export async function isStateSafeForInstall(
  currentVersion: string,
  update: { version?: string; nevermindStateEpoch?: unknown },
  stateRoots: readonly string[],
): Promise<boolean> {
  const comparison = compareSemverVersions(update.version, currentVersion);
  if (comparison === null) return false;
  if (comparison >= 0) return true;
  return stateRootsSupportDowngrade(stateRoots, update.nevermindStateEpoch);
}

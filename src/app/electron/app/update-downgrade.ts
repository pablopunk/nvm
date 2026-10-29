import * as logger from '../platform/logger';
import { compareSemverVersions } from '../platform/version-utils';
import type {
  UpdateInfo,
  UpdateManagerOptions,
  UpdateState,
} from './update-manager-types';

/**
 * Older builds are offered only after the host authorizes the target's state
 * format, and are never downloaded without an explicit request.
 */
export function createDowngradeGate(input: {
  state: UpdateState;
  managerOptions: UpdateManagerOptions;
  isCurrentChannelInfo: (info: UpdateInfo) => boolean;
}) {
  const { state, managerOptions, isCurrentChannelInfo } = input;

  function isDowngradeCandidate(info: UpdateInfo) {
    const currentVersion = managerOptions.getCurrentVersion?.();
    if (!currentVersion) return false;
    const comparison = compareSemverVersions(info.version, currentVersion);
    return comparison === null || comparison < 0;
  }

  async function isInstallPermitted(info: UpdateInfo) {
    try {
      return (await managerOptions.canInstallUpdate?.(info)) !== false;
    } catch (error) {
      logger.error('updater.install.compatibility.failed', error, {
        source: 'host',
        scope: 'updater',
      });
      return false;
    }
  }

  function blockDowngrade(info: UpdateInfo) {
    state.availableInfo = null;
    state.latestVersion = info.version ?? null;
    state.blockedDowngradeVersion = info.version ?? null;
    state.status = 'idle';
    logger.info(
      'updater.downgrade.blocked',
      { version: info.version },
      { source: 'host', scope: 'updater' },
    );
  }

  async function adoptDowngradeCandidate(info: UpdateInfo) {
    const permitted = await isInstallPermitted(info);
    if (!isCurrentChannelInfo(info) || state.downloadedInfo) return;
    if (!permitted) return blockDowngrade(info);
    state.availableInfo = info;
    state.latestVersion = info.version ?? null;
    state.status = 'available';
  }

  return {
    isDowngradeCandidate,
    isInstallPermitted,
    blockDowngrade,
    adoptDowngradeCandidate,
  };
}

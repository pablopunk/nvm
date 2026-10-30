import { app } from 'electron';
import * as logger from '../platform/logger';
import {
  confirmPortableUpdateLaunch,
  portableExecutableFromEnvironment,
} from '../os/windows-portable-update';
import {
  isNvmTestMode,
  recordPackagedStartupReady,
} from '../platform/test-mode';
import { preparePortableUpdate } from './windows-portable-updater';

function reportStartupReadyFailure(error: unknown) {
  logger.error('updater.portable.startupReady.failed', error, {
    source: 'host',
    scope: 'updater',
  });
}

/** Packaged CI drives the real helper launch with a local package because test mode blocks update downloads. */
async function startPortableUpdateSmokeIfRequested() {
  const sourcePath = process.env.NVM_PORTABLE_UPDATE_SMOKE_PACKAGE;
  const target = portableExecutableFromEnvironment();
  if (
    !(
      isNvmTestMode &&
      process.env.NVM_PACKAGED_STARTUP_SMOKE === '1' &&
      sourcePath &&
      target
    )
  )
    return;
  const launchHelper = await preparePortableUpdate({
    target,
    sourcePath,
    sha512: process.env.NVM_PORTABLE_UPDATE_SMOKE_SHA512 ?? '',
    version: process.env.NVM_PORTABLE_UPDATE_SMOKE_VERSION ?? '',
  });
  launchHelper();
  app.quit();
}

export function recordStartupReady() {
  recordPackagedStartupReady();
  void confirmPortableUpdateLaunch(app.getVersion()).catch(
    reportStartupReadyFailure,
  );
  void startPortableUpdateSmokeIfRequested().catch(reportStartupReadyFailure);
}

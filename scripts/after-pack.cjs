const fs = require('node:fs');
const path = require('node:path');
const {
  verifyPackagedDependencies,
} = require('./verify-packaged-dependencies.cjs');
const { stageTypeScriptNative } = require('./stage-typescript-native.cjs');
const {
  markAfterPackFinished,
  recordPackagingPhase,
} = require('./lib/packaging-timings.cjs');

// electron-builder only writes app-update.yml for NSIS; the portable updater
// reads the same GitHub feed and cache settings.
function writeWindowsPortableUpdateConfig(resources, packager) {
  const [publish] = [].concat(packager.config.publish || []);
  if (publish?.provider !== 'github' || !publish.owner || !publish.repo)
    throw new Error('Windows portable updates require a GitHub publish target');
  fs.writeFileSync(
    path.join(resources, 'app-update.yml'),
    [
      `owner: ${publish.owner}`,
      `repo: ${publish.repo}`,
      'provider: github',
      `updaterCacheDirName: ${packager.appInfo.updaterCacheDirName}`,
      '',
    ].join('\n'),
  );
}

module.exports = async function verifyPackagedRuntime(context) {
  let phaseStartedAt = Date.now();
  const resources =
    context.electronPlatformName === 'darwin'
      ? path.join(
          context.appOutDir,
          `${context.packager.appInfo.productFilename}.app`,
          'Contents',
          'Resources',
        )
      : path.join(context.appOutDir, 'resources');
  const archive = path.join(resources, 'app.asar');
  if (context.electronPlatformName === 'win32')
    writeWindowsPortableUpdateConfig(resources, context.packager);
  const stagedChecker = stageTypeScriptNative({
    resources,
    platform: context.electronPlatformName,
    arch: require('electron-builder').Arch[context.arch],
  });
  console.log(`Staged ${stagedChecker} native checker`);
  recordPackagingPhase('afterPack: stage native checker', phaseStartedAt);
  phaseStartedAt = Date.now();
  const result = verifyPackagedDependencies(archive);
  recordPackagingPhase(
    'afterPack: verify packaged dependencies',
    phaseStartedAt,
  );
  console.log(
    `Verified ${result.packagesChecked} packaged runtime dependencies`,
  );
  if (
    !process.env.CI ||
    context.arch !== require('electron-builder').Arch[process.arch]
  ) {
    markAfterPackFinished();
    return;
  }
  const product = context.packager.appInfo.productFilename;
  const executable =
    context.electronPlatformName === 'darwin'
      ? path.join(
          context.appOutDir,
          `${product}.app`,
          'Contents',
          'MacOS',
          product,
        )
      : path.join(
          context.appOutDir,
          context.electronPlatformName === 'win32'
            ? `${product}.exe`
            : context.packager.executableName,
        );
  const artifacts = path.join(
    context.packager.projectDir,
    'test-results',
    `packaged-diagnostics-${context.electronPlatformName}-${process.arch}`,
  );
  const piArtifacts =
    process.env.NVM_TEST_PI_PACKAGED === '1'
      ? path.join(
          context.packager.projectDir,
          'test-results',
          `packaged-pi-electron-${context.electronPlatformName}-${process.arch}`,
        )
      : undefined;
  phaseStartedAt = Date.now();
  await require('./packaged-diagnostics-smoke.cjs').packagedDiagnosticsSmoke(
    executable,
    artifacts,
    { archive, piArtifacts },
  );
  recordPackagingPhase('afterPack: packaged diagnostics smoke', phaseStartedAt);
  markAfterPackFinished();
};

const path = require('node:path');
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');
const {
  verifyPackagedDependencies,
} = require('./verify-packaged-dependencies.cjs');
const execFileAsync = promisify(execFile);

module.exports = async function verifyPackagedRuntime(context) {
  const resources =
    context.electronPlatformName === 'darwin'
      ? path.join(
          context.appOutDir,
          `${context.packager.appInfo.productFilename}.app`,
          'Contents',
          'Resources',
        )
      : path.join(context.appOutDir, 'resources');
  const result = verifyPackagedDependencies(path.join(resources, 'app.asar'));
  console.log(
    `Verified ${result.packagesChecked} packaged runtime dependencies`,
  );
  if (
    !process.env.CI ||
    context.arch !== require('electron-builder').Arch[process.arch]
  )
    return;
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
  await require('./packaged-diagnostics-smoke.cjs').packagedDiagnosticsSmoke(
    executable,
    artifacts,
  );
  if (process.env.NVM_TEST_PI_PACKAGED === '1') {
    const piArtifacts = path.join(
      context.packager.projectDir,
      'test-results',
      `packaged-pi-archive-node-${context.electronPlatformName}-${process.arch}`,
    );
    await execFileAsync(
      process.execPath,
      [
        path.join(__dirname, 'packaged-pi-archive-node-smoke.cjs'),
        path.join(resources, 'app.asar'),
        piArtifacts,
      ],
      { cwd: context.packager.projectDir, env: process.env },
    );
  }
};

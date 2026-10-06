const path = require('node:path');
const {
  verifyPackagedDependencies,
} = require('./verify-packaged-dependencies.cjs');

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
};

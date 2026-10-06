const path = require('node:path');
const {
  verifyPackagedDependencies,
} = require('./verify-packaged-dependencies.cjs');

module.exports = function verifyPackagedRuntime(context) {
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
};

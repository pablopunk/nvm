const fs = require('node:fs');
const path = require('node:path');

/**
 * TypeScript 7 runs its checker as a native binary, and binaries cannot spawn
 * from inside app.asar. Copies the target platform's `@typescript/typescript-*`
 * lib directory (binary plus lib*.d.ts) to `Resources/typescript-native`, where
 * src/app/electron/extension-host/typescript-api.ts looks for it.
 */
function stageTypeScriptNative({ resources, platform, arch }) {
  const packageName = `@typescript/typescript-${platform}-${arch}`;
  const typescriptRoot = path.dirname(
    require.resolve('typescript/package.json'),
  );
  const packageJson = require.resolve(`${packageName}/package.json`, {
    paths: [typescriptRoot],
  });
  const target = path.join(resources, 'typescript-native', 'lib');
  fs.rmSync(path.dirname(target), { recursive: true, force: true });
  fs.cpSync(path.join(path.dirname(packageJson), 'lib'), target, {
    recursive: true,
  });
  return packageName;
}

module.exports = { stageTypeScriptNative };

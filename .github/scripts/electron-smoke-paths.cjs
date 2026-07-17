'use strict';

const fs = require('node:fs');

const appPathPrefixes = ['src/', 'tests/electron/'];
const appPathExclusions = ['src/docs/'];
const appBuildPaths = new Set([
  'electron.vite.config.ts',
  'index.html',
  'mise.toml',
  'package.json',
  'playwright.config.ts',
  'pnpm-lock.yaml',
  'scripts/electron-test.cjs',
  'tsconfig.check.json',
  'tsconfig.json',
  'vite.config.ts',
]);

function isElectronSmokeRelevantPath(filePath) {
  const normalizedPath = filePath.replaceAll('\\', '/').replace(/^\.\//, '');
  if (appPathExclusions.some((prefix) => normalizedPath.startsWith(prefix)))
    return false;
  return (
    appBuildPaths.has(normalizedPath) ||
    appPathPrefixes.some((prefix) => normalizedPath.startsWith(prefix))
  );
}

function hasElectronSmokeChanges(filePaths) {
  return filePaths.some(isElectronSmokeRelevantPath);
}

if (require.main === module) {
  const filePaths = fs
    .readFileSync(0, 'utf8')
    .split('\n')
    .map((filePath) => filePath.trim())
    .filter(Boolean);
  process.stdout.write(hasElectronSmokeChanges(filePaths) ? 'true' : 'false');
}

module.exports = {
  appBuildPaths,
  appPathExclusions,
  appPathPrefixes,
  hasElectronSmokeChanges,
  isElectronSmokeRelevantPath,
};

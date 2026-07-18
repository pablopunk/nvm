'use strict';

const fs = require('node:fs');

const relevantPrefixes = [
  '.github/scripts/',
  'build/',
  'scripts/',
  'src/',
  'tests/electron/',
];
const relevantFiles = new Set([
  '.github/workflows/ci.yml',
  'electron-builder.yml',
  'electron.vite.config.ts',
  'index.html',
  'mise.toml',
  'package.json',
  'pnpm-lock.yaml',
  'tsconfig.check.json',
  'tsconfig.json',
]);

function isWindowsPackageRelevantPath(filePath) {
  const normalizedPath = filePath.replaceAll('\\', '/').replace(/^\.\//, '');
  return (
    relevantFiles.has(normalizedPath) ||
    relevantPrefixes.some((prefix) => normalizedPath.startsWith(prefix))
  );
}

function hasWindowsPackageChanges(filePaths) {
  return filePaths.some(isWindowsPackageRelevantPath);
}

if (require.main === module) {
  const filePaths = fs
    .readFileSync(0, 'utf8')
    .split('\n')
    .map((filePath) => filePath.trim())
    .filter(Boolean);
  process.stdout.write(hasWindowsPackageChanges(filePaths) ? 'true' : 'false');
}

module.exports = { hasWindowsPackageChanges, isWindowsPackageRelevantPath };

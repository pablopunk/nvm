'use strict';

// Paths whose change can alter the packaged desktop app. docs/ is included
// because electron-builder.yml packages docs/** into the app.
const appPathPrefixes = [
  'build/',
  'docs/',
  'scripts/',
  'src/app/',
  'tests/electron/',
];
const appFiles = new Set([
  '.gitattributes',
  '.npmrc',
  'LICENSE',
  'THIRD_PARTY_NOTICES.md',
  'electron-builder.yml',
  'electron.vite.config.ts',
  'mise.toml',
  'package.json',
  'pnpm-lock.yaml',
  'pnpm-workspace.yaml',
  'tsconfig.check.json',
  'tsconfig.json',
  'vite.config.ts',
  '.github/workflows/ci.yml',
  '.github/workflows/nightly-release.yml',
  '.github/scripts/verify-windows-package.ps1',
]);
const nonAppPathPrefixes = ['src/backend/'];
const PACKAGE_JSON_PATH = 'package.json';

function normalizeRepoPath(filePath) {
  return filePath.replaceAll('\\', '/').replace(/^\.\//, '');
}

function isNightlyAppAffectingPath(filePath) {
  const normalized = normalizeRepoPath(filePath);
  if (nonAppPathPrefixes.some((prefix) => normalized.startsWith(prefix)))
    return false;
  return (
    appFiles.has(normalized) ||
    appPathPrefixes.some((prefix) => normalized.startsWith(prefix))
  );
}

// A root package.json diff that changes only the top-level version is the
// mechanical stable version bump performed by scripts/release.sh. It does not
// alter the packaged app, so it must not trigger a nightly by itself.
function isVersionOnlyPackageJsonChange(baseText, headText) {
  const withoutVersion = (text) => {
    const manifest = JSON.parse(text);
    delete manifest.version;
    return JSON.stringify(manifest);
  };
  if (typeof baseText !== 'string' || typeof headText !== 'string')
    return false;
  return withoutVersion(baseText) === withoutVersion(headText);
}

// `isPackageJsonVersionOnly` receives the normalized path and returns whether
// the root package.json change is a version-only edit. It is consulted only for
// package.json; every other matched path is app-affecting on its own.
function classifyNightlyAppChanges(filePaths, { isPackageJsonVersionOnly }) {
  const matched = [];
  for (const rawPath of filePaths) {
    const normalized = normalizeRepoPath(rawPath.trim());
    if (!normalized) continue;
    if (!isNightlyAppAffectingPath(normalized)) continue;
    if (
      normalized === PACKAGE_JSON_PATH &&
      isPackageJsonVersionOnly(normalized)
    )
      continue;
    matched.push(normalized);
  }
  return { appAffecting: matched.length > 0, matched };
}

module.exports = {
  classifyNightlyAppChanges,
  isNightlyAppAffectingPath,
  isVersionOnlyPackageJsonChange,
  normalizeRepoPath,
};

'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const {
  classifyNightlyAppChanges,
  isNightlyAppAffectingPath,
  isVersionOnlyPackageJsonChange,
} = require('./nightly-app-paths.cjs');

test('desktop app, packaging, shared dependency and packaged docs paths are app-affecting', () => {
  for (const filePath of [
    'src/app/electron/main.ts',
    'src/app/resources/skills/example.md',
    'build/entitlements.mac.plist',
    'scripts/after-pack.cjs',
    'tests/electron/smoke.spec.ts',
    'electron-builder.yml',
    'electron.vite.config.ts',
    'pnpm-lock.yaml',
    'package.json',
    'mise.toml',
    'LICENSE',
    'docs/logging.md',
    '.github/workflows/ci.yml',
    '.github/workflows/nightly-release.yml',
    '.github/scripts/verify-windows-package.ps1',
  ]) {
    assert.equal(isNightlyAppAffectingPath(filePath), true, filePath);
  }
});

test('backend-only, workflow and unrelated paths never produce an app nightly', () => {
  for (const filePath of [
    'src/backend/src/lib/compatibility.ts',
    'src/backend/package.json',
    '.github/scripts/nightly-release-cli.cjs',
    'README.md',
    'biome.json',
  ]) {
    assert.equal(isNightlyAppAffectingPath(filePath), false, filePath);
  }
});

test('paths are normalized before classification', () => {
  assert.equal(isNightlyAppAffectingPath('./src/app/main.ts'), true);
  assert.equal(isNightlyAppAffectingPath('src\\app\\main.ts'), true);
});

test('a version-only root package.json change is the mechanical stable bump and is exempt', () => {
  const base = JSON.stringify(
    { name: 'nvm', version: '0.24.0', dependencies: { a: '1' } },
    null,
    2,
  );
  const bumped = JSON.stringify(
    { name: 'nvm', version: '0.24.1', dependencies: { a: '1' } },
    null,
    2,
  );
  const dependencyChanged = JSON.stringify(
    { name: 'nvm', version: '0.24.1', dependencies: { a: '2' } },
    null,
    2,
  );
  assert.equal(isVersionOnlyPackageJsonChange(base, bumped), true);
  assert.equal(isVersionOnlyPackageJsonChange(base, dependencyChanged), false);
  assert.equal(isVersionOnlyPackageJsonChange(null, bumped), false);
});

test('backend and unrelated workflow changes plus a version-only bump do not require an app build', () => {
  const calls = [];
  const result = classifyNightlyAppChanges(
    [
      'src/backend/src/lib/compatibility.ts',
      '.github/workflows/deployed-smoke.yml',
      'package.json',
    ],
    {
      isPackageJsonVersionOnly: (filePath) => {
        calls.push(filePath);
        return true;
      },
    },
  );
  assert.deepEqual(result, { appAffecting: false, matched: [] });
  assert.deepEqual(calls, ['package.json']);
});

test('a non-version package.json change is app-affecting and reported as matched', () => {
  const result = classifyNightlyAppChanges(
    ['src/backend/a.ts', 'package.json', ''],
    {
      isPackageJsonVersionOnly: () => false,
    },
  );
  assert.deepEqual(result, { appAffecting: true, matched: ['package.json'] });
});

test('a revert of an app change still counts as app-affecting', () => {
  const result = classifyNightlyAppChanges(['./src/app/palette/view.tsx'], {
    isPackageJsonVersionOnly: () => false,
  });
  assert.deepEqual(result, {
    appAffecting: true,
    matched: ['src/app/palette/view.tsx'],
  });
});

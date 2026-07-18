// biome-ignore-all lint/performance/useTopLevelRegex: Contract assertions are evaluated once per test.
'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');
const {
  normalizeRepositoryPath,
} = require('../../scripts/check-os-platform-boundaries.cjs');
const { isWindowsPackageRelevantPath } = require('./windows-package-paths.cjs');

function section(source, start, end) {
  const startIndex = source.indexOf(start);
  const endIndex = source.indexOf(end, startIndex + start.length);
  assert.notEqual(startIndex, -1, `missing ${start}`);
  assert.notEqual(endIndex, -1, `missing ${end}`);
  return source.slice(startIndex, endIndex);
}

test('Windows packaging filter covers every package and application input', () => {
  for (const candidate of [
    '.gitattributes',
    '.github/workflows/ci.yml',
    '.github/scripts/verify-windows-package.ps1',
    'build/Icon.icon/Assets/icon.png',
    'electron-builder.yml',
    'package.json',
    'pnpm-lock.yaml',
    'scripts/windows-portable-smoke.cjs',
    'src/electron/main.ts',
    'src/resources/nevermind-extension-api.d.ts',
    'src/ui.tsx',
    'tests/electron/palette.smoke.spec.ts',
  ]) {
    assert.equal(isWindowsPackageRelevantPath(candidate), true, candidate);
  }
  assert.equal(isWindowsPackageRelevantPath('backend/src/index.ts'), false);
  assert.equal(isWindowsPackageRelevantPath('README.md'), false);
});

test('builder config has deterministic unsigned-inspectable Windows targets', () => {
  const builder = fs.readFileSync('electron-builder.yml', 'utf8');
  const packageJson = JSON.parse(fs.readFileSync('package.json', 'utf8'));
  assert.match(
    builder,
    /^win:\n\s+icon: build\/Icon\.icon\/Assets\/icon\.png$/m,
  );
  assert.match(builder, /^\s+differentialPackage: true$/m);
  assert.match(builder, /Nevermind|\$\{productName\}/);
  assert.match(builder, /-setup\.\$\{ext\}/);
  assert.match(builder, /-portable\.\$\{ext\}/);
  assert.equal(packageJson.devDependencies['@electron/asar'], '3.4.1');
  assert.match(packageJson.scripts['dist:win:x64'], /--publish never/);
});

test('tracked text stays LF on Windows so aggregate verification is host-independent', () => {
  assert.equal(
    fs.readFileSync('.gitattributes', 'utf8'),
    '* text=auto eol=lf\n',
  );
});

test('platform boundary allowlists use host-independent repository paths', () => {
  assert.equal(
    normalizeRepositoryPath('src\\electron\\os.ts'),
    'src/electron/os.ts',
  );
});

test('package verifier enforces ASAR, signatures, metadata policy, icons, and fixed manifest schema', () => {
  const verifier = fs.readFileSync(
    '.github/scripts/verify-windows-package.ps1',
    'utf8',
  );
  for (const required of [
    'AsarExecutable',
    'UpdaterMetadataPolicy',
    'win-unpacked/resources/app.asar',
    'dist/main/main.js',
    'dist/preload/preload.cjs',
    'dist/renderer/index.html',
    'nevermind-extension-api.d.ts',
    'lib.es2022.full.d.ts',
    'Get-AuthenticodeSignature',
    "'NotSigned'",
    'latest.yml',
    'updaterMetadata = $updaterMetadata',
    'NativeResourceCounter',
    'schemaVersion = 1',
    'nsis = [ordered]@{',
    'portable = [ordered]@{',
    'unpacked = [ordered]@{',
    'configured-source-and-PE-resource-presence-only',
  ]) {
    assert.equal(verifier.includes(required), true, required);
  }
  assert.match(verifier, /finally\s*\{[\s\S]*Remove-Item/);
});

test('portable marker distinguishes wrapper identity from packaged child identity', () => {
  const marker = fs.readFileSync('src/electron/test-mode.ts', 'utf8');
  const harness = fs.readFileSync('scripts/windows-portable-smoke.cjs', 'utf8');
  for (const field of [
    'appIsPackaged',
    'appVersion',
    'processExecPath',
    'portableExecutableFile',
    'portableExecutableDir',
  ]) {
    assert.equal(marker.includes(field), true, field);
  }
  assert.match(harness, /manifest\.artifacts\.portable\.sha512/);
  assert.match(harness, /temporary storage/);
  assert.match(harness, /stabilityMilliseconds = 5_000/);
  assert.match(harness, /taskkill/);
});

test('Windows package smoke remains separate from first-run development smoke and cannot publish', () => {
  const workflow = fs.readFileSync('.github/workflows/ci.yml', 'utf8');
  const firstRun = section(
    workflow,
    '\n  windows-first-run:\n',
    '\n  windows-package-smoke:\n',
  );
  const packageSmoke = section(
    workflow,
    '\n  windows-package-smoke:\n',
    '\n  postgres-integration:\n',
  );
  assert.match(firstRun, /windows-first-run-smoke\.cjs/);
  for (const expected of [
    "github.event_name == 'workflow_dispatch'",
    'windows_package_changed',
    'runs-on: windows-latest',
    'mise exec -- pnpm verify',
    'CSC_IDENTITY_AUTO_DISCOVERY',
    'mise exec -- pnpm run dist:win:x64',
    "'-UpdaterMetadataPolicy', 'Nsis'",
    '-UpdaterMetadataPolicy Nsis',
    'Copy-Item release/latest.yml windows-artifacts/',
    'windows-package-negative',
    'verify-windows-private-file-acl.ps1',
    'windows-portable-smoke.cjs',
    'if-no-files-found: error',
  ]) {
    assert.equal(packageSmoke.includes(expected), true, expected);
  }
  assert.equal(/gh release|release upload|secrets\./.test(packageSmoke), false);
});

test('packaged checks accept an extracted package root', () => {
  assert.match(
    fs.readFileSync('scripts/check-packaged-resources.cjs', 'utf8'),
    /process\.argv\[2\]/,
  );
  assert.match(
    fs.readFileSync('scripts/check-packaged-runtime-imports.cjs', 'utf8'),
    /process\.argv\[2\]/,
  );
});

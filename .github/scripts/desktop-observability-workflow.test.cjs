'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const {
  listElectronSourceFiles,
} = require('../../scripts/lib/read-electron-sources.cjs');
const root = path.resolve(__dirname, '../..');

function electronSourcePath(basename) {
  const matches = listElectronSourceFiles(root).filter(
    (file) => path.basename(file) === basename,
  );
  assert.equal(matches.length, 1, `expected one ${basename} under electron`);
  return matches[0];
}

test('release packaging uploads the final generated maps before packaging without a rebuild', () => {
  const packageJson = JSON.parse(
    fs.readFileSync(path.join(root, 'package.json'), 'utf8'),
  );
  assert.match(
    packageJson.scripts.build,
    /^electron-vite build && node scripts\/upload-desktop-source-maps.cjs$/,
  );
  for (const name of ['dist:mac:arm64', 'dist:win:x64'])
    assert.match(packageJson.scripts[name], /^pnpm build &&/);
  const workflow = fs.readFileSync(
    path.join(root, '.github/workflows/ci.yml'),
    'utf8',
  );
  for (const job of ['release-mac', 'release-windows', 'release-linux']) {
    const definition = workflow
      .split(`  ${job}:`)[1]
      .split(/\n {2}[a-z-]+:/)[0];
    assert.match(definition, /NEVERMIND_RELEASE_BUILD: '1'/);
    assert.match(definition, /POSTHOG_CLI_PROJECT_ID:/);
    assert.match(definition, /VITE_POSTHOG_KEY:/);
  }
  assert.match(
    fs.readFileSync(path.join(root, 'electron-builder.yml'), 'utf8'),
    /!\*\*\/\*\.map/,
  );
});

test('maps are hidden for all processes and upload credentials are not build definitions', () => {
  const config = fs.readFileSync(
    path.join(root, 'electron.vite.config.ts'),
    'utf8',
  );
  assert.equal((config.match(/sourcemap: 'hidden'/g) || []).length, 3);
  assert.match(config, /__NEVERMIND_BUILD__/);
  assert.ok(!config.includes('POSTHOG_CLI_API_KEY'));
  const upload = fs.readFileSync(
    path.join(root, 'scripts/upload-desktop-source-maps.cjs'),
    'utf8',
  );
  assert.match(upload, /'sourcemap',\s*'process'/);
  assert.match(upload, /'--release-version'/);
});

test('desktop and backend capture all instrumented journeys and initialize diagnostics after logging', () => {
  assert.match(
    fs.readFileSync(electronSourcePath('diagnostic-reporting.ts'), 'utf8'),
    /captureDesktopEvent\('operation_completed'/,
  );
  assert.match(
    fs.readFileSync(path.join(root, 'src/backend/src/middleware.ts'), 'utf8'),
    /captureBackendRequest\(/,
  );
  const main = fs.readFileSync(
    path.join(root, 'src/app/electron/main.ts'),
    'utf8',
  );
  const loggerIndex = main.indexOf('configureLogger(isDev);');
  const diagnosticsIndex = main.indexOf('initDiagnostics({');
  assert.ok(loggerIndex >= 0 && diagnosticsIndex >= 0);
  assert.ok(loggerIndex < diagnosticsIndex);
});

test('packaging gates publication on the final dependency graph and native SDK capture', () => {
  const config = fs.readFileSync(
    path.join(root, 'electron-builder.yml'),
    'utf8',
  );
  assert.match(config, /afterPack: scripts\/after-pack.cjs/);
  const hook = fs.readFileSync(
    path.join(root, 'scripts/after-pack.cjs'),
    'utf8',
  );
  const graphCheck = hook.indexOf('const result = verifyPackagedDependencies');
  assert.ok(
    graphCheck >= 0 &&
      graphCheck <
        hook.indexOf("await require('./packaged-diagnostics-smoke.cjs')"),
  );
  assert.match(hook, /process\.env\.CI/);
  const workflow = fs.readFileSync(
    path.join(root, '.github/workflows/ci.yml'),
    'utf8',
  );
  assert.match(workflow, /Packaged diagnostic capture/);
  assert.match(workflow, /xvfb-run -a mise exec -- pnpm run dist:linux:/);
});

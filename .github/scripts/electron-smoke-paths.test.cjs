'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');
const {
  hasElectronSmokeChanges,
  isElectronSmokeRelevantPath,
} = require('./electron-smoke-paths.cjs');

test('runs for application source and smoke infrastructure', () => {
  for (const filePath of [
    'src/App.tsx',
    'src/electron/main.ts',
    'src/resources/nevermind-extension-api.d.ts',
    'tests/electron/palette.smoke.spec.ts',
    'scripts/electron-test.cjs',
    'package.json',
    'pnpm-lock.yaml',
    'electron.vite.config.ts',
    'playwright.config.ts',
  ]) {
    assert.equal(isElectronSmokeRelevantPath(filePath), true, filePath);
  }
});

test('skips backend, documentation, release-only, and unrelated changes', () => {
  for (const filePath of [
    'backend/src/pages/index.astro',
    'src/docs/logging.md',
    'README.md',
    'AGENTS.md',
    '.github/workflows/deployed-smoke.yml',
    'electron-builder.yml',
    'build/Icon.icon/icon.json',
    'scripts/release.sh',
  ]) {
    assert.equal(isElectronSmokeRelevantPath(filePath), false, filePath);
  }
});

test('runs mixed changes only when at least one app path changed', () => {
  assert.equal(
    hasElectronSmokeChanges(['backend/package.json', 'README.md']),
    false,
  );
  assert.equal(
    hasElectronSmokeChanges(['backend/package.json', 'src/ui.tsx']),
    true,
  );
});

test('CI wires the app gate and reporter wires serialized no-artifact skips', () => {
  const ciWorkflow = fs.readFileSync('.github/workflows/ci.yml', 'utf8');
  assert.match(
    ciWorkflow,
    /needs\.check\.outputs\.electron_smoke_changed == 'true'/,
  );
  const reportWorkflow = fs.readFileSync(
    '.github/workflows/report-linux-palette-screenshot.yml',
    'utf8',
  );
  assert.match(reportWorkflow, /^  pull-requests: write$/m);
  assert.doesNotMatch(reportWorkflow, /^  issues: write$/m);
  assert.match(
    reportWorkflow,
    /group: report-linux-palette-\$\{\{ github\.event\.workflow_run\.pull_requests\[0\]\.number \|\| github\.run_id \}\}/,
  );
  assert.match(reportWorkflow, /cancel-in-progress: false/);
  assert.equal(
    (
      reportWorkflow.match(
        /if: steps\.screenshot\.outputs\.found == 'true'/g,
      ) ?? []
    ).length,
    2,
  );
});

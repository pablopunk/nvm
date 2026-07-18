'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const { spawnSync } = require('node:child_process');
const test = require('node:test');

const inventory = JSON.parse(
  fs.readFileSync('src/docs/windows-platform-inventory.json', 'utf8'),
);

test('platform inventory covers every planned category and non-os platform seam', () => {
  assert.equal(inventory.supportStatus, 'unverified');
  const categories = new Set(
    inventory.sourceRules.map((rule) => rule.category),
  );
  assert.deepEqual(Array.from(categories).sort(), [
    'direct-platform',
    'injected-platform',
    'known-folder',
    'os-label',
    'platform-selector',
  ]);
  const files = new Set([
    ...inventory.sourceRules.map((rule) => rule.file),
    ...inventory.resolvedEntries.map((entry) => entry.file),
  ]);
  for (const expected of [
    'src/electron/byo-key.ts',
    'src/electron/nevermind-auth.ts',
    'src/electron/main.ts',
    'src/electron/nevermind-api.ts',
    'src/electron/system-settings.ts',
    'src/electron/app-uninstall-service.ts',
    'src/electron/running-app-status.ts',
    'src/electron/app-ipc-handlers.ts',
    'src/electron/extensions/system.ts',
    'src/extension-view.tsx',
  ]) {
    assert.equal(files.has(expected), true, expected);
  }
});

test('platform inventory checker accepts the frozen source and is wired into aggregate checks', () => {
  const result = spawnSync(
    process.execPath,
    ['scripts/check-windows-platform-inventory.cjs'],
    { encoding: 'utf8' },
  );
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
  assert.equal(
    fs
      .readFileSync('scripts/run-checks.cjs', 'utf8')
      .includes('check-windows-platform-inventory.cjs'),
    true,
  );
});

test('readiness document preserves the real Windows and support gates', () => {
  const readiness = fs.readFileSync(
    'src/docs/windows-release-readiness.md',
    'utf8',
  );
  for (const required of [
    'Windows support is **UNVERIFIED**',
    'Windows edition/build:',
    'GitHub Actions run URL:',
    'Dedicated non-production test account',
    'CI startup does **not** prove',
    'SmartScreen',
    'actual Windows update',
    'blocks a Windows support claim and close intent',
  ]) {
    assert.equal(readiness.includes(required), true, required);
  }
});

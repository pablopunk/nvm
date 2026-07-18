'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const { spawnSync } = require('node:child_process');
const test = require('node:test');
const {
  detectSitesInSource,
  validateInventoryShape,
  validateObservedSites,
} = require('../../scripts/check-windows-platform-inventory.cjs');
const {
  findUnownedBoundarySites,
} = require('../../scripts/check-os-platform-boundaries.cjs');

const inventory = JSON.parse(
  fs.readFileSync('src/docs/windows-platform-inventory.json', 'utf8'),
);

test('platform inventory covers every planned category and non-os platform seam', () => {
  assert.equal(inventory.schemaVersion, 2);
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
  const surfaceKeys = new Set(
    inventory.productSurfaces.map((surface) => surface.key),
  );
  for (const rule of inventory.sourceRules) {
    assert.equal(typeof rule.selector, 'string', rule.key);
    assert.equal(rule.selector.length > 0, true, rule.key);
    assert.equal(typeof rule.disposition, 'string', rule.key);
    assert.equal(rule.disposition.length > 0, true, rule.key);
    assert.equal(surfaceKeys.has(rule.surface), true, rule.key);
  }
});

test('every detector category rejects a second site in an already inventoried file', () => {
  const file = 'src/electron/os.ts';
  const source = fs.readFileSync(file, 'utf8');
  const additions = new Map([
    [
      'direct-platform',
      'export const inventoryRegressionDirect = process.platform;',
    ],
    [
      'injected-platform',
      "export const inventoryRegressionInjected = dependencies.processPlatform === 'win32';",
    ],
    [
      'platform-selector',
      'export const inventoryRegressionSelector = { win32: true };',
    ],
    ['os-label', "export const inventoryRegressionLabel = 'Finder';"],
    [
      'known-folder',
      "export const inventoryRegressionFolder = knownUserFolderPath('Desktop');",
    ],
  ]);

  for (const [category, addition] of additions) {
    const observed = detectSitesInSource(file, `${source}\n${addition}\n`);
    const errors = validateObservedSites(inventory, observed, {
      checkStale: false,
    });
    assert.equal(
      errors.some((error) => error.includes(`unowned ${category} site`)),
      true,
      `${category}: ${errors.join('\n')}`,
    );
  }
});

test('a new direct platform site needs its own selector, disposition, and readiness surface', () => {
  const file = 'src/electron/os.ts';
  const source = fs.readFileSync(file, 'utf8');
  const addition = 'export const inventoryRegressionDirect = process.platform;';
  const observed = detectSitesInSource(file, `${source}\n${addition}\n`);
  const originalSites = detectSitesInSource(file, source);
  const addedSite = observed.find(
    (site) =>
      site.category === 'direct-platform' &&
      !originalSites.some((original) => original.selector === site.selector),
  );
  assert.ok(addedSite);

  const boundaryErrors = findUnownedBoundarySites(inventory, observed);
  assert.equal(
    boundaryErrors.some((error) => error.includes(addedSite.selector)),
    true,
    boundaryErrors.join('\n'),
  );

  const incompleteRule = {
    key: 'inventory-regression-direct',
    file,
    category: addedSite.category,
    selector: addedSite.selector,
    surface: 'system-actions-settings',
  };
  const incompleteInventory = {
    ...inventory,
    sourceRules: [...inventory.sourceRules, incompleteRule],
  };
  assert.equal(
    validateInventoryShape(incompleteInventory).some((error) =>
      error.includes('is missing disposition'),
    ),
    true,
  );

  const completeInventory = {
    ...inventory,
    sourceRules: [
      ...inventory.sourceRules,
      { ...incompleteRule, disposition: 'capability-layer' },
    ],
  };
  assert.deepEqual(validateInventoryShape(completeInventory), []);
  assert.deepEqual(
    validateObservedSites(completeInventory, observed, { checkStale: false }),
    [],
  );
  assert.deepEqual(findUnownedBoundarySites(completeInventory, observed), []);
});

test('structurally ambiguous sites cannot share one inventory entry', () => {
  const file = 'src/electron/os.ts';
  const source = fs.readFileSync(file, 'utf8');
  const duplicate =
    'export const inventoryRegressionDuplicate = process.platform;';
  const observed = detectSitesInSource(
    file,
    `${source}\n${duplicate}\n${duplicate}\n`,
  );
  const duplicateSite = observed.find((site, index) =>
    observed.some(
      (candidate, candidateIndex) =>
        candidateIndex !== index &&
        candidate.category === site.category &&
        candidate.selector === site.selector,
    ),
  );
  assert.ok(duplicateSite);
  const errors = validateObservedSites(inventory, observed, {
    checkStale: false,
  });
  assert.equal(
    errors.some(
      (error) =>
        error.includes(duplicateSite.selector) &&
        error.includes('found 2 sites and 0 rules'),
    ),
    true,
    errors.join('\n'),
  );
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

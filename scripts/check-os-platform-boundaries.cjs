#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const {
  collectObservedSites,
  normalizeRepositoryPath,
  validateObservedSites,
} = require('./check-windows-platform-inventory.cjs');

function boundarySites(observedSites) {
  return observedSites.filter((site) =>
    ['direct-platform', 'os-label'].includes(site.category),
  );
}

function findUnownedBoundarySites(inventory, observedSites) {
  return validateObservedSites(inventory, boundarySites(observedSites), {
    checkStale: false,
  });
}

function main() {
  const rootDirectory = process.cwd();
  const inventory = JSON.parse(
    fs.readFileSync(
      path.join(
        rootDirectory,
        'src',
        'docs',
        'windows-platform-inventory.json',
      ),
      'utf8',
    ),
  );
  const observedSites = collectObservedSites(rootDirectory);
  const errors = findUnownedBoundarySites(inventory, observedSites);
  if (errors.length) {
    for (const error of errors)
      console.error(`OS platform boundary check failed: ${error}`);
    process.exit(1);
  }
  console.log(
    `OS platform boundary checks passed (${boundarySites(observedSites).length} site-specific boundaries)`,
  );
}

if (require.main === module) main();

module.exports = {
  boundarySites,
  findUnownedBoundarySites,
  normalizeRepositoryPath,
};

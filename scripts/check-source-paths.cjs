#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

const root = path.resolve(__dirname, '..');
const obsoleteDirectories = [
  ['src', 'electron'],
  ['src', 'docs'],
  ['src', 'resources'],
  ['src', 'shared'],
  ['src', 'type-fixtures'],
];
const paletteFiles = fs
  .readdirSync(path.join(root, 'src', 'app', 'palette'), {
    withFileTypes: true,
  })
  .filter((entry) => entry.isFile())
  .map((entry) => `src/${entry.name}`);
const obsoletePaths = obsoleteDirectories
  .map((segments) => `${segments.join('/')}/`)
  .concat(paletteFiles);
const trackedFiles = execFileSync('git', ['ls-files'], {
  cwd: root,
  encoding: 'utf8',
})
  .trim()
  .split('\n')
  .filter(Boolean);
const failures = [];

for (const file of trackedFiles) {
  if (file === 'scripts/check-source-paths.cjs') continue;
  const absolutePath = path.join(root, file);
  if (!fs.existsSync(absolutePath)) continue;
  const contents = fs.readFileSync(absolutePath);
  if (contents.includes(0)) continue;
  const source = contents.toString('utf8');
  for (const obsoletePath of obsoletePaths) {
    if (source.includes(obsoletePath))
      failures.push(`${file}: ${obsoletePath}`);
  }
  if (
    source.includes('src/fixtures/') &&
    !source.includes('src/backend/src/fixtures/')
  )
    failures.push(`${file}: src/fixtures/`);
}

if (failures.length > 0) {
  console.error(`Obsolete source paths found:\n${failures.join('\n')}`);
  process.exit(1);
}

console.log('Source path checks passed');

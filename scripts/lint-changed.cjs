'use strict';

const { spawnSync } = require('node:child_process');

const base = process.argv[2];
if (!base) {
  console.error('Usage: node scripts/lint-changed.cjs <base-ref>');
  process.exit(2);
}

const result = spawnSync(
  'npx',
  ['biome', 'check', '--changed', '--error-on-warnings', '--since', base],
  { stdio: 'pipe', timeout: 60_000 },
);

const stderr = result.stderr.toString('utf-8');
process.stderr.write(stderr);

if (result.status === 0) {
  process.exit(0);
}

if (stderr.includes('No files were processed')) {
  console.log('No biome-lintable files changed — skipping.');
  process.exit(0);
}

process.exit(result.status);

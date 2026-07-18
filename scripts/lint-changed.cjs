'use strict';

const { spawnSync } = require('node:child_process');

const base = process.argv[2];
if (!base) {
  process.stderr.write('Usage: node scripts/lint-changed.cjs <base-ref>\n');
  process.exit(2);
}

const result = spawnSync(
  process.execPath,
  [
    require.resolve('@biomejs/biome/bin/biome'),
    'check',
    '--changed',
    '--error-on-warnings',
    '--since',
    base,
  ],
  { stdio: 'pipe', timeout: 60_000 },
);

if (result.error) {
  process.stderr.write(`${result.error.message}\n`);
  process.exit(1);
}

const stderr = result.stderr.toString('utf-8');
process.stderr.write(stderr);

if (result.status === 0) {
  process.exit(0);
}

if (stderr.includes('No files were processed')) {
  process.stdout.write('No biome-lintable files changed — skipping.\n');
  process.exit(0);
}

process.exit(result.status);

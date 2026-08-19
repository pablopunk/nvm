'use strict';

const { execFileSync, spawnSync } = require('node:child_process');

const supportedExtension = /\.(cjs|cts|js|json|jsonc|jsx|mjs|mts|ts|tsx)$/i;
const write = process.argv.includes('--write');
const stagedFiles = execFileSync(
  'git',
  ['diff', '--cached', '--name-only', '--diff-filter=ACMR', '-z'],
  { encoding: 'utf8' },
)
  .split('\0')
  .filter(Boolean)
  .filter(
    (file) =>
      supportedExtension.test(file) &&
      !file.startsWith('src/backend/') &&
      !file.startsWith('.github/'),
  );

if (stagedFiles.length === 0) {
  process.stdout.write('No staged frontend files need checking.\n');
  process.exit(0);
}

const result = spawnSync(
  process.execPath,
  [
    require.resolve('@biomejs/biome/bin/biome'),
    'check',
    ...(write ? ['--write'] : []),
    '--error-on-warnings',
    '--assist-enabled=false',
    '--skip=suspicious',
    '--skip=style',
    '--skip=a11y',
    '--skip=performance',
    '--skip=complexity',
    '--',
    ...stagedFiles,
  ],
  { stdio: 'inherit' },
);

if (result.error) {
  process.stderr.write(`${result.error.message}\n`);
  process.exit(1);
}
if (write && result.status === 0)
  process.stdout.write('Re-stage the formatted files before committing.\n');
process.exit(result.status ?? 1);

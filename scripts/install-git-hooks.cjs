'use strict';

const { execFileSync, spawnSync } = require('node:child_process');

try {
  execFileSync('git', ['rev-parse', '--git-dir'], { stdio: 'ignore' });
} catch {
  process.stdout.write('Git hooks not installed: this is not a Git checkout.\n');
  process.exit(0);
}

const configuredPathResult = spawnSync(
  'git',
  ['config', '--local', '--get', 'core.hooksPath'],
  { encoding: 'utf8' },
);
if (configuredPathResult.error) {
  process.stderr.write(`${configuredPathResult.error.message}\n`);
  process.exit(1);
}
const configuredPath = configuredPathResult.stdout.trim();

if (configuredPath && configuredPath !== '.githooks') {
  process.stdout.write(
    `Git hooks not installed: core.hooksPath is already ${configuredPath}.\n`,
  );
  process.exit(0);
}

execFileSync('git', ['config', '--local', 'core.hooksPath', '.githooks']);
process.stdout.write('Git hooks installed from .githooks.\n');

'use strict';

const { execFileSync } = require('node:child_process');

function versionFromHistory(timestamps) {
  if (
    !Array.isArray(timestamps) ||
    timestamps.length < 1 ||
    timestamps.length > 65535
  )
    throw new Error(
      'First-parent history exceeds supported Windows version range',
    );
  if (timestamps.some((value) => !Number.isSafeInteger(value) || value < 0))
    throw new Error('Invalid commit timestamp');
  const date = new Date(Math.max(...timestamps) * 1000);
  if (
    !Number.isFinite(date.getTime()) ||
    date.getUTCFullYear() > 9999 ||
    date.getUTCFullYear() < 2000
  )
    throw new Error('Invalid release date');
  const monthDay = (date.getUTCMonth() + 1) * 100 + date.getUTCDate();
  return `${date.getUTCFullYear()}.${monthDay}.${timestamps.length}`;
}

if (require.main === module) {
  try {
    const sha = process.argv[2];
    if (!/^[0-9a-f]{40}$/.test(sha ?? ''))
      throw new Error('Expected full commit SHA');
    const history = execFileSync(
      'git',
      ['log', '--first-parent', '--format=%ct', sha],
      { encoding: 'utf8' },
    );
    console.log(versionFromHistory(history.trim().split('\n').map(Number)));
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}

module.exports = { versionFromHistory };

#!/usr/bin/env node
'use strict';

const path = require('node:path');
const { spawnSync } = require('node:child_process');

// Desktop packaging never imports src/backend, so CI packaging jobs opt out of
// the nested install with NEVERMIND_SKIP_BACKEND_INSTALL=1. Every other install
// keeps the backend ready for root tests, `pnpm verify`, and `pnpm dev`.
function installBackendDependencies() {
  if (process.env.NEVERMIND_SKIP_BACKEND_INSTALL === '1') {
    console.log(
      'Skipping src/backend install: NEVERMIND_SKIP_BACKEND_INSTALL=1.',
    );
    return 0;
  }
  // Same invocation the inline postinstall used: the pnpm running this
  // lifecycle script is on PATH, including standalone Windows binaries.
  const result = spawnSync(
    'pnpm',
    ['-C', path.join('src', 'backend'), 'install'],
    {
      cwd: path.resolve(__dirname, '..'),
      stdio: 'inherit',
      shell: true,
    },
  );
  if (result.error) throw result.error;
  return result.status ?? 1;
}

if (require.main === module) process.exitCode = installBackendDependencies();

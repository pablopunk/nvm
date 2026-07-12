#!/usr/bin/env node
'use strict';

const { spawnSync } = require('node:child_process');
const path = require('node:path');

const args = process.argv.slice(2);
let query = '';
let execute = '';
let skipBuild = false;

// biome-ignore lint/style/useForOf: lookahead requires index-based iteration
for (let i = 0; i < args.length; i += 1) {
  const arg = args[i];
  // biome-ignore lint/style/useBlockStatements: minimal CLI arg parser
  if (arg === '--query' || arg === '-q') { query = args[++i] || ''; }
  // biome-ignore lint/style/useBlockStatements: minimal CLI arg parser
  else if (arg === '--execute' || arg === '-x') { execute = args[++i] || ''; }
  // biome-ignore lint/style/useBlockStatements: minimal CLI arg parser
  else if (arg === '--no-build') { skipBuild = true; }
  // biome-ignore lint/style/useBlockStatements: minimal CLI arg parser
  else if (!query) { query = arg; }
}

function run(command, commandArgs, options = {}) {
  const result = spawnSync(command, commandArgs, {
    stdio: 'inherit',
    ...options,
  });
  // biome-ignore lint/style/useBlockStatements: minimal CLI script
  if (result.status !== 0) { process.exit(result.status || 1); }
}

// biome-ignore lint/style/useBlockStatements: minimal CLI script
if (!skipBuild) { run('pnpm', ['build']); }

const electronBin = path.join(
  process.cwd(),
  'node_modules',
  '.bin',
  process.platform === 'win32' ? 'electron.cmd' : 'electron',
);
// biome-ignore lint/style/noProcessEnv: CLI script, env is the intended config layer
// biome-ignore lint/style/useNamingConvention: env vars use SCREAMING_SNAKE_CASE by convention
run(electronBin, ['.'], {
  env: {
    ...process.env,
    NVM_PALETTE_DEBUG: '1',
    NVM_PALETTE_QUERY: query,
    ...(execute ? { NVM_PALETTE_EXECUTE: execute } : {}),
  },
});

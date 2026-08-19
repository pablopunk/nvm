#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const root = path.resolve(__dirname, '..');
const backendDirectory = path.join(root, 'src', 'backend');
const legacyBackendDirectory = path.join(root, 'backend');

function copyLegacyFile(relativePath) {
  const source = path.join(legacyBackendDirectory, relativePath);
  const destination = path.join(backendDirectory, relativePath);
  if (!fs.existsSync(source) || fs.existsSync(destination)) return;
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  fs.copyFileSync(source, destination);
  console.log(
    `Migrated backend/${relativePath} to src/backend/${relativePath}`,
  );
}

function installBackendDependencies() {
  const astroExecutable = path.join(
    backendDirectory,
    'node_modules',
    '.bin',
    process.platform === 'win32' ? 'astro.cmd' : 'astro',
  );
  if (fs.existsSync(astroExecutable)) return;

  const pnpmExecutable = process.env.npm_execpath;
  if (!pnpmExecutable) {
    throw new Error(
      'Cannot install backend dependencies because pnpm is unavailable.',
    );
  }
  console.log('Backend dependencies are missing; installing them...');
  const result = spawnSync(
    process.execPath,
    [pnpmExecutable, '-C', backendDirectory, 'install', '--frozen-lockfile'],
    { cwd: root, stdio: 'inherit' },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(
      `Backend dependency installation failed with exit code ${result.status}.`,
    );
  }
}

function ensureBackendAvailable() {
  copyLegacyFile('.env');
  copyLegacyFile(path.join('.vercel', 'project.json'));
  installBackendDependencies();
}

if (require.main === module) {
  try {
    ensureBackendAvailable();
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}

module.exports = { ensureBackendAvailable };

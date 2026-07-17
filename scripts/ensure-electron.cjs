#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

function resolveElectronPackageDirectory() {
  return path.dirname(require.resolve('electron/package.json'));
}

function resolveInstalledElectronExecutable() {
  const electronEntryPoint = require.resolve('electron');
  delete require.cache[electronEntryPoint];

  try {
    const executablePath = require(electronEntryPoint);
    if (
      typeof executablePath === 'string' &&
      fs.statSync(executablePath).isFile()
    ) {
      return executablePath;
    }
  } catch {
    // An absent path.txt or executable is repaired below.
  }

  return undefined;
}

function cleanElectronGeneratedPayload(electronPackageDirectory) {
  fs.rmSync(path.join(electronPackageDirectory, 'dist'), {
    force: true,
    recursive: true,
  });
  fs.rmSync(path.join(electronPackageDirectory, 'path.txt'), { force: true });
}

function electronInstallerEnvironment(environment = process.env) {
  const installerEnvironment = {};
  for (const [name, value] of Object.entries(environment)) {
    const normalizedName = name.toUpperCase();
    if (
      normalizedName !== 'ELECTRON_SKIP_BINARY_DOWNLOAD' &&
      normalizedName !== 'FORCE_NO_CACHE'
    ) {
      installerEnvironment[name] = value;
    }
  }
  installerEnvironment.force_no_cache = 'true';
  return installerEnvironment;
}

function installElectronBinary({
  electronPackageDirectory = resolveElectronPackageDirectory(),
  environment = process.env,
  nodeExecutable = process.execPath,
  spawn = spawnSync,
} = {}) {
  const installScript = path.join(electronPackageDirectory, 'install.js');
  const result = spawn(nodeExecutable, [installScript], {
    cwd: electronPackageDirectory,
    env: electronInstallerEnvironment(environment),
    stdio: 'inherit',
  });

  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(
      `Electron installer failed with exit code ${result.status}.`,
    );
  }
}

function ensureElectronAvailable({
  electronPackageDirectory = resolveElectronPackageDirectory(),
  resolveExecutable = resolveInstalledElectronExecutable,
  install = installElectronBinary,
} = {}) {
  const existingExecutable = resolveExecutable();
  if (existingExecutable) return existingExecutable;

  console.log('Electron binary is missing or invalid; repairing it...');
  cleanElectronGeneratedPayload(electronPackageDirectory);
  install({ electronPackageDirectory });

  const repairedExecutable = resolveExecutable();
  if (!repairedExecutable) {
    throw new Error(
      'Electron installer completed without a usable executable.',
    );
  }
  return repairedExecutable;
}

if (require.main === module) {
  try {
    const executablePath = ensureElectronAvailable();
    console.log(`Electron executable ready: ${executablePath}`);
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}

module.exports = {
  cleanElectronGeneratedPayload,
  electronInstallerEnvironment,
  ensureElectronAvailable,
  installElectronBinary,
  resolveInstalledElectronExecutable,
};

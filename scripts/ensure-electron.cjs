#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

function resolveElectronPackageDirectory() {
  return path.dirname(require.resolve('electron/package.json'));
}

function resolveInstalledElectronExecutable({
  electronPackageDirectory = resolveElectronPackageDirectory(),
  environment = process.env,
} = {}) {
  let executableRelativePath;
  try {
    executableRelativePath = fs
      .readFileSync(path.join(electronPackageDirectory, 'path.txt'), 'utf8')
      .trim();
  } catch {
    return undefined;
  }

  if (!executableRelativePath) return undefined;
  const executablePath = environment.ELECTRON_OVERRIDE_DIST_PATH
    ? path.join(environment.ELECTRON_OVERRIDE_DIST_PATH, executableRelativePath)
    : path.join(electronPackageDirectory, 'dist', executableRelativePath);
  try {
    return fs.statSync(executablePath).isFile() ? executablePath : undefined;
  } catch {
    return undefined;
  }
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

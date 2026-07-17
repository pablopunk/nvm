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

function pnpmRebuildInvocation({
  npmExecPath = process.env.npm_execpath,
  nodeExecutable = process.execPath,
  platform = process.platform,
} = {}) {
  const rebuildArguments = ['rebuild', 'electron'];
  if (!npmExecPath) {
    return platform === 'win32'
      ? {
          command: 'pnpm.cmd',
          commandArguments: rebuildArguments,
          shell: true,
        }
      : {
          command: 'pnpm',
          commandArguments: rebuildArguments,
          shell: false,
        };
  }

  const extension = path.extname(npmExecPath).toLowerCase();
  if (['.js', '.cjs', '.mjs'].includes(extension)) {
    return {
      command: nodeExecutable,
      commandArguments: [npmExecPath, ...rebuildArguments],
      shell: false,
    };
  }
  if (extension === '.cmd' || extension === '.bat') {
    return {
      command: npmExecPath,
      commandArguments: rebuildArguments,
      shell: platform === 'win32',
    };
  }
  return {
    command: npmExecPath,
    commandArguments: rebuildArguments,
    shell: false,
  };
}

function rebuildElectron({ spawn = spawnSync, ...invocationOptions } = {}) {
  const { command, commandArguments, shell } =
    pnpmRebuildInvocation(invocationOptions);
  const result = spawn(command, commandArguments, {
    cwd: path.join(__dirname, '..'),
    shell,
    stdio: 'inherit',
  });

  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`Electron rebuild failed with exit code ${result.status}.`);
  }
}

function ensureElectronAvailable({
  electronPackageDirectory = resolveElectronPackageDirectory(),
  resolveExecutable = resolveInstalledElectronExecutable,
  rebuild = rebuildElectron,
} = {}) {
  const existingExecutable = resolveExecutable();
  if (existingExecutable) return existingExecutable;

  console.log('Electron binary is missing or invalid; repairing it...');
  cleanElectronGeneratedPayload(electronPackageDirectory);
  rebuild();

  const repairedExecutable = resolveExecutable();
  if (!repairedExecutable) {
    throw new Error('Electron rebuild completed without a usable executable.');
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
  ensureElectronAvailable,
  pnpmRebuildInvocation,
  rebuildElectron,
  resolveInstalledElectronExecutable,
};

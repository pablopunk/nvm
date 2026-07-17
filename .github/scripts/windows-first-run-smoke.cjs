#!/usr/bin/env node
'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');
const { env } = require('node:process');
const { stripVTControlCharacters } = require('node:util');

const root = path.resolve(__dirname, '..', '..');
const pnpmExecutable = env.PNPM_STANDALONE_PATH;
const maximumCapturedOutputLength = 250_000;
const startupStabilityMilliseconds = 5000;
const startupTimeoutMilliseconds = 120_000;
const startupMarkers = [
  'Electron executable ready:',
  'electron main process built successfully',
  'electron preload scripts built successfully',
  'dev server running for the electron renderer process at:',
  'starting electron app...',
];

function resolveElectronExecutable() {
  const electronEntryPoint = require.resolve('electron', { paths: [root] });
  delete require.cache[electronEntryPoint];
  return require(electronEntryPoint);
}

function stopProcessTree(pid) {
  if (!pid) {
    return;
  }
  spawnSync('taskkill', ['/pid', String(pid), '/t', '/f'], {
    stdio: 'inherit',
  });
}

function finishDevelopmentProcess({ child, error, reject, resolve, state }) {
  if (state.settled) {
    return;
  }
  state.settled = true;
  clearTimeout(state.timeout);
  clearTimeout(state.startupTimer);
  stopProcessTree(child.pid);
  if (error) {
    reject(error);
  } else {
    resolve();
  }
}

function verifyStableElectronStartup(finish) {
  try {
    const electronExecutable = resolveElectronExecutable();
    assert.equal(
      path.basename(electronExecutable).toLowerCase(),
      'electron.exe',
    );
    assert.equal(fs.statSync(electronExecutable).isFile(), true);
    process.stdout.write(
      `Windows first-run Electron startup passed: ${electronExecutable}\n`,
    );
    finish();
  } catch (error) {
    finish(error);
  }
}

function captureDevelopmentOutput({ chunk, finish, state, target }) {
  target.write(chunk);
  state.output += stripVTControlCharacters(chunk.toString());
  if (state.output.length > maximumCapturedOutputLength) {
    state.output = state.output.slice(-maximumCapturedOutputLength);
  }
  if (
    !state.startupTimer &&
    startupMarkers.every((marker) => state.output.includes(marker))
  ) {
    state.startupTimer = setTimeout(
      () => verifyStableElectronStartup(finish),
      startupStabilityMilliseconds,
    );
  }
}

function timeoutDevelopmentStartup(state, finish) {
  const missingMarkers = startupMarkers.filter(
    (marker) => !state.output.includes(marker),
  );
  finish(
    new Error(
      `pnpm run dev did not reach startup within 120 seconds; missing: ${missingMarkers.join(', ')}`,
    ),
  );
}

function monitorDevelopmentStartup(resolve, reject) {
  const child = spawn(pnpmExecutable, ['run', 'dev'], {
    cwd: root,
    env,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const state = { output: '', settled: false };
  const finish = (error) =>
    finishDevelopmentProcess({ child, error, reject, resolve, state });
  const inspectOutput = (chunk, target) =>
    captureDevelopmentOutput({ chunk, finish, state, target });

  child.stdout.on('data', (chunk) => inspectOutput(chunk, process.stdout));
  child.stderr.on('data', (chunk) => inspectOutput(chunk, process.stderr));
  child.on('error', finish);
  child.on('exit', (code, signal) => {
    if (!state.settled) {
      finish(
        new Error(
          `pnpm run dev exited before a stable startup (code=${code}, signal=${signal}).`,
        ),
      );
    }
  });
  state.timeout = setTimeout(
    () => timeoutDevelopmentStartup(state, finish),
    startupTimeoutMilliseconds,
  );
}

async function runFirstDevelopmentStartup() {
  assert.equal(
    process.platform,
    'win32',
    'The first-run smoke test must execute on Windows.',
  );
  assert.ok(
    pnpmExecutable && fs.statSync(pnpmExecutable).isFile(),
    'PNPM_STANDALONE_PATH must identify the packaged pnpm.exe.',
  );
  assert.throws(
    resolveElectronExecutable,
    'The install fixture must begin without an Electron executable.',
  );
  await new Promise(monitorDevelopmentStartup);
}

runFirstDevelopmentStartup().catch((error) => {
  process.stderr.write(`${error instanceof Error ? error.stack : error}\n`);
  process.exitCode = 1;
});

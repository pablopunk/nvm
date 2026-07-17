#!/usr/bin/env node
'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const root = path.resolve(__dirname, '..', '..');
const pnpmExecutable = process.env.PNPM_STANDALONE_PATH;
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
  if (!pid) return;
  spawnSync('taskkill', ['/pid', String(pid), '/t', '/f'], {
    stdio: 'inherit',
  });
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

  await new Promise((resolve, reject) => {
    const child = spawn(pnpmExecutable, ['run', 'dev'], {
      cwd: root,
      env: process.env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let output = '';
    let settled = false;
    let startupTimer;
    let timeout;

    const finish = (error) => {
      if (settled) return;
      settled = true;
      clearTimeout(timeout);
      clearTimeout(startupTimer);
      stopProcessTree(child.pid);
      if (error) reject(error);
      else resolve();
    };

    const inspectOutput = (chunk, target) => {
      target.write(chunk);
      output += chunk.toString().replace(/\u001B\[[0-?]*[ -/]*[@-~]/g, '');
      if (output.length > 250_000) output = output.slice(-250_000);

      if (
        !startupTimer &&
        startupMarkers.every((marker) => output.includes(marker))
      ) {
        startupTimer = setTimeout(() => {
          try {
            const electronExecutable = resolveElectronExecutable();
            assert.equal(
              path.basename(electronExecutable).toLowerCase(),
              'electron.exe',
            );
            assert.equal(fs.statSync(electronExecutable).isFile(), true);
            console.log(
              `Windows first-run Electron startup passed: ${electronExecutable}`,
            );
            finish();
          } catch (error) {
            finish(error);
          }
        }, 5_000);
      }
    };

    child.stdout.on('data', (chunk) => inspectOutput(chunk, process.stdout));
    child.stderr.on('data', (chunk) => inspectOutput(chunk, process.stderr));
    child.on('error', finish);
    child.on('exit', (code, signal) => {
      if (!settled) {
        finish(
          new Error(
            `pnpm run dev exited before a stable startup (code=${code}, signal=${signal}).`,
          ),
        );
      }
    });

    timeout = setTimeout(() => {
      const missingMarkers = startupMarkers.filter(
        (marker) => !output.includes(marker),
      );
      finish(
        new Error(
          `pnpm run dev did not reach startup within 120 seconds; missing: ${missingMarkers.join(', ')}`,
        ),
      );
    }, 120_000);
  });
}

runFirstDevelopmentStartup().catch((error) => {
  console.error(error instanceof Error ? error.stack : error);
  process.exitCode = 1;
});

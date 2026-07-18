#!/usr/bin/env node
'use strict';

const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const startupTimeoutMilliseconds = 120_000;
const stabilityMilliseconds = 5_000;

function sha512Base64(filePath) {
  return crypto
    .createHash('sha512')
    .update(fs.readFileSync(filePath))
    .digest('base64');
}

function stopProcessTree(pid) {
  if (!pid) return;
  spawnSync('taskkill', ['/PID', String(pid), '/T', '/F'], {
    stdio: 'ignore',
  });
}

function processIsRunning(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function waitForMarker(markerPath, child) {
  return new Promise((resolve, reject) => {
    const deadline = Date.now() + startupTimeoutMilliseconds;
    const interval = setInterval(() => {
      if (fs.existsSync(markerPath)) {
        clearInterval(interval);
        try {
          resolve(JSON.parse(fs.readFileSync(markerPath, 'utf8')));
        } catch (error) {
          reject(error);
        }
      } else if (Date.now() > deadline) {
        clearInterval(interval);
        reject(
          new Error(
            'Portable app did not write its renderer-ready marker within 120 seconds.',
          ),
        );
      } else if (child.exitCode !== null) {
        clearInterval(interval);
        reject(
          new Error(
            `Portable wrapper exited before readiness with code ${child.exitCode}.`,
          ),
        );
      }
    }, 250);
  });
}

async function requireStableChild(pid) {
  const deadline = Date.now() + stabilityMilliseconds;
  while (Date.now() < deadline) {
    assert.equal(
      processIsRunning(pid),
      true,
      'Packaged child exited during the five-second stability window.',
    );
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
}

async function run() {
  assert.equal(
    process.platform,
    'win32',
    'Portable startup smoke must run on Windows.',
  );
  const [launcherInput, manifestInput, artifactDirectoryInput] =
    process.argv.slice(2);
  assert.ok(
    launcherInput && manifestInput && artifactDirectoryInput,
    'usage: windows-portable-smoke.cjs <portable.exe> <manifest.json> <artifact-dir>',
  );
  const launcherPath = path.win32.resolve(launcherInput);
  const manifestPath = path.win32.resolve(manifestInput);
  const artifactDirectory = path.win32.resolve(artifactDirectoryInput);
  assert.equal(
    path.win32.dirname(launcherPath),
    artifactDirectory,
    'Portable launcher must already be staged in the artifact directory.',
  );
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  const evidenceDirectory = path.win32.join(
    artifactDirectory,
    'startup-evidence',
  );
  const markerPath = path.win32.join(evidenceDirectory, 'portable-ready.json');
  const stdoutPath = path.win32.join(evidenceDirectory, 'portable-stdout.log');
  const stderrPath = path.win32.join(evidenceDirectory, 'portable-stderr.log');
  const userDataDirectory = fs.mkdtempSync(
    path.join(os.tmpdir(), 'nvm-portable-smoke-'),
  );
  fs.mkdirSync(evidenceDirectory, { recursive: true });
  fs.writeFileSync(stdoutPath, '');
  fs.writeFileSync(stderrPath, '');

  const child = spawn(launcherPath, ['--nvm-portable-smoke'], {
    cwd: artifactDirectory,
    env: {
      ...process.env,
      NVM_PACKAGED_STARTUP_SMOKE: '1',
      NVM_PORTABLE_SMOKE_MARKER: markerPath,
      NVM_TEST_ARTIFACT_DIR: evidenceDirectory,
      NVM_TEST_MODE: '1',
      NVM_TEST_USER_DATA_DIR: userDataDirectory,
    },
    stdio: [
      'ignore',
      fs.openSync(stdoutPath, 'a'),
      fs.openSync(stderrPath, 'a'),
    ],
  });
  let marker;
  try {
    marker = await waitForMarker(markerPath, child);
    const recordedWrapper = path.win32.resolve(marker.portableExecutableFile);
    const recordedWrapperDirectory = path.win32.resolve(
      marker.portableExecutableDir,
    );
    assert.equal(recordedWrapper.toLowerCase(), launcherPath.toLowerCase());
    assert.equal(
      recordedWrapperDirectory.toLowerCase(),
      artifactDirectory.toLowerCase(),
    );
    assert.equal(
      sha512Base64(recordedWrapper),
      manifest.artifacts.portable.sha512,
    );
    assert.equal(marker.appIsPackaged, true);
    assert.equal(marker.appVersion, manifest.build.version);
    const packagedChild = path.win32.resolve(marker.processExecPath);
    assert.equal(
      path.win32.basename(packagedChild).toLowerCase(),
      'nevermind.exe',
    );
    assert.notEqual(packagedChild.toLowerCase(), launcherPath.toLowerCase());
    const temporaryRoot = `${path.win32.resolve(os.tmpdir()).toLowerCase()}${path.win32.sep}`;
    assert.equal(
      packagedChild.toLowerCase().startsWith(temporaryRoot),
      true,
      'Portable child must run beneath Windows temporary storage.',
    );
    assert.equal(fs.statSync(packagedChild).isFile(), true);
    await requireStableChild(marker.pid);
    process.stdout.write(`Windows portable startup passed: ${markerPath}\n`);
  } finally {
    stopProcessTree(marker?.pid);
    stopProcessTree(child.pid);
    fs.rmSync(userDataDirectory, { force: true, recursive: true });
  }
}

run().catch((error) => {
  process.stderr.write(`${error instanceof Error ? error.stack : error}\n`);
  process.exitCode = 1;
});

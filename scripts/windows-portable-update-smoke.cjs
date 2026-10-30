#!/usr/bin/env node
'use strict';

// Packaged end-to-end check of the portable updater: a real old build starts the
// real helper, quits, and the helper swaps, relaunches, and rolls back as needed.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

const scenarioTimeoutMilliseconds = 420_000;
const lockHoldMilliseconds = 10_000;
const userFileName = 'My Nevermind.exe';

function sha512Base64(filePath) {
  return crypto
    .createHash('sha512')
    .update(fs.readFileSync(filePath))
    .digest('base64');
}

function stopProcessTree(pid) {
  if (pid)
    spawnSync('taskkill', ['/PID', String(pid), '/T', '/F'], {
      stdio: 'ignore',
    });
}

function readJson(filePath) {
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8').replace(/^﻿/, ''));
  } catch {
    return null;
  }
}

async function waitFor(description, probe) {
  const deadline = Date.now() + scenarioTimeoutMilliseconds;
  while (Date.now() < deadline) {
    const value = probe();
    if (value) return value;
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`Timed out waiting for ${description}.`);
}

function quote(value) {
  return `'${value.replace(/'/g, "''")}'`;
}

async function holdTargetOpen(targetPath, evidence) {
  const opened = path.join(evidence, 'lock-opened.txt');
  const release = path.join(evidence, 'lock-release.txt');
  const script = [
    `$stream = [IO.File]::Open(${quote(targetPath)}, 'Open', 'Read', 'Read')`,
    `[IO.File]::WriteAllText(${quote(opened)}, 'opened')`,
    `while (-not (Test-Path -LiteralPath ${quote(release)})) { Start-Sleep -Milliseconds 200 }`,
    '$stream.Dispose()',
  ].join('; ');
  const holder = spawn(
    'powershell.exe',
    ['-NoProfile', '-NonInteractive', '-Command', script],
    { stdio: 'ignore' },
  );
  await waitFor('the competing handle', () => fs.existsSync(opened));
  return {
    holder,
    async releaseAfterAppExit(pid) {
      await waitFor('the old build to exit', () => !processIsRunning(pid));
      await new Promise((resolve) => setTimeout(resolve, lockHoldMilliseconds));
      const releasedAt = new Date().toISOString();
      fs.writeFileSync(release, releasedAt);
      return releasedAt;
    },
  };
}

function processIsRunning(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

async function runScenario(scenario, input) {
  const root = fs.mkdtempSync(
    path.join(os.tmpdir(), `nvm-portable-update-${scenario.name}-`),
  );
  const evidence = path.join(input.artifactDirectory, scenario.name);
  const userData = path.join(root, 'profile');
  const installDirectory = path.join(root, 'Portable Apps');
  const target = path.join(installDirectory, userFileName);
  const marker = path.join(evidence, 'ready.json');
  const resultPath = path.join(userData, 'portable-update', 'result.json');
  fs.mkdirSync(evidence, { recursive: true });
  fs.mkdirSync(userData, { recursive: true });
  fs.mkdirSync(installDirectory, { recursive: true });
  fs.copyFileSync(input.oldPortable, target);
  const oldSha512 = sha512Base64(target);
  const lock = scenario.holdTarget
    ? await holdTargetOpen(target, evidence)
    : null;
  const launcher = spawn(target, [], {
    cwd: installDirectory,
    env: {
      ...process.env,
      NVM_PACKAGED_STARTUP_SMOKE: '1',
      NVM_PORTABLE_SMOKE_MARKER: marker,
      NVM_PORTABLE_UPDATE_SMOKE_PACKAGE: scenario.package,
      NVM_PORTABLE_UPDATE_SMOKE_SHA512: scenario.sha512,
      NVM_PORTABLE_UPDATE_SMOKE_VERSION: scenario.version,
      NVM_TEST_ARTIFACT_DIR: evidence,
      NVM_TEST_MODE: '1',
      NVM_TEST_USER_DATA_DIR: userData,
    },
    stdio: 'ignore',
  });
  let relaunched;
  let releasedAt;
  try {
    const first = await waitFor('the old build to start', () =>
      readJson(marker),
    );
    assert.equal(first.appVersion, input.oldVersion);
    if (lock) releasedAt = await lock.releaseAfterAppExit(first.pid);
    const result = await waitFor('the helper result', () =>
      readJson(resultPath),
    );
    relaunched = await waitFor('the relaunched app', () => {
      const ready = readJson(marker);
      return ready && ready.pid !== first.pid ? ready : null;
    });
    fs.copyFileSync(resultPath, path.join(evidence, 'result.json'));
    const helperLog = path.join(userData, 'portable-update', 'helper.log');
    if (fs.existsSync(helperLog))
      fs.copyFileSync(helperLog, path.join(evidence, 'helper.log'));
    assert.equal(result.outcome, scenario.expectedOutcome, result.reason);
    assert.match(result.reason, scenario.expectedReason);
    assert.equal(relaunched.appVersion, scenario.expectedRelaunchVersion);
    assert.equal(
      path.win32.resolve(relaunched.portableExecutableFile).toLowerCase(),
      target.toLowerCase(),
    );
    assert.equal(
      sha512Base64(target),
      scenario.expectedOutcome === 'installed' ? input.newSha512 : oldSha512,
    );
    assert.deepEqual(fs.readdirSync(installDirectory), [userFileName]);
    if (lock) {
      assert.ok(
        Date.parse(result.finishedAt) > Date.parse(releasedAt),
        'The helper replaced the executable while another handle still held it.',
      );
    }
    process.stdout.write(
      `${scenario.name}: ${result.outcome} (${result.reason}); relaunched ${relaunched.appVersion}\n`,
    );
  } finally {
    lock?.holder.kill();
    stopProcessTree(relaunched?.pid);
    stopProcessTree(launcher.pid);
    await new Promise((resolve) => setTimeout(resolve, 2000));
    fs.rmSync(root, {
      force: true,
      recursive: true,
      maxRetries: 10,
      retryDelay: 500,
    });
  }
}

async function run() {
  assert.equal(
    process.platform,
    'win32',
    'Portable update smoke needs Windows.',
  );
  const [oldPortable, oldVersion, newPortable, newVersion, artifactDirectory] =
    process.argv
      .slice(2)
      .map((value, index) =>
        index === 1 || index === 3 ? value : path.resolve(value),
      );
  assert.ok(
    oldPortable && oldVersion && newPortable && newVersion && artifactDirectory,
    'usage: windows-portable-update-smoke.cjs <old.exe> <old-version> <new.exe> <new-version> <artifact-dir>',
  );
  assert.notEqual(oldVersion, newVersion);
  fs.mkdirSync(artifactDirectory, { recursive: true });
  const corruptPackage = path.join(artifactDirectory, 'corrupt-update.exe');
  fs.writeFileSync(corruptPackage, crypto.randomBytes(4096));
  const input = {
    artifactDirectory,
    newSha512: sha512Base64(newPortable),
    oldPortable,
    oldVersion,
  };
  const scenarios = [
    {
      name: 'installs-and-relaunches',
      package: newPortable,
      sha512: input.newSha512,
      version: newVersion,
      expectedOutcome: 'installed',
      expectedReason: /^healthy$/,
      expectedRelaunchVersion: newVersion,
    },
    {
      name: 'waits-for-other-handles',
      package: newPortable,
      sha512: input.newSha512,
      version: newVersion,
      holdTarget: true,
      expectedOutcome: 'installed',
      expectedReason: /^healthy$/,
      expectedRelaunchVersion: newVersion,
    },
    {
      name: 'rejects-checksum-mismatch',
      package: newPortable,
      sha512: sha512Base64(corruptPackage),
      version: newVersion,
      expectedOutcome: 'failed',
      expectedReason: /^integrity:/,
      expectedRelaunchVersion: oldVersion,
    },
    {
      name: 'rolls-back-unstartable-update',
      package: corruptPackage,
      sha512: sha512Base64(corruptPackage),
      version: newVersion,
      expectedOutcome: 'rolled-back',
      expectedReason: /^startup-failed:/,
      expectedRelaunchVersion: oldVersion,
    },
  ];
  for (const scenario of scenarios) await runScenario(scenario, input);
  process.stdout.write('Windows portable update smoke passed.\n');
}

run().catch((error) => {
  process.stderr.write(`${error instanceof Error ? error.stack : error}\n`);
  process.exitCode = 1;
});

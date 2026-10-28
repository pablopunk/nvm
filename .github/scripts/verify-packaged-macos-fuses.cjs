#!/usr/bin/env node
'use strict';

const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

const startupTimeoutMilliseconds = 90_000;
const stabilityMilliseconds = 5000;
const disabledFuseState = 48;
const requiredDisabledFuses = [
  'RunAsNode',
  'EnableNodeOptionsEnvironmentVariable',
  'EnableNodeCliInspectArguments',
];
const inheritedEnvironmentPrefixes = ['NVM_', 'ELECTRON_', 'NODE_OPTIONS'];

function loadFusesModule() {
  const electronBuilderEntry = require.resolve('electron-builder');
  const appBuilderLibEntry = require.resolve('app-builder-lib', {
    paths: [path.dirname(electronBuilderEntry)],
  });
  return require(
    require.resolve('@electron/fuses', {
      paths: [path.dirname(appBuilderLibEntry)],
    }),
  );
}

function fusesNotDisabled(fuseWire, fuseOptions) {
  return requiredDisabledFuses.flatMap(function describeEnabledFuse(name) {
    const state = fuseWire[fuseOptions[name]];
    return state === disabledFuseState ? [] : [`${name}=${state}`];
  });
}

function appExecutablePath(appPath) {
  return path.posix.join(
    appPath,
    'Contents',
    'MacOS',
    path.posix.basename(appPath, '.app'),
  );
}

function isolatedLaunchEnvironment(baseEnvironment, overrides) {
  const inherited = Object.fromEntries(
    Object.entries(baseEnvironment).filter(function keepUnrelated([name]) {
      return !inheritedEnvironmentPrefixes.some(function matches(prefix) {
        return name.startsWith(prefix);
      });
    }),
  );
  return { ...inherited, ...overrides };
}

function nodeInjectionProbe(token) {
  const middle = Math.floor(token.length / 2);
  return {
    arguments: [
      '-e',
      `console.log(${JSON.stringify(token.slice(0, middle))} + ${JSON.stringify(token.slice(middle))})`,
    ],
    expectedOutput: token,
  };
}

function outputShowsInjectedNodeExecution(output, probe) {
  return output.includes(probe.expectedOutput);
}

function stopProcessGroup(child) {
  if (!child.pid) return;
  try {
    process.kill(-child.pid, 'SIGKILL');
  } catch {
    child.kill('SIGKILL');
  }
}

function processHasExited(child) {
  return child.exitCode !== null || child.signalCode !== null;
}

function launchPackagedApp(executable, options) {
  const markerPath = path.join(
    options.evidenceDirectory,
    `${options.name}.json`,
  );
  const output = { text: '' };
  const logPath = path.join(options.evidenceDirectory, `${options.name}.log`);
  const child = spawn(executable, options.arguments, {
    detached: true,
    env: isolatedLaunchEnvironment(process.env, {
      ...options.environment,
      NVM_PACKAGED_STARTUP_SMOKE: '1',
      NVM_PORTABLE_SMOKE_MARKER: markerPath,
      NVM_TEST_ARTIFACT_DIR: options.evidenceDirectory,
      NVM_TEST_MODE: '1',
      NVM_TEST_USER_DATA_DIR: options.userDataDirectory,
    }),
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  function recordOutput(chunk) {
    output.text += chunk.toString('utf8');
    fs.writeFileSync(logPath, output.text);
  }
  child.stdout.on('data', recordOutput);
  child.stderr.on('data', recordOutput);
  return { child, markerPath, output };
}

function waitForMarker(launched) {
  return new Promise(function waitForReadiness(resolve, reject) {
    const deadline = Date.now() + startupTimeoutMilliseconds;
    const interval = setInterval(function checkReadiness() {
      if (fs.existsSync(launched.markerPath)) {
        clearInterval(interval);
        try {
          resolve(JSON.parse(fs.readFileSync(launched.markerPath, 'utf8')));
        } catch (error) {
          reject(error);
        }
      } else if (processHasExited(launched.child)) {
        clearInterval(interval);
        reject(
          new Error(
            `Fused app exited before readiness (code ${launched.child.exitCode}, signal ${launched.child.signalCode}). Output: ${launched.output.text}`,
          ),
        );
      } else if (Date.now() > deadline) {
        clearInterval(interval);
        reject(
          new Error(
            `Fused app did not write its renderer-ready marker within ${startupTimeoutMilliseconds / 1000} seconds. Output: ${launched.output.text}`,
          ),
        );
      }
    }, 250);
  });
}

async function requireStableChild(launched) {
  const deadline = Date.now() + stabilityMilliseconds;
  while (Date.now() < deadline) {
    assert.equal(
      processHasExited(launched.child),
      false,
      `Fused app exited during the ${stabilityMilliseconds / 1000}-second stability window. Output: ${launched.output.text}`,
    );
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
}

function removeDirectory(directory) {
  fs.rmSync(directory, {
    force: true,
    recursive: true,
    maxRetries: 10,
    retryDelay: 250,
  });
}

async function runFusedLaunch(executable, options) {
  const userDataDirectory = fs.mkdtempSync(
    path.join(os.tmpdir(), 'nvm-fuses-smoke-'),
  );
  const launched = launchPackagedApp(executable, {
    ...options,
    userDataDirectory,
  });
  try {
    const marker = await waitForMarker(launched);
    assert.equal(marker.appIsPackaged, true);
    assert.equal(
      fs.realpathSync(marker.processExecPath),
      fs.realpathSync(executable),
    );
    await requireStableChild(launched);
    return launched.output.text;
  } finally {
    stopProcessGroup(launched.child);
    removeDirectory(userDataDirectory);
  }
}

async function assertFusesDisabled(appPath, evidenceDirectory) {
  const fuses = loadFusesModule();
  const fuseWire = await fuses.getCurrentFuseWire(appPath);
  fs.writeFileSync(
    path.join(evidenceDirectory, 'fuse-wire.json'),
    `${JSON.stringify(fuseWire, null, 2)}\n`,
  );
  assert.deepEqual(
    fusesNotDisabled(fuseWire, fuses.FuseV1Options),
    [],
    'Packaged macOS app must ship with the code-injection fuses disabled.',
  );
}

async function assertAppStartsWithoutNodeInjection(
  executable,
  evidenceDirectory,
) {
  const probe = nodeInjectionProbe(
    `nvm-fuse-probe-${crypto.randomBytes(8).toString('hex')}`,
  );
  await runFusedLaunch(executable, {
    name: 'fused-launch',
    arguments: [],
    environment: {},
    evidenceDirectory,
  });
  const injectedOutput = await runFusedLaunch(executable, {
    name: 'run-as-node-launch',
    arguments: probe.arguments,
    environment: { ELECTRON_RUN_AS_NODE: '1' },
    evidenceDirectory,
  });
  assert.equal(
    outputShowsInjectedNodeExecution(injectedOutput, probe),
    false,
    'ELECTRON_RUN_AS_NODE=1 executed injected JavaScript in the fused app.',
  );
}

async function run() {
  assert.equal(
    process.platform,
    'darwin',
    'Packaged fuse verification must run on macOS.',
  );
  const [appInput, evidenceInput = 'test-results/packaged-macos-fuses'] =
    process.argv.slice(2);
  assert.ok(
    appInput,
    'usage: verify-packaged-macos-fuses.cjs <Nevermind.app> [evidence-dir]',
  );
  const appPath = path.resolve(appInput);
  const executable = appExecutablePath(appPath);
  assert.equal(fs.statSync(executable).isFile(), true);
  const evidenceDirectory = path.resolve(evidenceInput);
  fs.mkdirSync(evidenceDirectory, { recursive: true });
  await assertFusesDisabled(appPath, evidenceDirectory);
  await assertAppStartsWithoutNodeInjection(executable, evidenceDirectory);
  process.stdout.write(`Packaged macOS fuse verification passed: ${appPath}\n`);
}

module.exports = {
  appExecutablePath,
  fusesNotDisabled,
  isolatedLaunchEnvironment,
  nodeInjectionProbe,
  outputShowsInjectedNodeExecution,
};

if (require.main === module) {
  run().catch((error) => {
    process.stderr.write(`${error instanceof Error ? error.stack : error}\n`);
    process.exitCode = 1;
  });
}

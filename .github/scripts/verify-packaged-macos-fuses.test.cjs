'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const {
  appExecutablePath,
  fusesNotDisabled,
  isolatedLaunchEnvironment,
  nodeInjectionProbe,
  outputShowsInjectedNodeExecution,
} = require('./verify-packaged-macos-fuses.cjs');

const fuseOptions = {
  RunAsNode: 0,
  EnableNodeOptionsEnvironmentVariable: 2,
  EnableNodeCliInspectArguments: 3,
};

test('accepts a fuse wire with the code-injection fuses disabled', function acceptsDisabledFuses() {
  const wire = { version: '1', 0: 48, 1: 49, 2: 48, 3: 48 };
  assert.deepEqual(fusesNotDisabled(wire, fuseOptions), []);
});

test('names every code-injection fuse that is enabled, removed or missing', function namesEnabledFuses() {
  const wire = { version: '1', 0: 49, 2: 114 };
  assert.deepEqual(fusesNotDisabled(wire, fuseOptions), [
    'RunAsNode=49',
    'EnableNodeOptionsEnvironmentVariable=114',
    'EnableNodeCliInspectArguments=undefined',
  ]);
});

test('derives the executable from the app bundle name', function derivesExecutable() {
  assert.equal(
    appExecutablePath('/tmp/dist/mac-arm64/Nevermind.app'),
    '/tmp/dist/mac-arm64/Nevermind.app/Contents/MacOS/Nevermind',
  );
});

test('drops inherited test, Electron and Node option variables before applying overrides', function isolatesEnvironment() {
  const environment = isolatedLaunchEnvironment(
    {
      HOME: '/Users/runner',
      NVM_TEST_MODE: '0',
      ELECTRON_RUN_AS_NODE: '1',
      NODE_OPTIONS: '--inspect',
    },
    { NVM_TEST_MODE: '1' },
  );
  assert.deepEqual(environment, { HOME: '/Users/runner', NVM_TEST_MODE: '1' });
});

test('the probe argv never contains the output it expects the app to print', function probeIsNotEchoedByArguments() {
  const probe = nodeInjectionProbe('nvm-fuse-probe-abcdef');
  assert.equal(probe.arguments[0], '-e');
  assert.equal(probe.arguments[1].includes(probe.expectedOutput), false);
  assert.equal(
    outputShowsInjectedNodeExecution('nvm-fuse-probe-abcdef\n', probe),
    true,
  );
  assert.equal(outputShowsInjectedNodeExecution('app started\n', probe), false);
});

function macosPackagedJobSteps() {
  const workflow = fs.readFileSync(
    path.resolve(__dirname, '..', 'workflows', 'ci.yml'),
    'utf8',
  );
  const job = workflow
    .split('\n  pi-packaged-macos:')[1]
    .split('\n  pi-integration-')[0];
  return job.split('\n      - name: ').slice(1);
}

test('the macOS packaged job verifies the fused app right after packaging it', function verifiesFusedAppAfterPackaging() {
  const steps = macosPackagedJobSteps();
  const packagingIndex = steps.findIndex((step) =>
    step.startsWith('Package unsigned native-architecture app'),
  );
  const fusesStep = steps[packagingIndex + 2] || '';
  assert.ok(packagingIndex > -1);
  assert.match(fusesStep, /verify-packaged-macos-fuses\.cjs/);
});

'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const { startupFailureFromOutput } = require('./windows-first-run-smoke.cjs');

test('detects Electron entry errors even after build success markers', () => {
  const output =
    'starting electron app...\nApp threw an error during load\nError [ERR_MODULE_NOT_FOUND]';
  assert.equal(
    startupFailureFromOutput(output),
    'App threw an error during load',
  );
  assert.equal(
    startupFailureFromOutput(
      'extension.dev.load.failed: Error [ERR_MODULE_NOT_FOUND]',
    ),
    null,
  );
  assert.equal(startupFailureFromOutput('starting electron app...'), null);
});

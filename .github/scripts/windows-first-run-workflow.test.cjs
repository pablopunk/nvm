'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');

function extractWindowsFirstRunJob(workflow) {
  const jobStart = workflow.indexOf('\n  windows-first-run:\n');
  const jobEnd = workflow.indexOf('\n  postgres-integration:\n', jobStart);
  assert.notEqual(jobStart, -1, 'CI must define the Windows first-run job.');
  assert.notEqual(jobEnd, -1, 'The Windows first-run job must be complete.');
  return workflow.slice(jobStart, jobEnd);
}

function assertIncludes(source, expected) {
  assert.equal(source.includes(expected), true, `Expected ${expected}`);
}

test('Windows CI uses standalone pnpm for the automatic first-run repair', () => {
  const workflow = fs.readFileSync('.github/workflows/ci.yml', 'utf8');
  const job = extractWindowsFirstRunJob(workflow);

  assertIncludes(job, 'runs-on: windows-latest');
  assertIncludes(job, 'pnpm-win32-x64.zip');
  assertIncludes(job, 'PNPM_STANDALONE_PATH=$pnpm');
  assertIncludes(job, 'GITHUB_PATH');
  assertIncludes(job, 'install --frozen-lockfile');
  assertIncludes(job, 'extension-json-store.test.ts');
  assertIncludes(job, 'extension-storage.test.ts');
  assertIncludes(job, 'data-loader.test.ts');
  assertIncludes(job, 'node .github/scripts/windows-first-run-smoke.cjs');
  assert.equal(job.split('ELECTRON_SKIP_BINARY_DOWNLOAD').length - 1, 2);
  assert.equal(job.includes('approve-builds'), false);
  assert.equal(job.includes('rebuild electron'), false);
});

test('Windows smoke launches the exact root dev command and verifies startup', () => {
  const smoke = fs.readFileSync(
    '.github/scripts/windows-first-run-smoke.cjs',
    'utf8',
  );

  assertIncludes(smoke, "spawn(pnpmExecutable, ['run', 'dev']");
  assertIncludes(smoke, 'electron.exe');
  assertIncludes(smoke, 'starting electron app...');
  assertIncludes(smoke, 'taskkill');
  assert.equal(smoke.includes('approve-builds'), false);
  assert.equal(smoke.includes('rebuild electron'), false);
});

'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');

test('Windows CI uses standalone pnpm for the automatic first-run repair', () => {
  const workflow = fs.readFileSync('.github/workflows/ci.yml', 'utf8');
  const job = workflow.match(
    /^  windows-first-run:\n([\s\S]*?)(?=^  postgres-integration:)/m,
  )?.[1];

  assert.ok(job, 'CI must define the Windows first-run job.');
  assert.match(job, /runs-on: windows-latest/);
  assert.match(job, /pnpm-win32-x64\.zip/);
  assert.match(job, /PNPM_STANDALONE_PATH=\$pnpm/);
  assert.match(job, /install --frozen-lockfile/);
  assert.match(job, /node \.github\/scripts\/windows-first-run-smoke\.cjs/);
  assert.equal((job.match(/ELECTRON_SKIP_BINARY_DOWNLOAD/g) ?? []).length, 2);
  assert.doesNotMatch(job, /approve-builds|rebuild electron/i);
});

test('Windows smoke launches the exact root dev command and verifies startup', () => {
  const smoke = fs.readFileSync(
    '.github/scripts/windows-first-run-smoke.cjs',
    'utf8',
  );

  assert.match(smoke, /spawn\(pnpmExecutable, \['run', 'dev'\]/);
  assert.match(smoke, /electron\.exe/);
  assert.match(smoke, /starting electron app\.\.\./);
  assert.match(smoke, /taskkill/);
  assert.doesNotMatch(smoke, /approve-builds|rebuild electron/i);
});

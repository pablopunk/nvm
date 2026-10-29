'use strict';
const assert = require('node:assert/strict');
const { mkdtempSync, rmSync, writeFileSync } = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test } = require('node:test');
const {
  desktopBuildIdentity,
} = require('../../scripts/desktop-build-identity.cjs');

test('pinned source commit overrides the scheduled workflow event SHA', (t) => {
  const root = mkdtempSync(path.join(os.tmpdir(), 'nvm-build-identity-'));
  const previousSource = process.env.NEVERMIND_SOURCE_COMMIT;
  const previousEvent = process.env.GITHUB_SHA;
  t.after(() => {
    if (previousSource === undefined)
      delete process.env.NEVERMIND_SOURCE_COMMIT;
    else process.env.NEVERMIND_SOURCE_COMMIT = previousSource;
    if (previousEvent === undefined) delete process.env.GITHUB_SHA;
    else process.env.GITHUB_SHA = previousEvent;
    rmSync(root, { recursive: true, force: true });
  });
  writeFileSync(path.join(root, 'package.json'), '{"version":"2026.1029.1"}');
  process.env.NEVERMIND_SOURCE_COMMIT = 'a'.repeat(40);
  process.env.GITHUB_SHA = 'b'.repeat(40);
  assert.equal(
    desktopBuildIdentity(root),
    `nevermind@2026.1029.1+${'a'.repeat(40)}.${process.platform}`,
  );
});

// biome-ignore-all lint/performance/useTopLevelRegex: Contract assertions are evaluated once per test.
'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');

const verifier = fs.readFileSync(
  '.github/workflows/nightly-package-verify.yml',
  'utf8',
);
const ci = fs.readFileSync('.github/workflows/ci.yml', 'utf8');
const nightly = fs.readFileSync(
  '.github/workflows/nightly-release.yml',
  'utf8',
);

// Commands and metadata names that define a nightly package. Both the publishing
// workflow and the PR verifier must contain every entry, so drift fails here.
const SHARED_PACKAGE_CONTRACT = [
  'pnpm run dist:win:x64',
  'pnpm run dist:mac:all',
  'set-package-version --version "$NIGHTLY_VERSION"',
  "CSC_IDENTITY_AUTO_DISCOVERY: 'false'",
  '-UpdaterMetadataName nightly.yml',
  'validate-linux-updater-metadata.cjs',
  'release/nightly.yml nightly-windows/',
  'release/nightly-mac.yml nightly-mac/',
  'cp release/nightly-linux-arm64.yml nightly-linux/nightly-linux-arm64.yml',
];

function section(source, start, end) {
  const startIndex = source.indexOf(start);
  const endIndex = source.indexOf(end, startIndex + start.length);
  assert.notEqual(startIndex, -1, `missing ${start}`);
  assert.notEqual(endIndex, -1, `missing ${end}`);
  return source.slice(startIndex, endIndex);
}

test('the verifier is a secret-free reusable workflow with no triggers of its own', () => {
  assert.match(verifier, /^on:\n {2}workflow_call:\n/m);
  assert.doesNotMatch(
    verifier,
    /^ {2}(?:pull_request|pull_request_target|push|workflow_dispatch|workflow_run|schedule):/m,
  );
  assert.doesNotMatch(
    verifier,
    /secrets\./,
    'fork pull requests must never receive publication or signing secrets',
  );
});

test('pull requests and the merge queue call the verifier with no secrets and read-only permissions', () => {
  const job = section(
    ci,
    '\n  nightly-package-verify:\n',
    '\n  release-windows:\n',
  );
  assert.match(
    job,
    /if: github\.event_name == 'pull_request' \|\| github\.event_name == 'merge_group'/,
  );
  assert.match(
    job,
    /uses: \.\/\.github\/workflows\/nightly-package-verify\.yml/,
  );
  assert.match(job, /permissions:\n {6}contents: read\n/);
  assert.doesNotMatch(job, /secrets/);
  assert.doesNotMatch(ci, /pull_request_target/);
  assert.match(
    job,
    /base_sha: \$\{\{ github\.event\.pull_request\.base\.sha \|\| github\.event\.merge_group\.base_sha \}\}/,
  );
  assert.match(
    job,
    /head_sha: \$\{\{ github\.event\.pull_request\.head\.sha \|\| github\.event\.merge_group\.head_sha \}\}/,
  );
});

test('the verifier never grants write access, publishes, or mutates tags', () => {
  assert.doesNotMatch(verifier, /: write$/m);
  assert.doesNotMatch(
    verifier,
    /gh release|release upload|git push|git tag|gh api -X|gh workflow/,
  );
});

test('verification does not depend on production health, CI gates, or publication guards', () => {
  for (const dependency of [
    'PRODUCTION_API_ORIGIN',
    'api.nvm.fyi',
    'identity --sha',
    'publish-guard',
    'ci-gate',
    'schedule-gate',
  ]) {
    assert.equal(verifier.includes(dependency), false, dependency);
  }
});

test('the verifier builds without the release-only Sentry upload that needs credentials', () => {
  assert.doesNotMatch(
    verifier,
    /NEVERMIND_RELEASE_BUILD|SENTRY_/,
    'NEVERMIND_RELEASE_BUILD makes scripts/upload-desktop-source-maps.cjs require SENTRY_AUTH_TOKEN, which secret-free PR builds lack',
  );
});

test('verification packages through the same commands and metadata names as the nightly build', () => {
  for (const command of SHARED_PACKAGE_CONTRACT) {
    assert.equal(nightly.includes(command), true, `nightly: ${command}`);
    assert.equal(verifier.includes(command), true, `verifier: ${command}`);
  }
  const linuxBuild = /pnpm run dist:linux:\$\{\{ matrix\.arch \}\}/;
  assert.match(nightly, linuxBuild);
  assert.match(verifier, linuxBuild);
});

test('verification stamps the nightly version shape and selects builds with the nightly classifier', () => {
  assert.match(verifier, /formatNightlyVersion/);
  assert.match(verifier, /classifyNightlyAppChanges/);
  assert.match(verifier, /isVersionOnlyPackageJsonChange/);
  assert.match(verifier, /nightly-app-paths\.cjs/);
  assert.match(
    verifier,
    /NIGHTLY_VERSION: \$\{\{ needs\.plan\.outputs\.version \}\}/,
  );
  assert.match(
    verifier,
    /nightly-package-verify\|nightly-release\|ci/,
    'changes to the verifier, nightly workflow, and CI must trigger it',
  );
});

test('every platform stages nightly-verify artifacts and one job validates the complete asset set', () => {
  for (const name of [
    'name: nightly-verify-windows',
    'name: nightly-verify-mac',
    'name: nightly-verify-linux-',
  ]) {
    assert.equal(verifier.includes(name), true, name);
  }
  const validate = verifier.slice(verifier.indexOf('\n  validate-assets:\n'));
  assert.match(
    validate,
    /needs: \[plan, build-windows, build-mac, build-linux\]/,
  );
  assert.match(validate, /pattern: nightly-verify-\*/);
  assert.match(
    validate,
    /validate-assets --dir dist-nightly --version "\$VERSION"/,
  );
});

test('the verifier is cancelled when a pull request is superseded', () => {
  assert.match(
    verifier,
    /concurrency:\n {2}group: nightly-package-verify-\$\{\{ github\.event\.pull_request\.number \|\| github\.ref \}\}\n {2}cancel-in-progress: \$\{\{ github\.event_name == 'pull_request' \}\}/,
  );
});

test('nightly publication gates are unchanged and independent of PR verification', () => {
  assert.doesNotMatch(nightly, /nightly-package-verify/);
  assert.match(
    nightly,
    /needs: \[plan, build-windows, build-mac, build-linux, validate-assets\]/,
  );
});

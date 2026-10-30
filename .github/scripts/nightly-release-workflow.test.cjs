'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');

const workflow = fs.readFileSync(
  '.github/workflows/nightly-release.yml',
  'utf8',
);
const ci = fs.readFileSync('.github/workflows/ci.yml', 'utf8');
const windowsVerifier = fs.readFileSync(
  '.github/scripts/verify-windows-package.ps1',
  'utf8',
);

test('nightly schedule registers both UTC hours and defers the Madrid 04:00 choice to the gate', () => {
  const promotion = fs.readFileSync(
    '.github/workflows/promote-nightly.yml',
    'utf8',
  );
  assert.match(promotion, /- cron: '0 2 \* \* \*'/);
  assert.match(promotion, /- cron: '0 3 \* \* \*'/);
  assert.match(promotion, /schedule-gate/);
  const gates = fs.readFileSync(
    '.github/scripts/nightly-release-gates.cjs',
    'utf8',
  );
  assert.match(gates, /Europe\/Madrid/);
  assert.match(gates, /SCHEDULED_MADRID_HOUR = 4/);
  assert.doesNotMatch(
    workflow,
    /workflow_run:/,
    'publication must not depend on workflow_run head metadata',
  );
});

test('manual trigger defaults to a dry run that never publishes', () => {
  assert.match(
    workflow,
    /workflow_dispatch:\n {4}inputs:\n {6}dry_run:[\s\S]*?default: true/,
  );
  assert.match(
    workflow,
    /dry_run: \$\{\{ github\.event_name == 'workflow_dispatch' && inputs\.dry_run \}\}/,
  );
  assert.match(workflow, /needs\.plan\.outputs\.dry_run != 'true'/);
  assert.match(workflow, /inputs\.dry_run && github\.sha \|\| 'main'/);
  assert.match(
    workflow,
    /build: \$\{\{ steps\.plan\.outputs\.build == 'true' && \(steps\.identity\.outputs\.verified == 'true' \|\| \(github\.event_name == 'workflow_dispatch' && inputs\.dry_run\)\) \}\}/,
  );
  assert.match(workflow, / {2}validate-assets:/);
});

test('every tag-driven stable release job refuses a tag older than the latest stable', () => {
  assert.equal(
    ci.match(/mise exec -- node \.github\/scripts\/release-tag-guard\.cjs/g)
      ?.length,
    3,
  );
});

test('manual runs cannot publish legacy nightlies outside promotion', () => {
  assert.match(
    workflow,
    /Refuse legacy publication\n {8}if: \$\{\{ !inputs\.dry_run \}\}[\s\S]*?promote-nightly\.yml[\s\S]*?exit 1/,
  );
});

test('runs are serialized and never cancel an in-flight publication', () => {
  assert.match(
    workflow,
    /concurrency:\n {2}group: channel-promotion\n {2}queue: max/,
  );
});

test('top-level permissions are read-only and only publish and retain can write contents', () => {
  assert.match(workflow, /^permissions:\n {2}contents: read\n/m);
  const writeGrants = workflow.match(/contents: write/g) ?? [];
  assert.equal(writeGrants.length, 2, 'publish and retain jobs only');
});

test('publication creates a draft prerelease first and makes it public only after every check', () => {
  const draftIndex = workflow.indexOf(
    'gh release create "$TAG" --draft --prerelease',
  );
  const verifyIndex = workflow.indexOf(
    'diff staged-assets.txt draft-assets.txt',
  );
  const publishIndex = workflow.indexOf(
    'gh release edit "$TAG" --draft=false --prerelease',
  );
  assert.ok(
    draftIndex > 0 && verifyIndex > draftIndex && publishIndex > verifyIndex,
  );
  assert.match(workflow, /--target "\$SOURCE_SHA"/);
  assert.ok(workflow.indexOf('published_commit="$(gh api') < publishIndex);
});

test('the publication recheck re-verifies CI, production identity, and the publication race', () => {
  const publishSection = workflow.slice(
    workflow.indexOf('  publish:'),
    workflow.indexOf('  retain:'),
  );
  assert.match(publishSection, /ci-gate --sha/);
  assert.match(publishSection, /identity --sha/);
  assert.match(publishSection, /publish-guard --sha/);
  const recheckIndex = publishSection.lastIndexOf('publish-guard --sha');
  const publishIndex = publishSection.indexOf('gh release edit');
  assert.ok(
    recheckIndex < publishIndex,
    'race recheck happens before going public',
  );
});

test('publish requires every platform build and a completed identity gate', () => {
  assert.match(
    workflow,
    /needs: \[plan, build-windows, build-mac, build-linux(?:, validate-assets)?\]/,
  );
  assert.match(
    workflow,
    /identity --sha "\$SOURCE_SHA" --health production-health\.json/,
  );
  assert.match(workflow, /jq -e '\.status == "verified"'/);
  assert.match(workflow, /jq -e '\.ok == true'/);
});

test('nightly builds reuse the stable packaging commands', () => {
  assert.match(workflow, /pnpm run dist:win:x64/);
  assert.match(workflow, /pnpm run dist:mac:arm64/);
  assert.match(workflow, /pnpm run dist:linux:\$\{\{ matrix\.arch \}\}/);
});

test('nightly versions are stamped into package.json only in the build checkout, never committed', () => {
  assert.match(workflow, /set-package-version --version "\$NIGHTLY_VERSION"/);
  assert.doesNotMatch(workflow, /git commit|git push/);
});

test('nightlies never publish latest-named updater metadata and are never the stable channel', () => {
  assert.doesNotMatch(workflow, /latest(-mac|-linux)?\.yml/);
  assert.match(workflow, /--prerelease/);
});

test('Windows nightly verification checks nightly.yml and stable default stays latest.yml', () => {
  assert.match(workflow, /-UpdaterMetadataName nightly\.yml/);
  assert.match(
    windowsVerifier,
    /\[ValidateSet\('latest\.yml', 'nightly\.yml'\)\]\[string\]\$UpdaterMetadataName = 'latest\.yml'/,
  );
});

test('stable release workflow excludes prerelease tags', () => {
  assert.match(ci, /tags: \['v\*', '!v\*-\*'\]/);
  assert.match(ci, /!contains\(github\.ref_name, '-'\)/);
});

test('retention deletes only after a successful publish and uses cleanup-tag for expired nightlies', () => {
  const retainSection = workflow.slice(workflow.indexOf('  retain:'));
  assert.match(retainSection, /needs: \[plan, publish\]/);
  assert.match(retainSection, /if: needs\.publish\.result == 'success'/);
  assert.match(
    retainSection,
    /gh release delete "\$expired_tag" --cleanup-tag --yes/,
  );
});

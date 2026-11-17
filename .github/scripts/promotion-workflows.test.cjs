'use strict';
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { test } = require('node:test');
const nightly = readFileSync('.github/workflows/promote-nightly.yml', 'utf8');
const stable = readFileSync('.github/workflows/promote-stable.yml', 'utf8');
const retention = readFileSync('.github/workflows/dev-retention.yml', 'utf8');
const dev = readFileSync('.github/workflows/dev-build.yml', 'utf8');

test('promotion verifies retained and uploaded bytes and never packages again', () => {
  for (const workflow of [nightly, stable]) {
    assert.match(workflow, /promotion-manifest\.cjs verify/);
    assert.match(workflow, /validate-assets/);
    assert.match(workflow, /diff -u expected-sha256\.txt remote-sha256\.txt/);
    assert.match(workflow, /identity --sha/);
    assert.match(workflow, /Draft is not owned by this promotion/);
    assert.match(workflow, /points to another commit/);
    assert.doesNotMatch(workflow, /pnpm (?:run )?dist:/);
    assert.match(workflow, /group: channel-promotion/);
  }
});
test('nightly selection and retention share the promotion lock', () => {
  assert.match(nightly, /nightly-promotion-plan\.cjs/);
  assert.match(retention, /group: channel-promotion/);
  assert.match(retention, /promotion-selection\.cjs retention/);
  assert.match(dev, /publish-dev:[\s\S]*?group: channel-promotion/);
});
test('the dev build packages the Go app for release and a dry run never publishes', () => {
  assert.match(dev, /uses: \.\/\.github\/workflows\/native-package\.yml[\s\S]*?release: true[\s\S]*?secrets: inherit/);
  assert.match(dev, /retarget-metadata --dir dist-nightly --version "\$VERSION" --tag "v\$\{VERSION\}-dev"/);
  assert.match(dev, /publish-dev:[\s\S]*?if: \$\{\{ !inputs\.dry_run \}\}/);
  assert.match(dev, /repair-dev-pointer:[\s\S]*?!inputs\.dry_run/);
  assert.match(nightly, /retarget-metadata --dir dev --version "\$VERSION" --tag "v\$\{VERSION\}-nightly"/);
  assert.match(stable, /retarget-metadata --dir dev --version "\$VERSION" --tag "v\$VERSION"/);
  for (const workflow of [nightly, stable])
    assert.match(workflow, /for workflow in ci\.yml native\.yml/);
});

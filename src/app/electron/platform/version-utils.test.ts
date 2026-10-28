import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  compareSemverVersions,
  isNewerVersion,
  parseSemverVersion,
} from './version-utils';

test('a stable release is newer than its own nightly prerelease', () => {
  assert.equal(isNewerVersion('0.24.1', '0.24.1-nightly.20261028.1'), true);
  assert.equal(isNewerVersion('0.24.1-nightly.20261028.1', '0.24.1'), false);
  assert.equal(isNewerVersion('v0.24.1', 'v0.24.1-nightly.20261028.1'), true);
});

test('orders nightly prereleases below their stable release', () => {
  assert.equal(
    compareSemverVersions('0.24.1-nightly.20261028.1', '0.24.1'),
    -1,
  );
  assert.equal(compareSemverVersions('0.24.2-nightly.20261028.1', '0.24.1'), 1);
});

test('orders nightly builds by date and run number numerically', () => {
  assert.equal(
    isNewerVersion('0.24.1-nightly.20261028.2', '0.24.1-nightly.20261028.1'),
    true,
  );
  assert.equal(
    isNewerVersion('0.24.1-nightly.20261028.10', '0.24.1-nightly.20261028.9'),
    true,
  );
  assert.equal(
    isNewerVersion('0.24.1-nightly.20261029.1', '0.24.1-nightly.20261028.999'),
    true,
  );
  assert.equal(
    isNewerVersion('0.24.1-nightly.20261028.1', '0.24.1-nightly.20261028.1'),
    false,
  );
});

test('compares core numbers before prerelease tags', () => {
  assert.equal(isNewerVersion('0.10.0', '0.9.0'), true);
  assert.equal(isNewerVersion('1.0.0', '0.99.99'), true);
  assert.equal(isNewerVersion('0.25.0-nightly.20261028.1', '0.24.9'), true);
  assert.equal(isNewerVersion('0.24.0', '0.24.1-nightly.20261028.1'), false);
});

test('follows the semver prerelease precedence chain', () => {
  const chain = [
    '1.0.0-alpha',
    '1.0.0-alpha.1',
    '1.0.0-alpha.beta',
    '1.0.0-beta',
    '1.0.0-beta.2',
    '1.0.0-beta.11',
    '1.0.0-rc.1',
    '1.0.0',
  ];
  for (let index = 1; index < chain.length; index += 1) {
    const [previous, next] = [chain[index - 1], chain[index]];
    assert.equal(
      compareSemverVersions(previous, next),
      -1,
      `${previous} < ${next}`,
    );
    assert.equal(
      compareSemverVersions(next, previous),
      1,
      `${next} > ${previous}`,
    );
  }
  assert.equal(compareSemverVersions('1.0.0-1', '1.0.0-alpha'), -1);
});

test('ignores build metadata when ordering versions', () => {
  assert.equal(compareSemverVersions('0.24.1+build.5', '0.24.1'), 0);
  assert.equal(isNewerVersion('0.24.1+build.5', '0.24.1'), false);
  assert.equal(
    isNewerVersion(
      '0.24.1-nightly.20261028.1+abcdef1',
      '0.24.1-nightly.20261028.1',
    ),
    false,
  );
});

test('parses stable and nightly versions into core and prerelease parts', () => {
  assert.deepEqual(parseSemverVersion('0.24.1'), {
    core: [0, 24, 1],
    prerelease: [],
  });
  assert.deepEqual(parseSemverVersion('0.24.1-nightly.20261028.1'), {
    core: [0, 24, 1],
    prerelease: ['nightly', '20261028', '1'],
  });
  assert.equal(parseSemverVersion(undefined), null);
});

test('keeps legacy comparison for inputs that are not strict semver', () => {
  assert.equal(isNewerVersion(undefined, '0.24.1'), false);
  assert.equal(isNewerVersion('0.24.1', undefined), true);
  assert.equal(isNewerVersion('0.24.1.1', '0.24.1'), true);
  assert.equal(isNewerVersion('0.24.0.9', '0.24.1'), false);
});

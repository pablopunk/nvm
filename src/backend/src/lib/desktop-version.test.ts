import assert from 'node:assert/strict';
import { test } from 'node:test';
import { compareVersions } from './desktop-version';

test('compares semver-like desktop versions', () => {
  assert.equal(compareVersions('v0.6.0', '0.6.0'), 0);
  assert.equal(compareVersions('0.6.1', '0.6.0'), 1);
  assert.equal(compareVersions('0.5.9', '0.6.0'), -1);
  assert.equal(compareVersions('0.6', '0.6.0'), 0);
  assert.equal(compareVersions('9007199254740993.0.0', '9007199254740992.0.0'), 1);
});

test('orders prereleases below their stable release', () => {
  assert.equal(compareVersions('0.7.0-nightly.20261028.1', '0.7.0'), -1);
  assert.equal(compareVersions('0.7.1-nightly.20261028.1', '0.7.0'), 1);
  assert.equal(compareVersions('0.7.1', '0.7.1-nightly.20261028.42'), 1);
});

test('orders nightly builds by date and run number numerically', () => {
  assert.equal(compareVersions('0.7.1-nightly.20261028.2', '0.7.1-nightly.20261028.1'), 1);
  assert.equal(compareVersions('0.7.1-nightly.20261028.100', '0.7.1-nightly.20261028.42'), 1);
  assert.equal(compareVersions('0.7.1-nightly.9007199254740993', '0.7.1-nightly.9007199254740992'), 1);
  assert.equal(compareVersions('0.7.1-nightly.20261029.1', '0.7.1-nightly.20261028.999'), 1);
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
    assert.equal(compareVersions(chain[index - 1], chain[index]), -1, `${chain[index - 1]} < ${chain[index]}`);
    assert.equal(compareVersions(chain[index], chain[index - 1]), 1, `${chain[index]} > ${chain[index - 1]}`);
  }
  assert.equal(compareVersions('1.0.0-1', '1.0.0-alpha'), -1);
});

test('ignores build metadata when ordering versions', () => {
  assert.equal(compareVersions('0.7.0+build.5', '0.7.0'), 0);
  assert.equal(compareVersions('v0.7.1-nightly.20261028.1+abcdef1', '0.7.1-nightly.20261028.1'), 0);
});

test('keeps legacy ordering for versions that are not strict semver', () => {
  assert.equal(compareVersions('0.6.0.1', '0.6.0'), 1);
  assert.equal(compareVersions('0.5.9.9', '0.6.0'), -1);
});

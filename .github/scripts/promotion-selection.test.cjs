'use strict';
const assert = require('node:assert/strict');
const { test } = require('node:test');
const {
  selectNewestDev,
  expiredDevTags,
  expiredPromotedNightlyTags,
  referencedDevTags,
} = require('./promotion-selection.cjs');

function release(tag_name, published_at = '2026-01-01T00:00:00Z') {
  return { tag_name, published_at, draft: false, prerelease: true };
}

test('preserves dev releases referenced by other releases', () => {
  assert.deepEqual(
    referencedDevTags([{ body: 'Promoted from v2026.101.1-dev.' }]),
    ['v2026.101.1-dev'],
  );
});

test('selection uses version order rather than completion order and skips unchanged', () => {
  const releases = [release('v2026.1029.9-dev'), release('v2026.1029.10-dev')];
  assert.equal(
    selectNewestDev(releases, '2026.1029.9').tag,
    'v2026.1029.10-dev',
  );
  assert.equal(selectNewestDev(releases, '2026.1029.10'), null);
  assert.equal(selectNewestDev([{ ...releases[1], draft: true }], null), null);
});

test('retention preserves protected and newest builds and unknown timestamps', () => {
  const releases = [
    release('v2026.101.1-dev'),
    release('v2026.102.2-dev'),
    release('v2026.103.3-dev', 'invalid'),
  ];
  assert.deepEqual(
    expiredDevTags(releases, new Date('2026-10-29T00:00:00Z'), [
      'v2026.101.1-dev',
    ]),
    ['v2026.102.2-dev'],
  );
});

test('promoted nightlies do not keep their source dev build alive', () => {
  assert.deepEqual(
    referencedDevTags([
      {
        tag_name: 'v2026.1029.5-nightly',
        body: 'Promoted from v2026.1029.4-dev.',
      },
      { tag_name: 'v2026.1029.5', body: 'Promoted from v2026.1029.3-dev.' },
    ]),
    ['v2026.1029.3-dev'],
  );
});

test('promoted nightly retention keeps the newest, pointer targets, and preserved builds', () => {
  const releases = [
    release('v2026.101.1-nightly'),
    release('v2026.102.2-nightly'),
    { ...release('v2026.103.3-nightly'), body: 'Nevermind-Preserve: true' },
    release('v2026.104.4-nightly', 'invalid'),
    release('v2026.105.5-nightly'),
    release('v2026.105.5-dev'),
    { ...release('v2026.106.6-nightly'), draft: true },
  ];
  assert.deepEqual(
    expiredPromotedNightlyTags(releases, new Date('2026-10-29T00:00:00Z'), [
      'v2026.102.2-nightly',
    ]),
    ['v2026.101.1-nightly'],
  );
  assert.throws(
    () => expiredPromotedNightlyTags(releases, new Date('invalid')),
    /Invalid retention input/,
  );
});

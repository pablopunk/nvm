'use strict';
const assert = require('node:assert/strict');
const { test } = require('node:test');
const {
  selectNewestDev,
  expiredDevTags,
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

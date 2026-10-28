'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const { selectNightlyRetention } = require('./nightly-retention.cjs');

const NOW = new Date('2026-10-28T00:00:00Z');

function nightly(version, publishedAt, extra = {}) {
  return {
    tag_name: `v${version}`,
    draft: false,
    prerelease: true,
    published_at: publishedAt,
    ...extra,
  };
}

test('nightlies older than 30 days expire and recent ones are kept', () => {
  const result = selectNightlyRetention({
    now: NOW,
    releases: [
      nightly('0.24.1-nightly.20260815.1', '2026-08-16T04:00:00Z'),
      nightly('0.24.1-nightly.20261022.2', '2026-10-23T04:00:00Z'),
    ],
  });
  assert.deepEqual(result.expired, ['v0.24.1-nightly.20260815.1']);
  assert.deepEqual(result.keep, ['v0.24.1-nightly.20261022.2']);
});

test('the newest published nightly is retained even when it is older than the window', () => {
  const result = selectNightlyRetention({
    now: NOW,
    releases: [
      nightly('0.24.1-nightly.20260815.1', '2026-08-16T04:00:00Z'),
      nightly('0.24.1-nightly.20260901.4', '2026-09-01T04:00:00Z'),
    ],
  });
  assert.deepEqual(result.expired, ['v0.24.1-nightly.20260815.1']);
  assert.deepEqual(result.keep, ['v0.24.1-nightly.20260901.4']);
});

test('drafts and stable releases are never candidates for retention', () => {
  const result = selectNightlyRetention({
    now: NOW,
    releases: [
      nightly('0.24.1-nightly.20260801.1', '2026-08-01T04:00:00Z', {
        draft: true,
      }),
      {
        tag_name: 'v0.23.0',
        draft: false,
        prerelease: false,
        published_at: '2026-01-01T00:00:00Z',
      },
      nightly('0.24.1-nightly.20261022.2', '2026-10-23T04:00:00Z'),
    ],
  });
  assert.deepEqual(result.expired, []);
  assert.deepEqual(result.keep, ['v0.24.1-nightly.20261022.2']);
});

test('a nightly without a parseable publish time fails safe and is kept', () => {
  const result = selectNightlyRetention({
    now: NOW,
    releases: [
      nightly('0.24.1-nightly.20260815.1', undefined),
      nightly('0.24.1-nightly.20261022.2', '2026-10-23T04:00:00Z'),
    ],
  });
  assert.deepEqual(result.expired, []);
  assert.equal(result.keep.length, 2);
});

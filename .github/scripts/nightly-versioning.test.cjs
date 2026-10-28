'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const {
  compareVersions,
  formatNightlyVersion,
  madridDateStamp,
  newestPublishedNightly,
  newestStableRelease,
  nightlyBaseline,
  nightlyTagName,
  parseVersion,
  publishedNightlies,
} = require('./nightly-versioning.cjs');

const SHA_A = 'a'.repeat(40);
const SHA_B = 'b'.repeat(40);

function nightlyRelease(
  version,
  { draft = false, prerelease = true, commit = SHA_A } = {},
) {
  return {
    tag_name: `v${version}`,
    draft,
    prerelease,
    target_commitish: commit,
    published_at: '2026-10-28T00:00:00Z',
  };
}

test('version parsing accepts stable and nightly shapes only', () => {
  assert.deepEqual(parseVersion('0.24.0'), { core: [0, 24, 0], nightly: null });
  assert.deepEqual(parseVersion('0.24.1-nightly.20261028.123'), {
    core: [0, 24, 1],
    nightly: { date: 20261028, run: 123 },
  });
  for (const rejected of [
    'v0.24.0',
    '0.24.1-beta.1',
    '0.24.1-nightly.2026102.1',
    '0.24',
    undefined,
  ]) {
    assert.equal(parseVersion(rejected), null, String(rejected));
  }
});

test('a stable release supersedes nightlies of the same base and every older version', () => {
  assert.equal(compareVersions('0.24.1-nightly.20261028.999', '0.24.1'), -1);
  assert.equal(compareVersions('0.24.0', '0.24.1-nightly.20261028.1'), -1);
  assert.equal(compareVersions('0.25.0', '0.24.1-nightly.20261028.1'), 1);
});

test('nightlies order by Madrid date then numeric run number, never lexically', () => {
  assert.equal(
    compareVersions('0.24.1-nightly.20261028.10', '0.24.1-nightly.20261028.9'),
    1,
  );
  assert.equal(
    compareVersions('0.24.1-nightly.20261029.1', '0.24.1-nightly.20261028.99'),
    1,
  );
  assert.equal(
    compareVersions('0.24.1-nightly.20261028.5', '0.24.1-nightly.20261028.5'),
    0,
  );
});

test('newest stable tag is chosen by numeric semver and ignores prereleases', () => {
  const newest = newestStableRelease([
    'v0.9.0',
    'v0.23.5',
    'v0.24.0',
    'v0.24.1-rc.1',
    'v0.24.1-nightly.20261028.1',
  ]);
  assert.deepEqual(newest, { tag: 'v0.24.0', version: '0.24.0' });
  assert.equal(newestStableRelease(['v0.24.1-nightly.20261028.1']), null);
});

test('nightly version uses the next patch base and rejects invalid inputs', () => {
  assert.equal(
    formatNightlyVersion({
      baseVersion: '0.24.0',
      dateStamp: '20261028',
      runNumber: 123,
    }),
    '0.24.1-nightly.20261028.123',
  );
  assert.equal(
    nightlyTagName('0.24.1-nightly.20261028.123'),
    'v0.24.1-nightly.20261028.123',
  );
  assert.throws(() =>
    formatNightlyVersion({
      baseVersion: '0.24.0',
      dateStamp: '2026-10-28',
      runNumber: 1,
    }),
  );
  assert.throws(() =>
    formatNightlyVersion({
      baseVersion: '0.24.0',
      dateStamp: '20261028',
      runNumber: 0,
    }),
  );
});

test('date stamps use Europe/Madrid calendar days across the UTC boundary', () => {
  // 23:30 UTC on 27 Oct 2026 is already 28 Oct in Madrid (CET, UTC+1).
  assert.equal(madridDateStamp(new Date('2026-10-27T23:30:00Z')), '20261028');
  // 22:30 UTC on 15 Jun 2026 is already 16 Jun in Madrid (CEST, UTC+2).
  assert.equal(madridDateStamp(new Date('2026-06-15T22:30:00Z')), '20260616');
});

test('only published, non-draft nightly prereleases count as nightlies', () => {
  const releases = [
    nightlyRelease('0.24.1-nightly.20261027.5'),
    nightlyRelease('0.24.1-nightly.20261028.9', { draft: true }),
    nightlyRelease('0.24.1-nightly.20261026.4', { prerelease: false }),
    {
      tag_name: 'v0.24.0',
      draft: false,
      prerelease: false,
      target_commitish: SHA_B,
    },
  ];
  assert.deepEqual(
    publishedNightlies(releases).map((entry) => entry.version),
    ['0.24.1-nightly.20261027.5'],
  );
});

test('baseline is the newest published nightly commit; drafts never advance it', () => {
  const releases = [
    nightlyRelease('0.24.1-nightly.20261027.5', { commit: SHA_A }),
    nightlyRelease('0.24.1-nightly.20261028.9', { draft: true, commit: SHA_B }),
  ];
  assert.deepEqual(nightlyBaseline({ releases, bootstrapCommit: SHA_B }), {
    commit: SHA_A,
    version: '0.24.1-nightly.20261027.5',
    source: 'published-nightly',
  });
  assert.equal(
    newestPublishedNightly(releases).version,
    '0.24.1-nightly.20261027.5',
  );
});

test('first run bootstraps the baseline from the newest stable tag commit', () => {
  assert.deepEqual(nightlyBaseline({ releases: [], bootstrapCommit: SHA_B }), {
    commit: SHA_B,
    version: null,
    source: 'bootstrap-stable-tag',
  });
  assert.throws(() =>
    nightlyBaseline({ releases: [], bootstrapCommit: 'not-a-sha' }),
  );
});

test('a published nightly without a full source commit fails closed', () => {
  const releases = [
    nightlyRelease('0.24.1-nightly.20261027.5', { commit: 'main' }),
  ];
  assert.throws(
    () => nightlyBaseline({ releases, bootstrapCommit: SHA_B }),
    /no full source commit/,
  );
});

import assert from 'node:assert/strict';
import test from 'node:test';
import {
  channelMetadataNameFor,
  isUpdateInfoForChannel,
  parseUpdateChannel,
  releaseTagsFromAtomFeed,
  releasesNewestFirst,
  releaseVersionFromTag,
} from './update-channel';

test('unknown or missing channel preferences fall back to stable', () => {
  assert.equal(parseUpdateChannel(undefined), 'stable');
  assert.equal(parseUpdateChannel('beta'), 'stable');
  assert.equal(parseUpdateChannel('nightly'), 'nightly');
});

test('stable channel excludes nightly and other prerelease tags', () => {
  const releases = releasesNewestFirst(
    ['v0.17.0', 'v0.17.1-nightly.20261028.42', 'v0.17.1-beta.1', 'v0.16.9'],
    'stable',
  );
  assert.deepEqual(
    releases.map((release) => release.version),
    ['0.17.0', '0.16.9'],
  );
});

test('nightly channel orders stable and nightly releases by semver, not feed order', () => {
  const releases = releasesNewestFirst(
    [
      'v0.17.1-nightly.20261027.40',
      'v0.17.0',
      'v0.17.1-nightly.20261028.42',
      'v0.16.9',
      'v0.17.1-beta.2',
    ],
    'nightly',
  );
  assert.deepEqual(
    releases.map((release) => release.version),
    [
      '0.17.1-nightly.20261028.42',
      '0.17.1-nightly.20261027.40',
      '0.17.0',
      '0.16.9',
    ],
  );
});

test('a stable release supersedes nightly builds of the same base version', () => {
  const [newest] = releasesNewestFirst(
    ['v0.17.1-nightly.20261028.42', 'v0.17.1'],
    'nightly',
  );
  assert.equal(newest.tag, 'v0.17.1');
});

test('tags that are not semver releases are never candidates', () => {
  assert.equal(releaseVersionFromTag('docs-2026-10-28'), null);
  assert.equal(releaseVersionFromTag('v1.2'), null);
  assert.equal(releaseVersionFromTag('v1.2.3'), '1.2.3');
});

test('nightly releases publish nightly metadata and stable releases keep latest metadata', () => {
  assert.equal(channelMetadataNameFor('0.17.1-nightly.20261028.42'), 'nightly');
  assert.equal(channelMetadataNameFor('0.17.1'), 'latest');
});

test('atom feed tags come from release links and are deduplicated', () => {
  const feed = [
    '<feed>',
    '<entry><link href="https://github.com/o/r/releases/tag/v0.17.1-nightly.20261028.42"/></entry>',
    '<entry><content>see <a href="/o/r/releases/tag/v0.17.0">notes</a></content></entry>',
    '<entry><link href="https://github.com/o/r/releases/tag/v0.17.1-nightly.20261028.42"/></entry>',
    '</feed>',
  ].join('');
  assert.deepEqual(releaseTagsFromAtomFeed(feed), [
    'v0.17.1-nightly.20261028.42',
    'v0.17.0',
  ]);
});

test('update results are accepted only for the channel that produced them', () => {
  assert.equal(
    isUpdateInfoForChannel(
      { version: '0.17.1', updateChannel: 'nightly' },
      'nightly',
    ),
    true,
  );
  assert.equal(
    isUpdateInfoForChannel(
      { version: '0.17.1', updateChannel: 'nightly' },
      'stable',
    ),
    false,
  );
  assert.equal(isUpdateInfoForChannel({ version: '0.17.1' }, 'stable'), false);
  assert.equal(isUpdateInfoForChannel(undefined, 'stable'), false);
});

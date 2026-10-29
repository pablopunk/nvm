'use strict';

const { compareVersions } = require('./nightly-versioning.cjs');

function selectNewestDev(releases, currentVersion) {
  if (!Array.isArray(releases)) throw new Error('Release list is unavailable');
  const eligible = releases.flatMap((release) => {
    const match = /^v(\d+\.\d+\.\d+)-dev$/.exec(release?.tag_name ?? '');
    if (!match || release.draft !== false || release.prerelease !== true)
      return [];
    return [{ tag: release.tag_name, version: match[1] }];
  });
  eligible.sort((a, b) => compareVersions(b.version, a.version));
  const newest = eligible[0];
  if (
    !newest ||
    (currentVersion && compareVersions(newest.version, currentVersion) <= 0)
  )
    return null;
  return newest;
}

function expiredDevTags(releases, now, protectedTags = []) {
  if (!Array.isArray(releases) || !Number.isFinite(now.getTime()))
    throw new Error('Invalid retention input');
  const newest = selectNewestDev(releases, null);
  const protectedSet = new Set([...protectedTags, newest?.tag]);
  const cutoff = now.getTime() - 30 * 86400000;
  return releases.flatMap((release) => {
    if (
      !/^v\d+\.\d+\.\d+-dev$/.test(release?.tag_name ?? '') ||
      release.draft !== false ||
      release.prerelease !== true ||
      /(?:^|\n)Nevermind-Preserve: true(?:\r?\n|$)/.test(release.body ?? '') ||
      protectedSet.has(release.tag_name)
    )
      return [];
    const published = Date.parse(release.published_at ?? '');
    return Number.isFinite(published) && published < cutoff
      ? [release.tag_name]
      : [];
  });
}

const PROMOTED_NIGHTLY_TAG = /^v(\d+\.\d+\.\d+)-nightly$/;

// Promoted nightlies hold independent asset copies, so only the newest build and the
// pointer's targets outlive the retention window.
function expiredPromotedNightlyTags(releases, now, protectedTags = []) {
  if (!Array.isArray(releases) || !Number.isFinite(now.getTime()))
    throw new Error('Invalid retention input');
  const published = releases.flatMap((release) => {
    const match = PROMOTED_NIGHTLY_TAG.exec(release?.tag_name ?? '');
    return match && release.draft === false && release.prerelease === true
      ? [{ release, version: match[1] }]
      : [];
  });
  published.sort((a, b) => compareVersions(b.version, a.version));
  const protectedSet = new Set([
    ...protectedTags,
    published[0]?.release.tag_name,
  ]);
  const cutoff = now.getTime() - 30 * 86400000;
  return published.flatMap(({ release }) => {
    if (
      /(?:^|\n)Nevermind-Preserve: true(?:\r?\n|$)/.test(release.body ?? '') ||
      protectedSet.has(release.tag_name)
    )
      return [];
    const publishedAt = Date.parse(release.published_at ?? '');
    return Number.isFinite(publishedAt) && publishedAt < cutoff
      ? [release.tag_name]
      : [];
  });
}

function referencedDevTags(releases) {
  if (!Array.isArray(releases)) throw new Error('Release list is unavailable');
  const referenced = new Set();
  for (const release of releases) {
    if (/^v\d+\.\d+\.\d+-dev$/.test(release?.tag_name ?? '')) continue;
    if (PROMOTED_NIGHTLY_TAG.test(release?.tag_name ?? '')) continue;
    if (typeof release?.body !== 'string') continue;
    for (const match of release.body.matchAll(/\bv\d+\.\d+\.\d+-dev\b/g))
      referenced.add(match[0]);
  }
  return [...referenced];
}

module.exports = {
  selectNewestDev,
  expiredDevTags,
  expiredPromotedNightlyTags,
  referencedDevTags,
};

if (require.main === module) {
  try {
    const [command, file, argument, pointerFile] = process.argv.slice(2);
    const releases = JSON.parse(require('node:fs').readFileSync(file, 'utf8'));
    if (command === 'select') {
      console.log(JSON.stringify(selectNewestDev(releases, argument || null)));
    } else if (command === 'retention' || command === 'nightly-retention') {
      const channel = command === 'retention' ? 'dev' : 'nightly';
      if (!pointerFile)
        throw new Error(`The ${channel} pointer is required for retention`);
      const pointer = JSON.parse(
        require('node:fs').readFileSync(pointerFile, 'utf8'),
      );
      if (
        pointer?.kind !== 'nevermind-channel-pointer' ||
        pointer.channel !== channel ||
        !Array.isArray(pointer.tags) ||
        pointer.tags.length < 1 ||
        pointer.tags.length > 2 ||
        pointer.tags.some(
          (tag) =>
            typeof tag !== 'string' ||
            !new RegExp(`^v\\d+\\.\\d+\\.\\d+-${channel}$`).test(tag),
        )
      )
        throw new Error(`Invalid ${channel} pointer`);
      const now = new Date(argument);
      console.log(
        JSON.stringify(
          channel === 'dev'
            ? expiredDevTags(releases, now, [
                ...referencedDevTags(releases),
                ...pointer.tags,
              ])
            : expiredPromotedNightlyTags(releases, now, pointer.tags),
        ),
      );
    } else {
      throw new Error(`Unknown command: ${command}`);
    }
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}

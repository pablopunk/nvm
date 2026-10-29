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

function referencedDevTags(releases) {
  if (!Array.isArray(releases)) throw new Error('Release list is unavailable');
  const referenced = new Set();
  for (const release of releases) {
    if (/^v\d+\.\d+\.\d+-dev$/.test(release?.tag_name ?? '')) continue;
    if (typeof release?.body !== 'string') continue;
    for (const match of release.body.matchAll(/\bv\d+\.\d+\.\d+-dev\b/g))
      referenced.add(match[0]);
  }
  return [...referenced];
}

module.exports = { selectNewestDev, expiredDevTags, referencedDevTags };

if (require.main === module) {
  try {
    const [command, file, argument] = process.argv.slice(2);
    const releases = JSON.parse(require('node:fs').readFileSync(file, 'utf8'));
    if (command === 'select') {
      console.log(JSON.stringify(selectNewestDev(releases, argument || null)));
    } else if (command === 'retention') {
      console.log(
        JSON.stringify(
          expiredDevTags(
            releases,
            new Date(argument),
            referencedDevTags(releases),
          ),
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

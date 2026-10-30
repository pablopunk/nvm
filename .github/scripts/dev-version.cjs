'use strict';

const { execFileSync } = require('node:child_process');
const { compareVersions } = require('./nightly-versioning.cjs');

function planDevVersion({ date, releases, tags, sourceSha, resolveTag }) {
  if (!/^[0-9a-f]{40}$/.test(sourceSha)) throw new Error('Invalid source SHA');
  if (!(date instanceof Date) || !Number.isFinite(date.getTime()))
    throw new Error('Invalid release date');
  const year = date.getUTCFullYear();
  if (year < 2000 || year > 9999) throw new Error('Invalid release year');
  const day = (date.getUTCMonth() + 1) * 100 + date.getUTCDate();
  const today = `${year}.${day}`;
  const versionPattern =
    /^v((?:[2-9]\d{3})\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))(?:-dev|-nightly)?$/;
  const versions = tags
    .map((tag) => versionPattern.exec(tag)?.[1])
    .filter(Boolean);
  const latestVersion = versions.sort(compareVersions).at(-1);
  if (latestVersion && compareVersions(latestVersion, `${today}.65535`) > 0)
    throw new Error('A release version is ahead of the current UTC date');

  const devReleases = releases
    .filter((release) => /^v\d+\.\d+\.\d+-dev$/.test(release.tag_name))
    .sort((left, right) =>
      compareVersions(right.tag_name.slice(1, -4), left.tag_name.slice(1, -4)),
    );
  const published = devReleases.find(
    (release) => !release.draft && release.prerelease,
  );
  if (published && resolveTag(published.tag_name) === sourceSha)
    return { version: published.tag_name.slice(1, -4), build: false };

  const draft = devReleases.find((release) => release.draft);
  if (
    draft &&
    draft.tag_name === `v${latestVersion}-dev` &&
    latestVersion.startsWith(`${today}.`) &&
    resolveTag(draft.tag_name) === sourceSha
  )
    return { version: latestVersion, build: true };

  const previous = latestVersion?.startsWith(`${today}.`)
    ? Number(latestVersion.split('.')[2])
    : 0;
  if (previous >= 65535) throw new Error('Daily Windows version limit reached');
  return { version: `${today}.${previous + 1}`, build: true };
}

if (require.main === module) {
  try {
    const sourceSha = process.argv[2];
    const repository = process.env.GITHUB_REPOSITORY;
    if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository ?? ''))
      throw new Error('Invalid GitHub repository');
    const pages = JSON.parse(
      execFileSync(
        'gh',
        [
          'api',
          '--paginate',
          '--slurp',
          '--jq',
          'map(.[] | {tag_name, draft, prerelease})',
          `repos/${repository}/releases?per_page=100`,
        ],
        { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 },
      ),
    );
    const tags = execFileSync('git', ['tag', '--list'], { encoding: 'utf8' })
      .trim()
      .split('\n')
      .filter(Boolean);
    const result = planDevVersion({
      date: new Date(),
      releases: pages,
      tags,
      sourceSha,
      resolveTag: (tag) =>
        execFileSync('git', ['rev-parse', `${tag}^{commit}`], {
          encoding: 'utf8',
        }).trim(),
    });
    console.log(JSON.stringify(result));
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}

module.exports = { planDevVersion };

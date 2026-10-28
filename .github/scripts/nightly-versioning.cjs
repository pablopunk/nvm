'use strict';

const MADRID_TIME_ZONE = 'Europe/Madrid';
const FULL_COMMIT_SHA_PATTERN = /^[0-9a-f]{40}$/;
const NIGHTLY_TAG_PREFIX = 'v';
const STABLE_TAG_PATTERN = /^v(\d+\.\d+\.\d+)$/;
const STABLE_VERSION_PATTERN = /^(\d+)\.(\d+)\.(\d+)$/;
const NIGHTLY_VERSION_PATTERN = /^(\d+)\.(\d+)\.(\d+)-nightly\.(\d{8})\.(\d+)$/;

function parseVersion(text) {
  if (typeof text !== 'string') return null;
  const nightly = NIGHTLY_VERSION_PATTERN.exec(text);
  if (nightly) {
    return {
      core: nightly.slice(1, 4).map(Number),
      nightly: { date: Number(nightly[4]), run: Number(nightly[5]) },
    };
  }
  const stable = STABLE_VERSION_PATTERN.exec(text);
  if (stable) return { core: stable.slice(1, 4).map(Number), nightly: null };
  return null;
}

function requireVersion(text) {
  const parsed = parseVersion(text);
  if (!parsed) throw new Error(`Unsupported release version: ${text}`);
  return parsed;
}

function compareNumbers(left, right) {
  if (left === right) return 0;
  return left < right ? -1 : 1;
}

// Semver-style ordering for the two shapes this pipeline emits: a stable
// X.Y.Z sorts above any X.Y.Z-nightly.* with the same core, and nightlies order
// by Europe/Madrid calendar date, then by the monotonically increasing run number.
// Build metadata is never emitted, so it never participates in ordering.
function compareVersions(leftText, rightText) {
  const left = requireVersion(leftText);
  const right = requireVersion(rightText);
  for (let index = 0; index < 3; index += 1) {
    if (left.core[index] !== right.core[index])
      return compareNumbers(left.core[index], right.core[index]);
  }
  if (!left.nightly && !right.nightly) return 0;
  if (!left.nightly) return 1;
  if (!right.nightly) return -1;
  return (
    compareNumbers(left.nightly.date, right.nightly.date) ||
    compareNumbers(left.nightly.run, right.nightly.run)
  );
}

function newestStableRelease(tagNames) {
  let newest = null;
  for (const tagName of tagNames) {
    const match = STABLE_TAG_PATTERN.exec(tagName);
    if (!match) continue;
    if (!newest || compareVersions(match[1], newest.version) > 0)
      newest = { tag: tagName, version: match[1] };
  }
  return newest;
}

function nextPatchBaseVersion(stableVersion) {
  const [major, minor, patch] = requireVersion(stableVersion).core;
  return `${major}.${minor}.${patch + 1}`;
}

function madridDateStamp(date) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: MADRID_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(date);
  const read = (type) => parts.find((part) => part.type === type).value;
  return `${read('year')}${read('month')}${read('day')}`;
}

function formatNightlyVersion({ baseVersion, dateStamp, runNumber }) {
  if (!/^\d{8}$/.test(dateStamp))
    throw new Error(`Invalid nightly date stamp: ${dateStamp}`);
  if (!Number.isInteger(runNumber) || runNumber < 1)
    throw new Error(`Invalid nightly run number: ${runNumber}`);
  const version = `${nextPatchBaseVersion(baseVersion)}-nightly.${dateStamp}.${runNumber}`;
  requireVersion(version);
  return version;
}

function nightlyTagName(version) {
  requireVersion(version);
  return `${NIGHTLY_TAG_PREFIX}${version}`;
}

function publishedNightlies(releases) {
  const nightlies = [];
  for (const release of releases) {
    if (!release || release.draft === true || release.prerelease !== true)
      continue;
    const tagName = release.tag_name;
    if (typeof tagName !== 'string' || !tagName.startsWith(NIGHTLY_TAG_PREFIX))
      continue;
    const version = tagName.slice(NIGHTLY_TAG_PREFIX.length);
    const parsed = parseVersion(version);
    if (!parsed?.nightly) continue;
    nightlies.push({ release, version });
  }
  return nightlies;
}

function newestPublishedNightly(releases) {
  let newest = null;
  for (const entry of publishedNightlies(releases)) {
    if (!newest || compareVersions(entry.version, newest.version) > 0)
      newest = entry;
  }
  return newest;
}

// The baseline is the source commit of the newest successfully published
// nightly. Drafts and failed runs never advance it. The first run, with no
// published nightly, bootstraps from the newest stable tag's commit.
function nightlyBaseline({ releases, bootstrapCommit }) {
  const newest = newestPublishedNightly(releases);
  if (!newest) {
    if (!FULL_COMMIT_SHA_PATTERN.test(bootstrapCommit ?? ''))
      throw new Error('Bootstrap baseline requires a full stable tag commit');
    return {
      commit: bootstrapCommit,
      version: null,
      source: 'bootstrap-stable-tag',
    };
  }
  const commit = newest.release.target_commitish;
  if (!FULL_COMMIT_SHA_PATTERN.test(commit ?? ''))
    throw new Error(
      `Published nightly ${newest.version} has no full source commit: ${commit}`,
    );
  return { commit, version: newest.version, source: 'published-nightly' };
}

module.exports = {
  compareVersions,
  formatNightlyVersion,
  madridDateStamp,
  newestPublishedNightly,
  newestStableRelease,
  nextPatchBaseVersion,
  nightlyBaseline,
  nightlyTagName,
  parseVersion,
  publishedNightlies,
};

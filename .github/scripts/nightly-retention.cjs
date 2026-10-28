'use strict';

const {
  compareVersions,
  publishedNightlies,
} = require('./nightly-versioning.cjs');

const DEFAULT_RETENTION_DAYS = 30;
const DAY_MS = 24 * 60 * 60 * 1000;

// The newest published nightly is always retained so a quiet or failed period
// leaves a usable release. Drafts are not public and are handled by publication.
function selectNightlyRetention({
  releases,
  now,
  retentionDays = DEFAULT_RETENTION_DAYS,
}) {
  const published = publishedNightlies(releases);
  const newest = published.reduce(
    (best, entry) =>
      !best || compareVersions(entry.version, best.version) > 0 ? entry : best,
    null,
  );
  const cutoff = now.getTime() - retentionDays * DAY_MS;
  const expired = [];
  const keep = [];
  for (const entry of published) {
    const tagName = entry.release.tag_name;
    const publishedAt = Date.parse(entry.release.published_at ?? '');
    const isNewest = entry === newest;
    // A missing or unparsable publish time fails safe: keep the release.
    if (isNewest || !Number.isFinite(publishedAt) || publishedAt >= cutoff) {
      keep.push(tagName);
      continue;
    }
    expired.push(tagName);
  }
  return { keep: keep.sort(), expired: expired.sort() };
}

module.exports = { DEFAULT_RETENTION_DAYS, selectNightlyRetention };

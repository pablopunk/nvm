'use strict';

const { compareVersions } = require('./nightly-versioning.cjs');

// A pushed stable tag publishes through the legacy path. It must not create a release
// that sorts below the current latest stable, which installed clients would never see.
function assertTagCanBeLatest(tagName, latestTagName) {
  const tag = /^v(\d+\.\d+\.\d+)$/.exec(tagName ?? '');
  if (!tag) throw new Error(`Unsupported stable tag: ${tagName}`);
  const latest = /^v(\d+\.\d+\.\d+)$/.exec(latestTagName ?? '');
  if (!latest)
    throw new Error(`Latest stable tag is unavailable: ${latestTagName}`);
  if (compareVersions(tag[1], latest[1]) < 0)
    throw new Error(
      `${tagName} is older than latest stable ${latestTagName}; promote a retained dev build instead`,
    );
}

module.exports = { assertTagCanBeLatest };

if (require.main === module) {
  try {
    const [tagName, latestTagName] = process.argv.slice(2);
    assertTagCanBeLatest(tagName, latestTagName);
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}

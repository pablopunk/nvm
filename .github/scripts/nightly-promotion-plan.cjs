'use strict';

const { compareVersions, parseVersion } = require('./nightly-versioning.cjs');
const { selectNewestDev } = require('./promotion-selection.cjs');

function planNightlyPromotion(releases) {
  if (!Array.isArray(releases)) throw new Error('Release list is unavailable');
  let current = null;
  let currentTag = null;
  for (const release of releases) {
    if (release?.draft !== false || release.prerelease !== true) continue;
    const tag = release.tag_name ?? '';
    const promoted = /^v(\d+\.\d+\.\d+)-nightly$/.exec(tag);
    const legacy = tag.startsWith('v') ? tag.slice(1) : '';
    const version =
      promoted?.[1] ?? (parseVersion(legacy)?.nightly ? legacy : null);
    if (version && (!current || compareVersions(version, current) > 0)) {
      current = version;
      currentTag = tag;
    }
  }
  return { current, currentTag, selected: selectNewestDev(releases, current) };
}

if (require.main === module) {
  try {
    const releases = JSON.parse(
      require('node:fs').readFileSync(process.argv[2], 'utf8'),
    );
    console.log(JSON.stringify(planNightlyPromotion(releases)));
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}
module.exports = { planNightlyPromotion };

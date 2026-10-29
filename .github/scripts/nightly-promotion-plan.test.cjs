'use strict';
const assert = require('node:assert/strict');
const { test } = require('node:test');
const { planNightlyPromotion } = require('./nightly-promotion-plan.cjs');
function release(tag_name) {
  return { tag_name, draft: false, prerelease: true };
}
test('selects newest dev after nightly and skips an unchanged build', () => {
  const releases = [
    release('v2026.1029.2-dev'),
    release('v2026.1029.1-nightly'),
  ];
  assert.equal(planNightlyPromotion(releases).selected.tag, 'v2026.1029.2-dev');
  releases.push(release('v2026.1029.2-nightly'));
  assert.equal(planNightlyPromotion(releases).selected, null);
});

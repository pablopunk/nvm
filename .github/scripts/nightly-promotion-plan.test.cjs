'use strict';
const assert = require('node:assert/strict');
const { test } = require('node:test');
const {
  planNightlyPromotion,
  createEligibilityProbe,
} = require('./nightly-promotion-plan.cjs');
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

test('falls back to the newest candidate whose actual CI gate and production identity pass', () => {
  const newer = 'v2026.1029.3-dev';
  const older = 'v2026.1029.2-dev';
  const newSha = 'a'.repeat(40);
  const oldSha = 'b'.repeat(40);
  function gh(endpoint) {
    if (endpoint.endsWith(newer)) return { sha: newSha };
    if (endpoint.endsWith(older)) return { sha: oldSha };
    const sha = endpoint.includes(newSha) ? newSha : oldSha;
    return {
      workflow_runs: [
        {
          head_sha: sha,
          event: 'push',
          run_number: 1,
          id: 1,
          status: 'completed',
          conclusion: sha === newSha ? 'failure' : 'success',
        },
      ],
    };
  }
  const probe = createEligibilityProbe({
    repository: 'pablopunk/nvm',
    healthFile: 'health.json',
    gh,
    identity: () => ({ status: 'verified' }),
  });
  assert.equal(
    planNightlyPromotion([release(newer), release(older)], probe).selected.tag,
    older,
  );
});

test('falls back for a confirmed older production commit but fails closed on unknown identity', () => {
  const newer = 'v2026.1029.3-dev';
  const older = 'v2026.1029.2-dev';
  const gh = (endpoint) =>
    endpoint.includes('/commits/')
      ? { sha: (endpoint.endsWith(newer) ? 'a' : 'b').repeat(40) }
      : {
          workflow_runs: [
            {
              head_sha: endpoint.includes('a'.repeat(40))
                ? 'a'.repeat(40)
                : 'b'.repeat(40),
              event: 'push',
              run_number: 1,
              id: 1,
              status: 'completed',
              conclusion: 'success',
            },
          ],
        };
  const identity = (sha) =>
    sha.startsWith('a')
      ? {
          status: 'rejected',
          reason:
            'production is older, rolled back, or unrelated (compare status behind)',
        }
      : { status: 'verified' };
  const probe = createEligibilityProbe({
    repository: 'pablopunk/nvm',
    healthFile: 'health.json',
    gh,
    identity,
  });
  assert.equal(
    planNightlyPromotion([release(newer), release(older)], probe).selected.tag,
    older,
  );
  const unknown = createEligibilityProbe({
    repository: 'pablopunk/nvm',
    healthFile: 'health.json',
    gh,
    identity: () => ({ status: 'rejected', reason: 'compare status null' }),
  });
  assert.throws(
    () => planNightlyPromotion([release(newer), release(older)], unknown),
    /Cannot verify production identity/,
  );
});

test('does not select ineligible, unpublished or older releases, and propagates API failure', () => {
  const releases = [
    null,
    release('v2026.1029.1-dev'),
    release('v2026.1029.4-dev'),
    { ...release('v2026.1029.5-dev'), draft: true },
    release('v2026.1029.2-nightly'),
    release('v2026.1029.3-dev'),
  ];
  const visited = [];
  const result = planNightlyPromotion(releases, (candidate) => {
    visited.push(candidate.tag);
    return false;
  });
  assert.deepEqual(visited, ['v2026.1029.4-dev', 'v2026.1029.3-dev']);
  assert.equal(result.selected, null);
  const probe = createEligibilityProbe({
    repository: 'pablopunk/nvm',
    healthFile: 'health.json',
    gh: () => {
      throw new Error('rate limited');
    },
    identity: () => ({ status: 'verified' }),
  });
  assert.throws(() => planNightlyPromotion(releases, probe), /rate limited/);
});

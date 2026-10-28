import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import { backendCommitSha, backendShortVersion, isFullCommitSha } from './deployment-identity';

const DEPLOYED_COMMIT = '0123456789abcdef0123456789abcdef01234567';

function clearPlatformCommit() {
  delete process.env.VERCEL_GIT_COMMIT_SHA;
}

beforeEach(clearPlatformCommit);
afterEach(clearPlatformCommit);

test('accepts only full lowercase commit SHAs', () => {
  assert.equal(isFullCommitSha(DEPLOYED_COMMIT), true);
  assert.equal(isFullCommitSha(DEPLOYED_COMMIT.slice(0, 7)), false);
  assert.equal(isFullCommitSha(DEPLOYED_COMMIT.toUpperCase()), false);
  assert.equal(isFullCommitSha(`${DEPLOYED_COMMIT} `), false);
  assert.equal(isFullCommitSha(`${DEPLOYED_COMMIT}0`), false);
  assert.equal(isFullCommitSha('dev'), false);
  assert.equal(isFullCommitSha(null), false);
  assert.equal(isFullCommitSha(undefined), false);
});

test('reports no commit identity outside a deployment', () => {
  assert.equal(backendCommitSha(), null);
  assert.equal(backendShortVersion(), 'dev');
});

test('reports the full deployed commit and its seven-character version', () => {
  process.env.VERCEL_GIT_COMMIT_SHA = DEPLOYED_COMMIT;
  assert.equal(backendCommitSha(), DEPLOYED_COMMIT);
  assert.equal(backendShortVersion(), '0123456');
});

test('does not report abbreviated or malformed platform SHAs as identity', () => {
  process.env.VERCEL_GIT_COMMIT_SHA = '0123456';
  assert.equal(backendCommitSha(), null);
  assert.equal(backendShortVersion(), '0123456');

  process.env.VERCEL_GIT_COMMIT_SHA = DEPLOYED_COMMIT.toUpperCase();
  assert.equal(backendCommitSha(), null);
  assert.equal(backendShortVersion(), '0123456');
});

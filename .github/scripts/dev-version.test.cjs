'use strict';

const assert = require('node:assert/strict');
const { test } = require('node:test');
const { isBuildRelevantPath, planDevVersion } = require('./dev-version.cjs');

const sha = 'a'.repeat(40);
const otherSha = 'b'.repeat(40);

function plan(overrides = {}) {
  return planDevVersion({
    date: new Date('2026-10-31T12:00:00Z'),
    releases: [],
    tags: [],
    sourceSha: sha,
    resolveTag: () => otherSha,
    ...overrides,
  });
}

test('allocates daily build numbers and resets on a new UTC day', () => {
  assert.deepEqual(plan({ tags: ['v2026.1030.911-dev'] }), {
    version: '2026.1031.1',
    build: true,
  });
  assert.deepEqual(
    plan({ tags: ['v2026.1031.1-dev', 'v2026.1031.2-nightly'] }),
    { version: '2026.1031.3', build: true },
  );
  assert.deepEqual(plan({ date: new Date('2027-01-05T00:00:00Z') }), {
    version: '2027.105.1',
    build: true,
  });
});

test('preserves ordering on the transition day and rejects a future version', () => {
  assert.equal(
    plan({
      date: new Date('2026-10-30T23:00:00Z'),
      tags: ['v2026.1030.911-dev'],
    }).version,
    '2026.1030.912',
  );
  assert.throws(() => plan({ tags: ['v2026.1101.1-dev'] }), /ahead/);
  assert.throws(() => plan({ tags: ['v2026.1031.65535-dev'] }), /limit/);
});

test('repairs an already published build for the same source without rebuilding', () => {
  const release = {
    tag_name: 'v2026.1030.911-dev',
    draft: false,
    prerelease: true,
  };
  assert.deepEqual(
    plan({
      tags: [release.tag_name],
      releases: [release],
      resolveTag: () => sha,
    }),
    { version: '2026.1030.911', build: false },
  );
});

test('skips non-package changes since the last published build', () => {
  const release = {
    tag_name: 'v2026.1030.911-dev',
    draft: false,
    prerelease: true,
  };
  assert.equal(
    plan({ releases: [release], hasRelevantChanges: () => false }).build,
    false,
  );
  assert.equal(
    plan({ releases: [release], hasRelevantChanges: () => true }).build,
    true,
  );
  for (const path of [
    'README.md',
    'AGENTS.md',
    '.agents/skills/a.md',
    'marketing/index.html',
    'src/backend/package.json',
    'src/backend/src/pages/index.astro',
  ])
    assert.equal(isBuildRelevantPath(path), false, path);
  for (const path of [
    'docs/guide.md',
    'THIRD_PARTY_NOTICES.md',
    'src/app/main.ts',
    'electron-builder.yml',
  ])
    assert.equal(isBuildRelevantPath(path), true, path);
});

test('reuses a matching draft but never reuses another source or an older draft', () => {
  const draft = { tag_name: 'v2026.1031.1-dev', draft: true, prerelease: true };
  assert.deepEqual(
    plan({ tags: [draft.tag_name], releases: [draft], resolveTag: () => sha }),
    { version: '2026.1031.1', build: true },
  );
  assert.equal(
    plan({ tags: [draft.tag_name], releases: [draft] }).version,
    '2026.1031.2',
  );
  assert.equal(
    plan({
      tags: [draft.tag_name, 'v2026.1031.2-dev'],
      releases: [draft],
      resolveTag: () => sha,
    }).version,
    '2026.1031.3',
  );
});

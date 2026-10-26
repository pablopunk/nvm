const assert = require('node:assert/strict');
const test = require('node:test');
const {
  candidateBody,
  exactCandidateMarker,
  mergeGeneratedFileWithCurrentBase,
  parseCandidateMarker,
  parseStateMarker,
  setGeneratedCommit,
} = require('./publish-pi-update.cjs');

const versions = {
  '@earendil-works/pi-ai': '1.4.0',
  '@earendil-works/pi-coding-agent': '1.4.0',
};

test('clean base changes merge with the bot candidate and conflicting edits require attention', async function semanticMergeCheck() {
  const originalBase = [
    'dependencies:',
    '  pi: 0.80.10',
    '  other: 1.0.0',
    '',
  ].join('\n');
  const generated = originalBase.replace('pi: 0.80.10', 'pi: 1.4.0');
  const currentBase = originalBase.replace('other: 1.0.0', 'other: 2.0.0');
  assert.equal(
    await mergeGeneratedFileWithCurrentBase(
      generated,
      originalBase,
      currentBase,
    ),
    generated.replace('other: 1.0.0', 'other: 2.0.0'),
  );

  const humanEdit = currentBase.replace('pi: 0.80.10', 'pi: 9.9.9');
  assert.equal(
    await mergeGeneratedFileWithCurrentBase(generated, originalBase, humanEdit),
    null,
  );
});

test('PR body records an exact rejected tuple and generated commit fingerprint', function machineReadableMarkers() {
  const fingerprint = 'a'.repeat(64);
  const generatedCommitSha = 'b'.repeat(40);
  const candidate = {
    metadata: {
      candidateVersions: versions,
      baseVersions: {
        '@earendil-works/pi-ai': '0.80.10',
        '@earendil-works/pi-coding-agent': '0.80.10',
      },
      baseSha: 'c'.repeat(40),
      transitiveDelta: [],
    },
    fingerprint,
    transitiveDelta: [],
  };
  const body = setGeneratedCommit(
    candidateBody(candidate, generatedCommitSha),
    generatedCommitSha,
  );
  assert.equal(body.includes(exactCandidateMarker(versions)), true);
  assert.deepEqual(
    parseCandidateMarker(exactCandidateMarker(versions)),
    versions,
  );
  assert.throws(() => parseCandidateMarker('<!-- edited marker -->'));
  const state = parseStateMarker(body);
  assert.deepEqual(state, {
    baseSha: 'c'.repeat(40),
    fingerprint,
    candidateMarker: exactCandidateMarker(versions),
    generatedCommitSha,
  });
  assert.match(body, /Review classification: routine update/);
  assert.match(body, /Generated commit: b{40}/);
  assert.match(body, /Required CI: pending/);
});

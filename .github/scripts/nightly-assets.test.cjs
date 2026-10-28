'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const {
  expectedNightlyAssetNames,
  parseUpdaterMetadata,
  validateNightlyAssetSet,
  verifyUpdaterMetadata,
} = require('./nightly-assets.cjs');

const VERSION = '0.24.1-nightly.20261028.123';
const SETUP = `Nevermind-${VERSION}-win-x64-setup.exe`;

function windowsMetadata({
  version = VERSION,
  path = SETUP,
  sha512 = 'AAAA',
} = {}) {
  return [
    `version: ${version}`,
    'files:',
    `  - url: ${SETUP}`,
    `    sha512: ${sha512}`,
    '    size: 10',
    `  - url: ${SETUP}.blockmap`,
    '    sha512: BBBB',
    '    size: 5',
    `path: ${path}`,
    `sha512: ${sha512}`,
    "releaseDate: '2026-10-28T04:00:00.000Z'",
    '',
  ].join('\n');
}

test('expected asset set mirrors stable packaging with channel-named updater metadata', () => {
  const names = expectedNightlyAssetNames(VERSION);
  assert.equal(names.length, 17);
  for (const metadata of [
    'nightly.yml',
    'nightly-mac.yml',
    'nightly-linux.yml',
    'nightly-linux-arm64.yml',
  ]) {
    assert.ok(names.includes(metadata), metadata);
  }
  assert.equal(
    names.some((name) => /(^|[-/])latest/.test(name)),
    false,
    'nightlies never publish latest*.yml metadata',
  );
});

test('publication requires the exact expected asset set', () => {
  const expected = expectedNightlyAssetNames(VERSION);
  assert.deepEqual(validateNightlyAssetSet(expected, VERSION), expected);
  assert.throws(
    () => validateNightlyAssetSet(expected.slice(1), VERSION),
    /Missing: \[Nevermind-/,
  );
  assert.throws(
    () => validateNightlyAssetSet([...expected, 'latest.yml'], VERSION),
    /Unexpected: \[latest\.yml\]/,
  );
  assert.throws(
    () => validateNightlyAssetSet([...expected, expected[0]], VERSION),
    /Duplicate release assets/,
  );
});

test('updater metadata parser reads the top-level path and every listed file', () => {
  const metadata = parseUpdaterMetadata(windowsMetadata());
  assert.equal(metadata.version, VERSION);
  assert.equal(metadata.path, SETUP);
  assert.deepEqual(
    metadata.files.map((file) => file.url),
    [SETUP, `${SETUP}.blockmap`],
  );
});

test('updater metadata verifies digests of staged files and rejects any mismatch', () => {
  const assetNames = expectedNightlyAssetNames(VERSION);
  const digests = { [SETUP]: 'AAAA', [`${SETUP}.blockmap`]: 'BBBB' };
  const verify = (text, overrides = {}) =>
    verifyUpdaterMetadata({
      name: 'nightly.yml',
      text,
      version: VERSION,
      assetNames,
      sha512Of: (name) => digests[name] ?? null,
      ...overrides,
    });
  assert.equal(verify(windowsMetadata()).version, VERSION);
  assert.throws(
    () => verify(windowsMetadata({ version: '0.24.1-nightly.20261028.124' })),
    /does not match/,
  );
  assert.throws(
    () => verify(windowsMetadata({ path: 'other.exe' })),
    /is not a listed file/,
  );
  assert.throws(
    () => verify(windowsMetadata(), { sha512Of: () => 'ZZZZ' }),
    /does not match the staged file/,
  );
  assert.throws(
    () =>
      verify(windowsMetadata(), {
        sha512Of: (name) => (name === SETUP ? 'AAAA' : null),
      }),
    /references missing/,
  );
});

test('updater metadata must reference only release assets', () => {
  const assetNames = expectedNightlyAssetNames(VERSION).filter(
    (name) => name !== `${SETUP}.blockmap`,
  );
  assert.throws(
    () =>
      verifyUpdaterMetadata({
        name: 'nightly.yml',
        text: windowsMetadata(),
        version: VERSION,
        assetNames,
        sha512Of: () => 'AAAA',
      }),
    /is not a release asset/,
  );
});

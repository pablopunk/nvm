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
const PRIMARY = `Nevermind-${VERSION}-linux-x86_64.AppImage`;

function updaterMetadata({
  version = VERSION,
  path = PRIMARY,
  sha512 = 'AAAA',
} = {}) {
  return [
    `version: ${version}`,
    'files:',
    `  - url: ${PRIMARY}`,
    `    sha512: ${sha512}`,
    '    size: 10',
    `  - url: ${PRIMARY}.blockmap`,
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
  assert.equal(names.length, 13);
  assert.equal(
    names.filter((name) => name.includes('-win-')).join(),
    `Nevermind-${VERSION}-win-x64-portable.exe`,
    'Windows ships only the portable executable',
  );
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
  const metadata = parseUpdaterMetadata(updaterMetadata());
  assert.equal(metadata.version, VERSION);
  assert.equal(metadata.path, PRIMARY);
  assert.deepEqual(
    metadata.files.map((file) => file.url),
    [PRIMARY, `${PRIMARY}.blockmap`],
  );
});

test('updater metadata verifies digests of staged files and rejects any mismatch', () => {
  const assetNames = expectedNightlyAssetNames(VERSION);
  const digests = { [PRIMARY]: 'AAAA', [`${PRIMARY}.blockmap`]: 'BBBB' };
  const verify = (text, overrides = {}) =>
    verifyUpdaterMetadata({
      name: 'nightly.yml',
      text,
      version: VERSION,
      assetNames,
      sha512Of: (name) => digests[name] ?? null,
      ...overrides,
    });
  assert.equal(verify(updaterMetadata()).version, VERSION);
  assert.throws(
    () => verify(updaterMetadata({ version: '0.24.1-nightly.20261028.124' })),
    /does not match/,
  );
  assert.throws(
    () => verify(updaterMetadata({ path: 'other.AppImage' })),
    /is not a listed file/,
  );
  assert.throws(
    () => verify(updaterMetadata(), { sha512Of: () => 'ZZZZ' }),
    /does not match the staged file/,
  );
  assert.throws(
    () =>
      verify(updaterMetadata(), {
        sha512Of: (name) => (name === PRIMARY ? 'AAAA' : null),
      }),
    /references missing/,
  );
});

test('updater metadata must reference only release assets', () => {
  const assetNames = expectedNightlyAssetNames(VERSION).filter(
    (name) => name !== `${PRIMARY}.blockmap`,
  );
  assert.throws(
    () =>
      verifyUpdaterMetadata({
        name: 'nightly.yml',
        text: updaterMetadata(),
        version: VERSION,
        assetNames,
        sha512Of: () => 'AAAA',
      }),
    /is not a release asset/,
  );
});

test('updater metadata size and top-level digest must match the staged primary file', () => {
  const assetNames = expectedNightlyAssetNames(VERSION);
  const digests = { [PRIMARY]: 'AAAA', [`${PRIMARY}.blockmap`]: 'BBBB' };
  const sizes = { [PRIMARY]: 10, [`${PRIMARY}.blockmap`]: 5 };
  const verify = (text, overrides = {}) =>
    verifyUpdaterMetadata({
      name: 'nightly.yml',
      text,
      version: VERSION,
      assetNames,
      sha512Of: (name) => digests[name] ?? null,
      sizeOf: (name) => sizes[name],
      ...overrides,
    });
  assert.equal(verify(updaterMetadata()).version, VERSION);
  assert.throws(
    () => verify(updaterMetadata(), { sizeOf: () => 11 }),
    /size for .* does not match/,
  );
  assert.throws(
    () => verify(updaterMetadata().replace(/^sha512: AAAA$/m, 'sha512: CCCC')),
    /top-level sha512 does not match/,
  );
});

test('macOS updater metadata must target the arm64 ZIP only', () => {
  const zip = `Nevermind-${VERSION}-mac-arm64.zip`;
  const metadata = `version: ${VERSION}\nfiles:\n  - url: ${zip}\n    sha512: AAAA\n    size: 10\npath: ${zip}\nsha512: AAAA\n`;
  const verify = (text) =>
    verifyUpdaterMetadata({
      name: 'nightly-mac.yml',
      text,
      version: VERSION,
      assetNames: [zip],
      sha512Of: () => 'AAAA',
      sizeOf: () => 10,
    });
  assert.equal(verify(metadata).path, zip);
  assert.throws(
    () => verify(metadata.replaceAll(zip, 'other.zip')),
    /release asset/,
  );
  assert.throws(
    () =>
      verify(
        metadata.replace(
          `files:\n`,
          `files:\n  - url: ${zip}\n    sha512: AAAA\n    size: 10\n`,
        ),
      ),
    /must target only/,
  );
});

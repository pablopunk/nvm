'use strict';

const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const {
  validateWindowsUpdaterMetadata,
} = require('../../scripts/validate-windows-updater-metadata.cjs');

function fixture(overrides = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'windows-updater-'));
  const version = '0.13.2';
  const arch = overrides.arch || 'x64';
  const setupName = `Nevermind-${version}-win-${arch}-setup.exe`;
  const setupPath = path.join(root, setupName);
  const bytes = Buffer.from('unsigned setup fixture');
  fs.writeFileSync(setupPath, bytes);
  if (!overrides.missingBlockmap) {
    fs.writeFileSync(`${setupPath}.blockmap`, 'blockmap fixture');
  }
  const hash = crypto.createHash('sha512').update(bytes).digest('base64');
  const metadataPath = path.join(root, 'latest.yml');
  const url = overrides.portable
    ? `Nevermind-${version}-win-${arch}-portable.exe`
    : setupName;
  fs.writeFileSync(
    metadataPath,
    [
      `version: ${version}`,
      'files:',
      `  - url: ${url}`,
      `    sha512: ${overrides.wrongHash ? 'wrong' : hash}`,
      `    size: ${overrides.wrongSize ? bytes.length + 1 : bytes.length}`,
      `path: ${url}`,
      `sha512: ${overrides.wrongHash ? 'wrong' : hash}`,
    ].join('\n'),
  );
  return { arch, metadataPath, root, version };
}

function withFixture(overrides, assertion) {
  const created = fixture(overrides);
  try {
    assertion(created);
  } finally {
    fs.rmSync(created.root, { force: true, recursive: true });
  }
}

test('accepts one correctly hashed x64 NSIS artifact and blockmap', () => {
  withFixture({}, ({ metadataPath, root, version, arch }) => {
    assert.doesNotThrow(() =>
      validateWindowsUpdaterMetadata(metadataPath, root, version, arch),
    );
  });
});

for (const [name, overrides] of Object.entries({
  'wrong hash': { wrongHash: true },
  'wrong size': { wrongSize: true },
  'wrong architecture': { arch: 'arm64' },
  'missing blockmap': { missingBlockmap: true },
  'portable linked': { portable: true },
})) {
  test(`rejects ${name}`, () => {
    withFixture(overrides, ({ metadataPath, root, version }) => {
      assert.throws(() =>
        validateWindowsUpdaterMetadata(metadataPath, root, version, 'x64'),
      );
    });
  });
}

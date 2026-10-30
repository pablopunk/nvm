'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const {
  parseMetadata,
  portableArtifactName,
  validateWindowsUpdaterMetadata,
  writeWindowsPortableUpdaterMetadata,
} = require('../../scripts/validate-windows-updater-metadata.cjs');
const { verifyUpdaterMetadata } = require('./nightly-assets.cjs');
const { stampMetadata } = require('./stamp-state-compatibility.cjs');

const VERSION = '2026.1030.2';

function fixture(overrides = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'windows-updater-'));
  const arch = overrides.arch || 'x64';
  const portableName =
    overrides.artifactName || portableArtifactName(VERSION, arch);
  const portablePath = path.join(root, portableName);
  fs.writeFileSync(portablePath, Buffer.from('unsigned portable fixture'));
  const metadataPath = path.join(root, 'nightly.yml');
  writeWindowsPortableUpdaterMetadata(
    portablePath,
    metadataPath,
    VERSION,
    '2026-10-30T00:00:00.000Z',
  );
  if (overrides.edit)
    fs.writeFileSync(
      metadataPath,
      overrides.edit(fs.readFileSync(metadataPath, 'utf8')),
    );
  if (overrides.tamperArtifact) fs.appendFileSync(portablePath, 'tampered');
  return { metadataPath, portableName, portablePath, root };
}

function withFixture(overrides, assertion) {
  const created = fixture(overrides);
  try {
    assertion(created);
  } finally {
    fs.rmSync(created.root, { force: true, recursive: true });
  }
}

test('generated metadata names exactly the hashed portable executable', () => {
  withFixture({}, ({ metadataPath, portableName, root }) => {
    const metadata = parseMetadata(fs.readFileSync(metadataPath, 'utf8'));
    assert.equal(metadata.version, VERSION);
    assert.equal(metadata.path, portableName);
    assert.deepEqual(
      metadata.files.map((file) => file.url),
      [portableName],
    );
    assert.doesNotThrow(() =>
      validateWindowsUpdaterMetadata(metadataPath, root, VERSION, 'x64'),
    );
  });
});

test('stamped portable metadata passes the promotion asset verifier', () => {
  withFixture({}, ({ metadataPath, portableName, portablePath }) => {
    stampMetadata(metadataPath);
    const text = fs.readFileSync(metadataPath, 'utf8');
    assert.match(text, /^nevermindStateEpoch: \d+$/m);
    const digest = require('node:crypto')
      .createHash('sha512')
      .update(fs.readFileSync(portablePath))
      .digest('base64');
    assert.doesNotThrow(() =>
      verifyUpdaterMetadata({
        name: 'nightly.yml',
        text,
        version: VERSION,
        sha512Of: (name) => (name === portableName ? digest : null),
        sizeOf: () => fs.statSync(portablePath).size,
        assetNames: [portableName, 'nightly.yml'],
      }),
    );
  });
});

for (const [name, overrides] of Object.entries({
  'a tampered executable': { tamperArtifact: true },
  'a wrong size': {
    edit: (text) =>
      text.replace(/size: (\d+)/, (_, size) => `size: ${Number(size) + 1}`),
  },
  'a wrong version': { edit: (text) => text.replace(VERSION, '2026.1030.1') },
  'a wrong architecture': { arch: 'arm64' },
  'an NSIS installer': {
    artifactName: `Nevermind-${VERSION}-win-x64-setup.exe`,
  },
  'an extra file entry': {
    edit: (text) =>
      text.replace(
        'path:',
        '  - url: extra.exe\n    sha512: x\n    size: 1\npath:',
      ),
  },
})) {
  test(`rejects ${name}`, () => {
    withFixture(overrides, ({ metadataPath, root }) => {
      assert.throws(() =>
        validateWindowsUpdaterMetadata(metadataPath, root, VERSION, 'x64'),
      );
    });
  });
}

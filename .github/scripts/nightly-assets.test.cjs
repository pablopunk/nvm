'use strict';

const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const test = require('node:test');
const {
  NIGHTLY_UPDATER_METADATA,
  checksumsFor,
  expectedNightlyAssetNames,
  isReleaseAsset,
  retargetUpdaterMetadata,
  validateNightlyAssetSet,
  verifyChecksums,
  verifyUpdaterMetadata,
} = require('./nightly-assets.cjs');

const VERSION = '2026.1117.3';
const REPOSITORY = 'pablopunk/nvm';
const download = (tag, name) =>
  `https://github.com/${REPOSITORY}/releases/download/${tag}/${name}`;

function signingKeys() {
  const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
  const raw = publicKey.export({ format: 'der', type: 'spki' }).subarray(12);
  return { publicKey: raw.toString('base64'), privateKey };
}

// A staged release: each asset's bytes and a manifest per target, signed as `mygo build` signs them.
function stagedRelease({ tag, privateKey, deltas = [] }) {
  const files = new Map();
  for (const name of expectedNightlyAssetNames(VERSION))
    if (!NIGHTLY_UPDATER_METADATA.includes(name) && name !== 'SHA256SUMS')
      files.set(name, Buffer.from(`bytes:${name}`));
  for (const delta of deltas) files.set(delta, Buffer.from(`bytes:${delta}`));
  const digest = (name) => crypto.createHash('sha256').update(files.get(name)).digest();
  const sign = (name) => crypto.sign(null, digest(name), privateKey).toString('base64');
  for (const metadata of NIGHTLY_UPDATER_METADATA) {
    const target = /^update-(.+)\.json$/.exec(metadata)[1];
    const archive = `nevermind-next-${VERSION}-${target}.tar.gz`;
    const manifest = {
      version: VERSION,
      url: download(tag, archive),
      size: files.get(archive).length,
      signature: sign(archive),
      deltas: deltas
        .filter((delta) => delta.endsWith(`-${target}.delta`))
        .map((delta) => ({
          from: /^nevermind-next-(.+?)-to-/.exec(delta)[1],
          url: download(tag, delta),
          size: files.get(delta).length,
          signature: sign(delta),
        })),
    };
    files.set(metadata, Buffer.from(JSON.stringify(manifest)));
  }
  const hex = (name) => crypto.createHash('sha256').update(files.get(name)).digest('hex');
  files.set('SHA256SUMS', Buffer.from(checksumsFor([...files.keys()], hex)));
  return files;
}

function verify(files, { tag, publicKey, name = 'update-darwin-arm64.json' }) {
  return verifyUpdaterMetadata({
    name,
    text: files.get(name).toString('utf8'),
    version: VERSION,
    repository: REPOSITORY,
    tag,
    assetNames: [...files.keys()],
    sha256Of: (asset) => crypto.createHash('sha256').update(files.get(asset)).digest(),
    sizeOf: (asset) => files.get(asset).length,
    publicKey,
  });
}

test('the release holds exactly the Go packages, with or without deltas', () => {
  const names = expectedNightlyAssetNames(VERSION);
  assert.deepEqual(validateNightlyAssetSet(names, VERSION), names);
  validateNightlyAssetSet(
    [...names, `nevermind-next-2026.1110.1-to-${VERSION}-linux-amd64.delta`],
    VERSION,
  );
  assert.throws(
    () => validateNightlyAssetSet(names.filter((name) => name !== 'update-linux-arm64.json'), VERSION),
    /Missing: \[update-linux-arm64\.json\]/,
  );
  assert.throws(
    () => validateNightlyAssetSet([...names, 'nevermind-next-fast.zip'], VERSION),
    /Unexpected: \[nevermind-next-fast\.zip\]/,
  );
  assert.throws(
    () => validateNightlyAssetSet([...names, `nevermind-next-2026.1110.1-to-2026.1117.2-linux-amd64.delta`], VERSION),
    /Unexpected/,
  );
});

test('a manifest must name its release, its archive and signed files', () => {
  const { publicKey, privateKey } = signingKeys();
  const tag = `v${VERSION}-dev`;
  const delta = `nevermind-next-2026.1110.1-to-${VERSION}-darwin-arm64.delta`;
  const files = stagedRelease({ tag, privateKey, deltas: [delta] });
  verify(files, { tag, publicKey });
  assert.throws(() => verify(files, { tag: `v${VERSION}-nightly`, publicKey }), /is not in the release/);
  const other = signingKeys();
  assert.throws(() => verify(files, { tag, publicKey: other.publicKey }), /not signed with the update key/);
  const changed = JSON.parse(files.get('update-darwin-arm64.json'));
  changed.size += 1;
  files.set('update-darwin-arm64.json', Buffer.from(JSON.stringify(changed)));
  assert.throws(() => verify(files, { tag, publicKey }), /size/);
});

test('promotion retargets the manifests and keeps their signatures valid', () => {
  const { publicKey, privateKey } = signingKeys();
  const delta = `nevermind-next-2026.1110.1-to-${VERSION}-linux-amd64.delta`;
  const files = stagedRelease({ tag: `v${VERSION}`, privateKey, deltas: [delta] });
  for (const tag of [`v${VERSION}-dev`, `v${VERSION}-nightly`, `v${VERSION}`]) {
    for (const metadata of NIGHTLY_UPDATER_METADATA) {
      const text = retargetUpdaterMetadata({
        text: files.get(metadata).toString('utf8'),
        repository: REPOSITORY,
        version: VERSION,
        tag,
      });
      files.set(metadata, Buffer.from(text));
      verify(files, { tag, publicKey, name: metadata });
    }
  }
  assert.throws(
    () =>
      retargetUpdaterMetadata({
        text: files.get('update-linux-amd64.json').toString('utf8'),
        repository: REPOSITORY,
        version: VERSION,
        tag: 'v2026.1117.4-nightly',
      }),
    /is not a release of/,
  );
});

test('SHA256SUMS lists every asset but the retargeted manifests', () => {
  const { privateKey } = signingKeys();
  const files = stagedRelease({ tag: `v${VERSION}-dev`, privateKey });
  const hex = (name) => crypto.createHash('sha256').update(files.get(name)).digest('hex');
  const text = files.get('SHA256SUMS').toString('utf8');
  verifyChecksums({ text, names: [...files.keys()], sha256Hex: hex });
  assert.doesNotMatch(text, /update-.*\.json|SHA256SUMS/);
  files.set('install.sh', Buffer.from('changed'));
  assert.throws(() => verifyChecksums({ text, names: [...files.keys()], sha256Hex: hex }), /SHA256SUMS/);
});

test('only publishable package files are release assets', () => {
  for (const name of ['nevermind-next-1.2.3-arm64.dmg', 'nevermind-next_1.2.3_amd64.deb', 'nevermind-next-setup-1.2.3-amd64.exe', 'update-linux-arm64.json', 'install.sh', 'install.ps1'])
    assert.equal(isReleaseAsset(name), true, name);
  for (const name of ['Nevermind Next.exe', 'nevermind-next', 'SHA256SUMS', 'PKGBUILD', 'nevermind-next-fast.zip'])
    assert.equal(isReleaseAsset(name), false, name);
});

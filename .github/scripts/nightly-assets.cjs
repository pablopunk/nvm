'use strict';

const crypto = require('node:crypto');

// The Go app's release assets, as `mygo build` names them (src/native/mygo.json slug) and native-package.yml builds
// them: one disk image and update archive per Mac, an archive and Debian package per Linux machine, the Windows
// installer and archive, MyGo's install scripts, and one SHA256SUMS for the lot. Each update-<target>.json is the
// manifest that installed apps read; its archive and delta URLs name the release that holds them, so promotion
// retargets them (retargetUpdaterMetadata) and leaves every other byte as built.
const SLUG = 'nevermind-next';
const UPDATE_TARGETS = [
  'darwin-amd64',
  'darwin-arm64',
  'linux-amd64',
  'linux-arm64',
  'windows-amd64',
];
const NIGHTLY_UPDATER_METADATA = UPDATE_TARGETS.map(
  (target) => `update-${target}.json`,
);
const CHECKSUMS = 'SHA256SUMS';
const VERSION_PATTERN = /^\d+\.\d+\.\d+$/;
// The DER prefix of an Ed25519 public key, before its 32 bytes.
const ED25519_SPKI_PREFIX = Buffer.from('302a300506032b6570032100', 'hex');

function requireReleaseVersion(version) {
  if (!VERSION_PATTERN.test(version ?? ''))
    throw new Error(`Invalid release version: ${version}`);
}

function updateArchiveName(version, target) {
  return `${SLUG}-${version}-${target}.tar.gz`;
}

function expectedNightlyAssetNames(version) {
  requireReleaseVersion(version);
  return [
    `${SLUG}-${version}-arm64.dmg`,
    `${SLUG}-${version}-amd64.dmg`,
    ...UPDATE_TARGETS.map((target) => updateArchiveName(version, target)),
    `${SLUG}_${version}_amd64.deb`,
    `${SLUG}_${version}_arm64.deb`,
    `${SLUG}-setup-${version}-amd64.exe`,
    ...NIGHTLY_UPDATER_METADATA,
    'install.sh',
    'install.ps1',
    CHECKSUMS,
  ];
}

// A delta updates an earlier published version to this one; MyGo makes up to three per target when an earlier
// version is published, and none for the first.
function deltaAsset(name, version) {
  const match =
    /^nevermind-next-(\d+\.\d+\.\d+)-to-(\d+\.\d+\.\d+)-([a-z]+-[a-z0-9]+)\.delta$/.exec(
      name,
    );
  if (!match || match[2] !== version || !UPDATE_TARGETS.includes(match[3]))
    return null;
  return { from: match[1], target: match[3] };
}

// Publication requires the exact expected set: a missing or extra asset means a
// partial or contaminated build, and nothing may become public.
function validateNightlyAssetSet(names, version) {
  const expected = expectedNightlyAssetNames(version);
  const actual = [...names].sort();
  const duplicates = actual.filter(
    (name, index) => actual.indexOf(name) !== index,
  );
  if (duplicates.length > 0)
    throw new Error(`Duplicate release assets: ${duplicates.join(', ')}`);
  const missing = expected.filter((name) => !actual.includes(name));
  const extra = actual.filter(
    (name) => !expected.includes(name) && !deltaAsset(name, version),
  );
  if (missing.length > 0 || extra.length > 0)
    throw new Error(
      `Release asset set mismatch. Missing: [${missing.join(', ')}]. Unexpected: [${extra.join(', ')}].`,
    );
  return expected;
}

function releaseDownloadPrefix(repository, tag) {
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository ?? ''))
    throw new Error('Invalid repository');
  return `https://github.com/${repository}/releases/download/${encodeURIComponent(tag)}/`;
}

function verifySignature(publicKey, digest, signature, label) {
  if (!publicKey) return;
  const raw = Buffer.from(publicKey.trim(), 'base64');
  if (raw.length !== 32) throw new Error('Invalid update public key');
  const key = crypto.createPublicKey({
    key: Buffer.concat([ED25519_SPKI_PREFIX, raw]),
    format: 'der',
    type: 'spki',
  });
  if (!crypto.verify(null, digest, key, Buffer.from(signature ?? '', 'base64')))
    throw new Error(`${label} is not signed with the update key`);
}

// `sha256Of(name)` returns the SHA-256 digest of a staged asset as a Buffer, and
// `sizeOf(name)` its size. Every file a manifest references must be an asset of
// this release, at the release's tag, with its size and a valid signature.
function verifyUpdaterMetadata({
  name,
  text,
  version,
  repository,
  tag,
  assetNames,
  sha256Of,
  sizeOf,
  publicKey,
}) {
  const target = /^update-(.+)\.json$/.exec(name)?.[1];
  if (!UPDATE_TARGETS.includes(target))
    throw new Error(`${name} is not an update manifest`);
  const manifest = JSON.parse(text);
  if (manifest.version !== version)
    throw new Error(
      `${name} version ${manifest.version} does not match ${version}`,
    );
  const prefix = releaseDownloadPrefix(repository, tag);
  const checkFile = (url, size, signature, label) => {
    if (typeof url !== 'string' || !url.startsWith(prefix))
      throw new Error(`${label} ${url} is not in the release ${tag}`);
    const file = decodeURIComponent(url.slice(prefix.length));
    if (!assetNames.includes(file))
      throw new Error(
        `${label} references ${file}, which is not a release asset`,
      );
    if (Number(size) !== sizeOf(file))
      throw new Error(
        `${label} size for ${file} does not match the staged file`,
      );
    verifySignature(publicKey, sha256Of(file), signature, `${label} ${file}`);
    return file;
  };
  const archive = checkFile(
    manifest.url,
    manifest.size,
    manifest.signature,
    name,
  );
  if (archive !== updateArchiveName(version, target))
    throw new Error(
      `${name} must update to ${updateArchiveName(version, target)}`,
    );
  for (const delta of manifest.deltas ?? []) {
    const file = checkFile(
      delta.url,
      delta.size,
      delta.signature,
      `${name} delta`,
    );
    const parsed = deltaAsset(file, version);
    if (!parsed || parsed.target !== target || parsed.from !== delta.from)
      throw new Error(
        `${name} delta ${file} does not update ${delta.from} on ${target}`,
      );
  }
  return manifest;
}

// Points a manifest's archive and delta URLs at the release `tag` of the same version: its dev, nightly or stable
// release. The archives are signed, not the manifest, so the signatures stay valid.
function retargetUpdaterMetadata({ text, repository, version, tag }) {
  requireReleaseVersion(version);
  const manifest = JSON.parse(text);
  if (manifest.version !== version)
    throw new Error(
      `Manifest version ${manifest.version} does not match ${version}`,
    );
  const sources = [`v${version}`, `v${version}-dev`, `v${version}-nightly`].map(
    (sourceTag) => releaseDownloadPrefix(repository, sourceTag),
  );
  const destination = releaseDownloadPrefix(repository, tag);
  if (!sources.includes(destination))
    throw new Error(`${tag} is not a release of ${version}`);
  const retarget = (url) => {
    const source = sources.find((prefix) => url?.startsWith?.(prefix));
    if (!source)
      throw new Error(`${url} is not a file of a release of ${version}`);
    return destination + url.slice(source.length);
  };
  manifest.url = retarget(manifest.url);
  for (const delta of manifest.deltas ?? []) delta.url = retarget(delta.url);
  return `${JSON.stringify(manifest, null, 2)}\n`;
}

// SHA256SUMS lists every asset but itself and the manifests, which promotion retargets, in sha256sum's format.
function checksumsFor(names, sha256Hex) {
  return `${names
    .filter(
      (name) => name !== CHECKSUMS && !NIGHTLY_UPDATER_METADATA.includes(name),
    )
    .sort()
    .map((name) => `${sha256Hex(name)}  ${name}`)
    .join('\n')}\n`;
}

function verifyChecksums({ text, names, sha256Hex }) {
  if (text !== checksumsFor(names, sha256Hex))
    throw new Error(`${CHECKSUMS} does not list exactly the staged assets`);
}

// Whether a file a packaging job uploaded belongs in the release.
function isReleaseAsset(name) {
  return (
    /\.(dmg|tar\.gz|deb|delta)$/.test(name) ||
    /^nevermind-next-setup-.+\.exe$/.test(name) ||
    /^update-.+\.json$/.test(name) ||
    name === 'install.sh' ||
    name === 'install.ps1'
  );
}

module.exports = {
  CHECKSUMS,
  NIGHTLY_UPDATER_METADATA,
  checksumsFor,
  expectedNightlyAssetNames,
  isReleaseAsset,
  retargetUpdaterMetadata,
  validateNightlyAssetSet,
  verifyChecksums,
  verifyUpdaterMetadata,
};

'use strict';

// Asset names mirror the stable packaging collection in .github/workflows/ci.yml,
// with the updater metadata renamed to the channel files electron-updater reads:
// nightly.yml (Windows), nightly-mac.yml (macOS), and nightly-linux[-arm64].yml.
const NIGHTLY_UPDATER_METADATA = [
  'nightly-linux-arm64.yml',
  'nightly-linux.yml',
  'nightly-mac.yml',
  'nightly.yml',
];

function expectedNightlyAssetNames(version) {
  return [
    `Nevermind-${version}-win-x64-setup.exe`,
    `Nevermind-${version}-win-x64-setup.exe.blockmap`,
    `Nevermind-${version}-win-x64-portable.exe`,
    `Nevermind-${version}-mac-arm64.zip`,
    `Nevermind-${version}-mac-arm64.zip.blockmap`,
    `Nevermind-${version}-linux-x86_64.AppImage`,
    `Nevermind-${version}-linux-x86_64.AppImage.blockmap`,
    `Nevermind-${version}-linux-amd64.deb`,
    `Nevermind-${version}-linux-arm64.AppImage`,
    `Nevermind-${version}-linux-arm64.AppImage.blockmap`,
    `Nevermind-${version}-linux-arm64.deb`,
    'nightly.yml',
    'nightly-mac.yml',
    'nightly-linux.yml',
    'nightly-linux-arm64.yml',
  ];
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
  const extra = actual.filter((name) => !expected.includes(name));
  if (missing.length > 0 || extra.length > 0)
    throw new Error(
      `Release asset set mismatch. Missing: [${missing.join(', ')}]. Unexpected: [${extra.join(', ')}].`,
    );
  return expected;
}

function yamlScalar(text) {
  return text.trim().replace(/^['"]|['"]$/g, '');
}

// Minimal parser for electron-builder's update metadata shape: top-level scalars
// plus a `files:` list of { url, sha512, size } entries.
function parseUpdaterMetadata(text) {
  const metadata = { version: null, path: null, sha512: null, files: [] };
  let current = null;
  for (const line of text.split(/\r?\n/)) {
    const top = /^([a-zA-Z0-9]+):\s*(.*)$/.exec(line);
    if (top) {
      current = null;
      if (top[1] === 'version') metadata.version = yamlScalar(top[2]);
      if (top[1] === 'path') metadata.path = yamlScalar(top[2]);
      if (top[1] === 'sha512') metadata.sha512 = yamlScalar(top[2]);
      continue;
    }
    const entry = /^\s*-\s+url:\s*(.+)$/.exec(line);
    if (entry) {
      current = { url: yamlScalar(entry[1]), sha512: null, size: null };
      metadata.files.push(current);
      continue;
    }
    const field = /^\s+(sha512|size):\s*(.+)$/.exec(line);
    if (field && current) current[field[1]] = yamlScalar(field[2]);
  }
  return metadata;
}

// `sha512Of(name)` returns the base64 SHA-512 of a staged asset, or null when the
// asset is absent. Every referenced file must exist with a matching digest and,
// when `sizeOf` is supplied, a matching byte size.
function verifyUpdaterMetadata({
  name,
  text,
  version,
  sha512Of,
  sizeOf,
  assetNames,
}) {
  const metadata = parseUpdaterMetadata(text);
  if (metadata.version !== version)
    throw new Error(
      `${name} version ${metadata.version} does not match ${version}`,
    );
  if (metadata.files.length === 0) throw new Error(`${name} lists no files`);
  if (!metadata.files.some((file) => file.url === metadata.path))
    throw new Error(`${name} path ${metadata.path} is not a listed file`);
  for (const file of metadata.files) {
    if (!assetNames.includes(file.url))
      throw new Error(
        `${name} references ${file.url}, which is not a release asset`,
      );
    const actual = sha512Of(file.url);
    if (!actual) throw new Error(`${name} references missing ${file.url}`);
    if (actual !== file.sha512)
      throw new Error(
        `${name} sha512 for ${file.url} does not match the staged file`,
      );
    if (sizeOf && Number(file.size) !== sizeOf(file.url))
      throw new Error(
        `${name} size for ${file.url} does not match the staged file`,
      );
  }
  const primary = metadata.files.find((file) => file.url === metadata.path);
  if (metadata.sha512 && metadata.sha512 !== primary.sha512)
    throw new Error(`${name} top-level sha512 does not match ${metadata.path}`);
  if (name === 'nightly-mac.yml') {
    const expected = `Nevermind-${version}-mac-arm64.zip`;
    if (
      metadata.files.length !== 1 ||
      metadata.files[0].url !== expected ||
      metadata.path !== expected
    ) {
      throw new Error(`${name} must target only the macOS arm64 ZIP`);
    }
  }
  return metadata;
}

module.exports = {
  NIGHTLY_UPDATER_METADATA,
  expectedNightlyAssetNames,
  parseUpdaterMetadata,
  validateNightlyAssetSet,
  verifyUpdaterMetadata,
};

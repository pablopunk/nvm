#!/usr/bin/env node
'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

// electron-builder writes no update metadata for its portable target, so
// releases generate it here in the shape electron-updater parses.

function fail(message) {
  throw new Error(`Windows updater metadata validation failed: ${message}`);
}

function portableArtifactName(version, arch) {
  return `Nevermind-${version}-win-${arch}-portable.exe`;
}

function sha512Base64(filePath) {
  return crypto
    .createHash('sha512')
    .update(fs.readFileSync(filePath))
    .digest('base64');
}

function unquote(value) {
  const trimmed = String(value || '').trim();
  if (
    (trimmed.startsWith("'") && trimmed.endsWith("'")) ||
    (trimmed.startsWith('"') && trimmed.endsWith('"'))
  )
    return trimmed.slice(1, -1);
  return trimmed;
}

function parseMetadata(source) {
  const lines = source.split(/\r?\n/);
  const scalar = (key) => {
    const match = lines.find((line) => new RegExp(`^${key}:\\s*`).test(line));
    return match ? unquote(match.replace(new RegExp(`^${key}:\\s*`), '')) : '';
  };
  const files = [];
  let current = null;
  const filesStart = lines.findIndex((line) => /^files:\s*$/.test(line));
  for (const line of filesStart >= 0 ? lines.slice(filesStart + 1) : []) {
    if (/^\S/.test(line)) break;
    const entry = /^\s*-\s*url:\s*(.+)$/.exec(line);
    if (entry) {
      current = { url: unquote(entry[1]), sha512: '', size: Number.NaN };
      files.push(current);
      continue;
    }
    const field = /^\s+(sha512|size):\s*(.+)$/.exec(line);
    if (field && current)
      current[field[1]] =
        field[1] === 'size' ? Number(unquote(field[2])) : unquote(field[2]);
  }
  return {
    version: scalar('version'),
    path: scalar('path'),
    sha512: scalar('sha512'),
    files,
  };
}

function writeWindowsPortableUpdaterMetadata(
  portablePath,
  metadataPath,
  version,
  releaseDate = new Date().toISOString(),
) {
  const name = path.basename(portablePath);
  const sha512 = sha512Base64(portablePath);
  const size = fs.statSync(portablePath).size;
  fs.writeFileSync(
    metadataPath,
    [
      `version: ${version}`,
      'files:',
      `  - url: ${name}`,
      `    sha512: ${sha512}`,
      `    size: ${size}`,
      `path: ${name}`,
      `sha512: ${sha512}`,
      `releaseDate: '${releaseDate}'`,
      '',
    ].join('\n'),
  );
}

function validateWindowsUpdaterMetadata(
  metadataPath,
  artifactDirectory,
  expectedVersion,
  expectedArch,
) {
  const metadata = parseMetadata(fs.readFileSync(metadataPath, 'utf8'));
  if (metadata.version !== expectedVersion)
    fail(
      `expected version ${expectedVersion}, received ${metadata.version || '<missing>'}`,
    );
  if (metadata.files.length !== 1)
    fail('metadata must contain exactly one portable file entry');
  const expectedName = portableArtifactName(expectedVersion, expectedArch);
  const [fileEntry] = metadata.files;
  if (metadata.path !== expectedName || fileEntry.url !== expectedName)
    fail(`path and files[0].url must both name ${expectedName}`);
  const portablePath = path.join(artifactDirectory, expectedName);
  if (!(fs.existsSync(portablePath) && fs.statSync(portablePath).size > 0))
    fail(`missing non-empty ${expectedName}`);
  const expectedHash = sha512Base64(portablePath);
  if (metadata.sha512 !== expectedHash || fileEntry.sha512 !== expectedHash)
    fail('top-level and file-entry SHA-512 must match the portable executable');
  if (fileEntry.size !== fs.statSync(portablePath).size)
    fail('file-entry size must match the portable executable');
  return { portablePath, sha512: expectedHash };
}

if (require.main === module) {
  try {
    const [command, ...args] = process.argv.slice(2);
    if (command === 'write' && args.length === 3) {
      const [portablePath, metadataPath, version] = args;
      writeWindowsPortableUpdaterMetadata(
        path.resolve(portablePath),
        path.resolve(metadataPath),
        version,
      );
      console.log(`Wrote ${metadataPath}`);
    } else if (command === 'validate' && args.length === 4) {
      const [metadataPath, artifactDirectory, version, arch] = args;
      validateWindowsUpdaterMetadata(
        path.resolve(metadataPath),
        path.resolve(artifactDirectory),
        version,
        arch,
      );
      console.log('Windows updater metadata checks passed');
    } else {
      fail(
        'usage: validate-windows-updater-metadata.cjs write <portable.exe> <metadata.yml> <version> | validate <metadata.yml> <artifact-dir> <version> <arch>',
      );
    }
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}

module.exports = {
  parseMetadata,
  portableArtifactName,
  validateWindowsUpdaterMetadata,
  writeWindowsPortableUpdaterMetadata,
};

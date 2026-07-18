#!/usr/bin/env node
'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

function fail(message) {
  throw new Error(`Windows updater metadata validation failed: ${message}`);
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
  const filesStart = lines.findIndex((line) => /^files:\s*$/.test(line));
  const fileLines = filesStart >= 0 ? lines.slice(filesStart + 1) : [];
  const fileUrl = fileLines.find((line) => /^\s*-\s*url:\s*/.test(line));
  const fileHash = fileLines.find((line) => /^\s+sha512:\s*/.test(line));
  const fileSize = fileLines.find((line) => /^\s+size:\s*/.test(line));
  const extraFileUrls = fileLines.filter((line) =>
    /^\s*-\s*url:\s*/.test(line),
  );
  return {
    version: scalar('version'),
    path: scalar('path'),
    sha512: scalar('sha512'),
    files: fileUrl
      ? [
          {
            url: unquote(fileUrl.replace(/^\s*-\s*url:\s*/, '')),
            sha512: unquote((fileHash || '').replace(/^\s+sha512:\s*/, '')),
            size: Number((fileSize || '').replace(/^\s+size:\s*/, '')),
          },
        ]
      : [],
    fileEntryCount: extraFileUrls.length,
  };
}

function sha512Base64(filePath) {
  return crypto
    .createHash('sha512')
    .update(fs.readFileSync(filePath))
    .digest('base64');
}

function validateWindowsUpdaterMetadata(
  metadataPath,
  artifactDirectory,
  expectedVersion,
  expectedArch,
) {
  const source = fs.readFileSync(metadataPath, 'utf8');
  const metadata = parseMetadata(source);
  if (metadata.version !== expectedVersion)
    fail(
      `expected version ${expectedVersion}, received ${metadata.version || '<missing>'}`,
    );
  if (metadata.fileEntryCount !== 1 || metadata.files.length !== 1)
    fail('metadata must contain exactly one NSIS file entry');
  const expectedSetupName = `Nevermind-${expectedVersion}-win-${expectedArch}-setup.exe`;
  const fileEntry = metadata.files[0];
  if (
    metadata.path !== expectedSetupName ||
    fileEntry.url !== expectedSetupName
  )
    fail(`path and files[0].url must both name ${expectedSetupName}`);
  if (!metadata.path.includes(`-${expectedArch}-`))
    fail('artifact architecture is missing');
  if (/portable/i.test(source))
    fail('portable artifacts are not updater targets');

  const setupPath = path.join(artifactDirectory, expectedSetupName);
  const blockmapPath = `${setupPath}.blockmap`;
  if (!(fs.existsSync(setupPath) && fs.statSync(setupPath).size > 0))
    fail(`missing non-empty ${expectedSetupName}`);
  if (!(fs.existsSync(blockmapPath) && fs.statSync(blockmapPath).size > 0))
    fail(`missing non-empty ${path.basename(blockmapPath)}`);
  const expectedHash = sha512Base64(setupPath);
  if (metadata.sha512 !== expectedHash || fileEntry.sha512 !== expectedHash)
    fail('top-level and file-entry SHA-512 must match the setup executable');
  if (fileEntry.size !== fs.statSync(setupPath).size)
    fail('file-entry size must match the setup executable');
  return { setupPath, blockmapPath, sha512: expectedHash };
}

if (require.main === module) {
  try {
    const [metadataPath, artifactDirectory, expectedVersion, expectedArch] =
      process.argv.slice(2);
    if (!(metadataPath && artifactDirectory && expectedVersion && expectedArch))
      fail(
        'usage: validate-windows-updater-metadata.cjs <latest.yml> <artifact-dir> <version> <arch>',
      );
    validateWindowsUpdaterMetadata(
      path.resolve(metadataPath),
      path.resolve(artifactDirectory),
      expectedVersion,
      expectedArch,
    );
    console.log('Windows updater metadata checks passed');
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}

module.exports = { parseMetadata, validateWindowsUpdaterMetadata };

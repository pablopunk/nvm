'use strict';

const { createHash } = require('node:crypto');
const { readFileSync, readdirSync, lstatSync } = require('node:fs');
const path = require('node:path');

function digest(file) {
  return createHash('sha256').update(readFileSync(file)).digest('hex');
}

function filesIn(directory) {
  const names = readdirSync(directory).sort();
  for (const name of names) {
    if (!lstatSync(path.join(directory, name)).isFile())
      throw new Error(`Artifact is not a regular file: ${name}`);
  }
  return names;
}

function createManifest({ directory, version, sourceSha, runId }) {
  if (!/^\d+\.\d+\.\d+$/.test(version)) throw new Error('Invalid version');
  if (!/^[0-9a-f]{40}$/.test(sourceSha)) throw new Error('Invalid source SHA');
  if (!/^[1-9]\d*$/.test(String(runId))) throw new Error('Invalid run ID');
  const names = filesIn(directory);
  if (!names.length) throw new Error('Empty artifact set');
  return {
    schema: 1,
    version,
    sourceSha,
    runId: String(runId),
    files: names.map((name) => ({
      name,
      sha256: digest(path.join(directory, name)),
    })),
  };
}

function verifyManifest({ directory, manifest, version, sourceSha, runId }) {
  if (!manifest || manifest.schema !== 1 || !Array.isArray(manifest.files))
    throw new Error('Invalid manifest');
  if (
    manifest.version !== version ||
    manifest.sourceSha !== sourceSha ||
    manifest.runId !== String(runId)
  )
    throw new Error('Artifact provenance mismatch');
  const actual = createManifest({ directory, ...manifest });
  if (JSON.stringify(actual) !== JSON.stringify(manifest))
    throw new Error('Artifact manifest mismatch');
}

module.exports = { createManifest, verifyManifest };

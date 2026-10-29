'use strict';

const { createHash } = require('node:crypto');
const {
  readFileSync,
  readdirSync,
  lstatSync,
  writeFileSync,
} = require('node:fs');
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

function main(args) {
  const [command, directory, manifestPath, version, sourceSha, runId] = args;
  if (!directory || !manifestPath || !version || !sourceSha || !runId)
    throw new Error(
      'Expected: create|verify directory manifest version source-sha run-id',
    );
  if (path.dirname(path.resolve(manifestPath)) === path.resolve(directory))
    throw new Error('Manifest must be stored outside the artifact directory');
  if (command === 'create') {
    const manifest = createManifest({ directory, version, sourceSha, runId });
    writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, {
      flag: 'wx',
    });
  } else if (command === 'verify') {
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    verifyManifest({ directory, manifest, version, sourceSha, runId });
  } else {
    throw new Error(`Unknown command: ${command}`);
  }
}

if (require.main === module) {
  try {
    main(process.argv.slice(2));
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}

module.exports = { createManifest, verifyManifest };

'use strict';

const assert = require('node:assert/strict');
const { mkdtempSync, rmSync, writeFileSync, symlinkSync } = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test } = require('node:test');
const { createManifest, verifyManifest } = require('./promotion-manifest.cjs');

test('manifest binds provenance and every artifact byte', (t) => {
  const directory = mkdtempSync(path.join(os.tmpdir(), 'promotion-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  writeFileSync(path.join(directory, 'package.zip'), 'signed bytes');
  const provenance = {
    version: '2026.1029.1',
    sourceSha: 'a'.repeat(40),
    runId: '123',
  };
  const manifest = createManifest({ directory, ...provenance });
  verifyManifest({ directory, manifest, ...provenance });
  assert.throws(
    () => verifyManifest({ directory, manifest, ...provenance, runId: '124' }),
    /provenance mismatch/,
  );
  writeFileSync(path.join(directory, 'package.zip'), 'changed bytes');
  assert.throws(
    () => verifyManifest({ directory, manifest, ...provenance }),
    /mismatch/,
  );
  writeFileSync(path.join(directory, 'extra'), 'unexpected');
  assert.throws(
    () => verifyManifest({ directory, manifest, ...provenance }),
    /mismatch/,
  );
});

test('rejects symlinks and invalid provenance', (t) => {
  const directory = mkdtempSync(path.join(os.tmpdir(), 'promotion-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  writeFileSync(path.join(directory, 'package.zip'), 'data');
  const input = {
    directory,
    version: '2026.1029.1',
    sourceSha: 'a'.repeat(40),
    runId: '1',
  };
  assert.throws(() => createManifest({ ...input, sourceSha: 'main' }), /SHA/);
  symlinkSync('package.zip', path.join(directory, 'link'));
  assert.throws(() => createManifest(input), /regular file/);
});

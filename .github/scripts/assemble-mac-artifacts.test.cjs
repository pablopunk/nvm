'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const { assembleMacArtifacts } = require('./assemble-mac-artifacts.cjs');
const { parseUpdaterMetadata } = require('./nightly-assets.cjs');

test('assembles both updater targets and rejects a corrupted architecture', (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'mac-assembly-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  for (const arch of ['arm64', 'x64']) {
    const dir = path.join(root, `mac-build-${arch}`);
    fs.mkdirSync(dir);
    const zip = `Nevermind-1.2.3-mac-${arch}.zip`;
    const bytes = Buffer.from(arch);
    const hash = crypto.createHash('sha512').update(bytes).digest('base64');
    fs.writeFileSync(path.join(dir, zip), bytes);
    fs.writeFileSync(path.join(dir, `${zip}.blockmap`), 'blockmap');
    fs.writeFileSync(
      path.join(dir, 'nightly-mac.yml'),
      `version: 1.2.3\nfiles:\n  - url: ${zip}\n    sha512: ${hash}\n    size: ${bytes.length}\npath: ${zip}\nsha512: ${hash}\nreleaseDate: '2026-01-01T00:00:00Z'\n`,
    );
  }
  const output = path.join(root, 'output');
  assembleMacArtifacts(root, output, '1.2.3');
  assert.equal(
    parseUpdaterMetadata(
      fs.readFileSync(path.join(output, 'nightly-mac.yml'), 'utf8'),
    ).files.length,
    2,
  );
  fs.writeFileSync(
    path.join(root, 'mac-build-x64', 'Nevermind-1.2.3-mac-x64.zip'),
    'corrupt',
  );
  assert.throws(
    () => assembleMacArtifacts(root, path.join(root, 'bad'), '1.2.3'),
    /sha512/,
  );
});

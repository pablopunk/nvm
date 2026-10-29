'use strict';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const { test } = require('node:test');
const {
  expectedNightlyAssetNames,
  NIGHTLY_UPDATER_METADATA,
} = require('./nightly-assets.cjs');
const { stampMetadata } = require('./stamp-state-compatibility.cjs');
const { createManifest, verifyManifest } = require('./promotion-manifest.cjs');
const { epoch } = require('../../src/app/resources/state-compatibility.json');

test('dev assets retain verified metadata and bytes through stable promotion', (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'nvm-promotion-flow-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const dev = path.join(root, 'dev');
  const stable = path.join(root, 'stable');
  fs.mkdirSync(dev);
  fs.mkdirSync(stable);
  const version = '2026.1029.901';
  const names = expectedNightlyAssetNames(version);
  const payloads = names.filter(
    (name) => !NIGHTLY_UPDATER_METADATA.includes(name),
  );
  for (const name of payloads)
    fs.writeFileSync(path.join(dev, name), `bytes:${name}`);
  const binary = payloads[0];
  const digest = crypto
    .createHash('sha512')
    .update(fs.readFileSync(path.join(dev, binary)))
    .digest('base64');
  const metadata = `version: ${version}\nfiles:\n  - url: ${binary}\n    sha512: ${digest}\n    size: ${fs.statSync(path.join(dev, binary)).size}\npath: ${binary}\nsha512: ${digest}\n`;
  for (const name of NIGHTLY_UPDATER_METADATA) {
    const file = path.join(dev, name);
    fs.writeFileSync(file, metadata);
    stampMetadata(file);
  }
  const cli = path.join(__dirname, 'nightly-release-cli.cjs');
  execFileSync(process.execPath, [
    cli,
    'validate-assets',
    '--dir',
    dev,
    '--version',
    version,
    '--state-epoch',
    String(epoch),
  ]);
  const provenance = { version, sourceSha: 'a'.repeat(40), runId: '42' };
  const manifest = createManifest({ directory: dev, ...provenance });
  for (const name of names)
    fs.copyFileSync(path.join(dev, name), path.join(stable, name));
  verifyManifest({ directory: stable, manifest, ...provenance });
  for (const suffix of ['', '-mac', '-linux', '-linux-arm64']) {
    fs.copyFileSync(
      path.join(stable, `nightly${suffix}.yml`),
      path.join(stable, `latest${suffix}.yml`),
    );
    assert.equal(
      fs.readFileSync(path.join(stable, `latest${suffix}.yml`), 'utf8'),
      fs.readFileSync(path.join(dev, `nightly${suffix}.yml`), 'utf8'),
    );
  }
  fs.writeFileSync(path.join(dev, 'nightly.yml'), metadata);
  assert.throws(() =>
    execFileSync(
      process.execPath,
      [
        cli,
        'validate-assets',
        '--dir',
        dev,
        '--version',
        version,
        '--state-epoch',
        String(epoch),
      ],
      { stdio: 'pipe' },
    ),
  );
});

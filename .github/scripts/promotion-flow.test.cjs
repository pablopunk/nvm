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
const { createManifest, verifyManifest } = require('./promotion-manifest.cjs');

const CLI = path.join(__dirname, 'nightly-release-cli.cjs');
const REPOSITORY = 'pablopunk/nvm';

function cli(args) {
  return execFileSync(process.execPath, [CLI, ...args], {
    env: { ...process.env, GITHUB_REPOSITORY: REPOSITORY },
    stdio: 'pipe',
    encoding: 'utf8',
  });
}

// The packaging jobs' artifacts as native-package.yml uploads them: one directory per job, each with its own
// SHA256SUMS and files that are not release assets.
function writePackagingArtifacts(root, version, privateKey) {
  const jobs = {
    'native-macos-arm64': [
      'darwin-arm64',
      `nevermind-next-${version}-arm64.dmg`,
      'install.sh',
    ],
    'native-macos-amd64': [
      'darwin-amd64',
      `nevermind-next-${version}-amd64.dmg`,
      'install.sh',
    ],
    'native-linux-amd64': [
      'linux-amd64',
      `nevermind-next_${version}_amd64.deb`,
      'install.sh',
    ],
    'native-linux-arm64': [
      'linux-arm64',
      `nevermind-next_${version}_arm64.deb`,
      'install.sh',
    ],
    'native-windows-amd64': [
      'windows-amd64',
      `nevermind-next-setup-${version}-amd64.exe`,
      'install.ps1',
    ],
  };
  for (const [job, [target, installer, script]] of Object.entries(jobs)) {
    const directory = path.join(root, job);
    fs.mkdirSync(directory, { recursive: true });
    const archive = `nevermind-next-${version}-${target}.tar.gz`;
    const bytes = Buffer.from(`bytes:${archive}`);
    fs.writeFileSync(path.join(directory, archive), bytes);
    fs.writeFileSync(path.join(directory, installer), `bytes:${installer}`);
    fs.writeFileSync(path.join(directory, script), `script:${script}`);
    fs.writeFileSync(
      path.join(directory, 'SHA256SUMS'),
      `job sums of ${job}\n`,
    );
    const digest = crypto.createHash('sha256').update(bytes).digest();
    fs.writeFileSync(
      path.join(directory, `update-${target}.json`),
      JSON.stringify({
        version,
        url: `https://github.com/${REPOSITORY}/releases/download/v${version}/${archive}`,
        size: bytes.length,
        signature: crypto.sign(null, digest, privateKey).toString('base64'),
      }),
    );
  }
}

test('dev assets keep their verified bytes through nightly and stable promotion', (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'nvm-promotion-flow-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const version = '2026.1029.901';
  const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
  const key = publicKey
    .export({ format: 'der', type: 'spki' })
    .subarray(12)
    .toString('base64');
  const packages = path.join(root, 'packages');
  const dev = path.join(root, 'dev');
  writePackagingArtifacts(packages, version, privateKey);

  cli(['assemble-assets', '--from', packages, '--dir', dev]);
  cli([
    'retarget-metadata',
    '--dir',
    dev,
    '--version',
    version,
    '--tag',
    `v${version}-dev`,
  ]);
  cli([
    'validate-assets',
    '--dir',
    dev,
    '--version',
    version,
    '--tag',
    `v${version}-dev`,
    '--public-key',
    key,
  ]);
  assert.deepEqual(
    fs.readdirSync(dev).sort(),
    [...expectedNightlyAssetNames(version)].sort(),
  );

  const provenance = { version, sourceSha: 'a'.repeat(40), runId: '42' };
  const manifest = createManifest({ directory: dev, ...provenance });
  for (const [channel, tag] of [
    ['nightly', `v${version}-nightly`],
    ['stable', `v${version}`],
  ]) {
    const staged = path.join(root, channel);
    fs.mkdirSync(staged);
    for (const name of fs.readdirSync(dev))
      fs.copyFileSync(path.join(dev, name), path.join(staged, name));
    verifyManifest({ directory: staged, manifest, ...provenance });
    cli([
      'retarget-metadata',
      '--dir',
      staged,
      '--version',
      version,
      '--tag',
      tag,
    ]);
    cli([
      'validate-assets',
      '--dir',
      staged,
      '--version',
      version,
      '--tag',
      tag,
      '--public-key',
      key,
    ]);
    for (const name of fs
      .readdirSync(dev)
      .filter((name) => !NIGHTLY_UPDATER_METADATA.includes(name)))
      assert.ok(
        fs
          .readFileSync(path.join(staged, name))
          .equals(fs.readFileSync(path.join(dev, name))),
        name,
      );
    assert.match(
      fs.readFileSync(path.join(staged, 'update-linux-amd64.json'), 'utf8'),
      new RegExp(`/releases/download/${tag}/`),
    );
  }

  fs.writeFileSync(path.join(dev, 'install.sh'), 'tampered');
  assert.throws(() =>
    cli([
      'validate-assets',
      '--dir',
      dev,
      '--version',
      version,
      '--tag',
      `v${version}-dev`,
      '--public-key',
      key,
    ]),
  );
});

test('assembly refuses two different files of one name', (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'nvm-promotion-flow-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const { privateKey } = crypto.generateKeyPairSync('ed25519');
  const packages = path.join(root, 'packages');
  writePackagingArtifacts(packages, '2026.1029.902', privateKey);
  fs.writeFileSync(
    path.join(packages, 'native-linux-arm64', 'install.sh'),
    'another script',
  );
  assert.throws(
    () =>
      cli([
        'assemble-assets',
        '--from',
        packages,
        '--dir',
        path.join(root, 'dev'),
      ]),
    /another install\.sh/,
  );
});

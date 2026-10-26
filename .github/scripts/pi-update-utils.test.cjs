const assert = require('node:assert/strict');
const test = require('node:test');
const { parse: parseYaml, stringify: stringifyYaml } = require('yaml');
const {
  DEFAULT_MINIMUM_RELEASE_AGE_MINUTES,
  compareVersions,
  isExcludedFromMinimumAge,
  isOldEnough,
  lockPackageVersions,
  minimumAgeExclusions,
  minimumReleaseAgeMinutes,
  packageVersionDelta,
  parseStableVersion,
  validateLockfile,
  validateManifestChanges,
} = require('./pi-update-utils.cjs');

const versions = {
  '@earendil-works/pi-ai': '1.4.0',
  '@earendil-works/pi-coding-agent': '1.4.0',
};

function lockfile(extraPackage = '') {
  return [
    "lockfileVersion: '9.0'",
    '',
    'importers:',
    '  .:',
    '    dependencies:',
    "      '@earendil-works/pi-ai':",
    '        specifier: 1.4.0',
    '        version: 1.4.0',
    "      '@earendil-works/pi-coding-agent':",
    '        specifier: 1.4.0',
    '        version: 1.4.0',
    '',
    'packages:',
    "  '@earendil-works/pi-ai@1.4.0':",
    '    resolution: {integrity: sha512-example}',
    "  '@earendil-works/pi-coding-agent@1.4.0':",
    '    resolution: {tarball: https://registry.npmjs.org/pi-coding-agent/-/pi-coding-agent-1.4.0.tgz}',
    extraPackage,
    '',
    'snapshots:',
    "  '@earendil-works/pi-ai@1.4.0': {}",
    "  '@earendil-works/pi-coding-agent@1.4.0': {}",
    '',
  ].join('\n');
}

function pnpmPiWebPeerContextLockfile(piVersion) {
  const piAiPath =
    piVersion === '0.80.10'
      ? '0.80.10(supports-color@8.1.1)(ws@8.21.0)(zod@4.4.3)'
      : '1.4.0(@aws-sdk/credential-provider-node@3.972.88)(@smithy/signature-v4@5.7.4)(supports-color@8.1.1)(undici@8.10.2)(ws@8.21.0)(zod@4.4.3)';
  const piCodingAgentPath =
    piVersion === '0.80.10'
      ? '0.80.10(supports-color@8.1.1)(ws@8.21.0)(zod@4.4.3)'
      : '1.4.0(@aws-sdk/credential-provider-node@3.972.88)(@smithy/signature-v4@5.7.4)(supports-color@8.1.1)(ws@8.21.0)(zod@4.4.3)';
  const piTuiVersion = piVersion;
  const webVersion = [
    `0.24.0(@earendil-works/pi-ai@${piAiPath})`,
    `(@earendil-works/pi-coding-agent@${piCodingAgentPath})`,
    `(@earendil-works/pi-tui@${piTuiVersion})`,
  ].join('');
  return stringifyYaml({
    lockfileVersion: '9.0',
    settings: { autoInstallPeers: true, excludeLinksFromLockfile: false },
    importers: {
      '.': {
        dependencies: {
          '@earendil-works/pi-ai': {
            specifier: piVersion,
            version: piAiPath,
          },
          '@earendil-works/pi-coding-agent': {
            specifier: piVersion,
            version: piCodingAgentPath,
          },
          'pi-web-access': { specifier: '0.24.0', version: webVersion },
          unrelated: {
            specifier: '1.0.0',
            version: '1.0.0(peer-runtime@1.0.0)',
          },
        },
      },
    },
    packages: {
      [`@earendil-works/pi-ai@${piVersion}`]: {
        resolution: { integrity: `sha512-pi-ai-${piVersion}` },
      },
      [`@earendil-works/pi-coding-agent@${piVersion}`]: {
        resolution: { integrity: `sha512-pi-agent-${piVersion}` },
      },
      [`@earendil-works/pi-tui@${piTuiVersion}`]: {
        resolution: { integrity: `sha512-pi-tui-${piTuiVersion}` },
      },
      'pi-web-access@0.24.0': {
        resolution: { integrity: 'sha512-pi-web-access-0.24.0' },
        peerDependencies: {
          '@earendil-works/pi-ai': '*',
          '@earendil-works/pi-coding-agent': '*',
          '@earendil-works/pi-tui': '*',
        },
      },
    },
    snapshots: {
      [`@earendil-works/pi-ai@${piAiPath}`]: {},
      [`@earendil-works/pi-coding-agent@${piCodingAgentPath}`]: {
        dependencies: { '@earendil-works/pi-tui': piTuiVersion },
      },
      [`@earendil-works/pi-tui@${piTuiVersion}`]: {},
      [`pi-web-access@${webVersion}`]: {
        dependencies: {
          '@earendil-works/pi-ai': piAiPath,
          '@earendil-works/pi-coding-agent': piCodingAgentPath,
          '@earendil-works/pi-tui': piTuiVersion,
          '@mozilla/readability': '0.6.0',
        },
      },
    },
  });
}

test('Pi candidates are exact stable semver versions and majors remain comparable', function semverPolicy() {
  assert.deepEqual(parseStableVersion('1.4.0'), [1, 4, 0]);
  assert.equal(parseStableVersion('1.4.0-rc.1'), null);
  assert.equal(parseStableVersion('9007199254740992.0.0'), null);
  assert.equal(compareVersions('2.0.0', '1.99.99') > 0, true);
});

test('resolver applies pnpm minimum-age defaults, explicit policy, and package exclusions', function releaseAgePolicy() {
  assert.equal(
    minimumReleaseAgeMinutes(''),
    DEFAULT_MINIMUM_RELEASE_AGE_MINUTES,
  );
  assert.equal(minimumReleaseAgeMinutes('minimumReleaseAge: 60\n'), 60);
  const exclusions = minimumAgeExclusions(
    "minimumReleaseAgeExclude:\n  - 'torph@0.1.3'\n  - '@scope/pkg'\n",
  );
  assert.deepEqual(exclusions, ['torph@0.1.3', '@scope/pkg']);
  assert.equal(isExcludedFromMinimumAge('torph', '0.1.3', exclusions), true);
  assert.equal(
    isExcludedFromMinimumAge('@scope/pkg', '4.2.0', exclusions),
    true,
  );
  assert.equal(
    isOldEnough(
      '2026-10-25T00:00:00Z',
      60,
      Date.parse('2026-10-26T00:00:00Z'),
      false,
    ),
    true,
  );
  assert.equal(
    isOldEnough(
      '2026-10-25T23:30:00Z',
      60,
      Date.parse('2026-10-26T00:00:00Z'),
      false,
    ),
    false,
  );
});

test('candidate lockfiles retain exact Pi importer pins and report package-version deltas', function exactLockPins() {
  const current = lockfile();
  validateLockfile(current, versions);
  assert.deepEqual(lockPackageVersions(current).get('@earendil-works/pi-ai'), [
    '1.4.0',
  ]);
  assert.deepEqual(packageVersionDelta(current, current), []);
  assert.throws(
    () =>
      validateLockfile(
        current.replace('specifier: 1.4.0', 'specifier: ^1.4.0'),
        versions,
      ),
    /root importer/,
  );
});

test('new local, git, and non-registry lockfile resolutions fail while unchanged baseline entries remain allowed', function resolutionBoundary() {
  const baseline = lockfile(
    "  'old-local-package@1.0.0':\n    resolution: {tarball: file:../reviewed-old.tgz}",
  );
  validateLockfile(baseline, versions, baseline);
  const candidate = lockfile(
    "  'old-local-package@1.0.0':\n    resolution: {tarball: file:../reviewed-old.tgz}\n  'new-package@2.0.0':\n    resolution: {tarball: https://example.invalid/new-package.tgz}",
  );
  assert.throws(
    () => validateLockfile(candidate, versions, baseline),
    /new non-registry package resolution/,
  );
  assert.throws(
    () =>
      validateLockfile(
        lockfile(
          "  'new-git@2.0.0':\n    resolution: {repo: git+https://example.invalid/repo.git}",
        ),
        versions,
      ),
    /new non-registry package resolution/,
  );
  assert.throws(
    () =>
      validateLockfile(
        lockfile(
          "  'new-github@github:owner/repository#commit':\n    resolution: {integrity: sha512-example}",
        ),
        versions,
      ),
    /new non-registry package resolution/,
  );
  assert.throws(
    () =>
      validateLockfile(
        lockfile(
          "  'new-tarball@2.0.0':\n    resolution:\n      tarball: 'https://example.invalid/package.tgz'",
        ),
        versions,
      ),
    /new non-registry package resolution/,
  );
  assert.throws(
    () =>
      validateLockfile(
        lockfile(
          "  'new-ssh@ssh://example.invalid/repo#commit':\n    resolution: {integrity: sha512-example}",
        ),
        versions,
      ),
    /new non-registry package resolution/,
  );
  assert.throws(
    () =>
      validateLockfile(
        lockfile(
          "  'new-url@2.0.0':\n    resolution:\n      type: tarball\n      url: 'https://example.invalid/package.tgz'",
        ),
        versions,
      ),
    /new non-registry package resolution/,
  );
  validateLockfile(
    lockfile(
      "  'alias@npm:real-package@2.0.0':\n    resolution: {tarball: https://registry.npmjs.org/real-package/-/real-package-2.0.0.tgz}",
    ),
    versions,
  );
});

test('publisher rejects unrelated importer and pnpm policy edits in a candidate lockfile', function importerAndPolicyBoundary() {
  const baseline = lockfile().replace(
    '        version: 1.4.0\n\npackages:',
    '        version: 1.4.0\n      unrelated:\n        specifier: 1.0.0\n        version: 1.0.0\n\npackages:',
  );
  validateLockfile(baseline, versions, baseline);
  const unrelatedImporterEdit = baseline.replace(
    '        specifier: 1.0.0\n        version: 1.0.0',
    '        specifier: 2.0.0\n        version: 2.0.0',
  );
  assert.throws(
    () => validateLockfile(unrelatedImporterEdit, versions, baseline),
    /changed importers beyond the two direct Pi pins/,
  );

  const baselineWithPolicy = baseline.replace(
    "lockfileVersion: '9.0'",
    "lockfileVersion: '9.1'",
  );
  assert.throws(
    () => validateLockfile(baseline, versions, baselineWithPolicy),
    /changed pnpm policy or metadata/,
  );
});

test('publisher accepts only coherent Pi-induced pi-web-access peer context changes', function piWebPeerContextBoundary() {
  const baseline = pnpmPiWebPeerContextLockfile('0.80.10');
  const candidate = pnpmPiWebPeerContextLockfile('1.4.0');
  validateLockfile(candidate, versions, baseline);

  const wrongPiPeer = parseYaml(candidate);
  wrongPiPeer.importers['.'].dependencies['pi-web-access'].version =
    wrongPiPeer.importers['.'].dependencies['pi-web-access'].version.replace(
      '@earendil-works/pi-tui@1.4.0',
      '@earendil-works/pi-tui@9.9.9',
    );
  assert.throws(
    () => validateLockfile(stringifyYaml(wrongPiPeer), versions, baseline),
    /changed the @earendil-works\/pi-tui peer context incoherently/,
  );

  const unrelatedPeer = parseYaml(candidate);
  unrelatedPeer.importers['.'].dependencies.unrelated.version =
    '1.0.0(peer-runtime@2.0.0)';
  assert.throws(
    () => validateLockfile(stringifyYaml(unrelatedPeer), versions, baseline),
    /changed importers beyond the two direct Pi pins/,
  );

  const changedSpecifier = parseYaml(candidate);
  changedSpecifier.importers['.'].dependencies['pi-web-access'].specifier =
    '0.25.0';
  assert.throws(
    () => validateLockfile(stringifyYaml(changedSpecifier), versions, baseline),
    /changed the pi-web-access specifier/,
  );

  const changedSource = parseYaml(candidate);
  changedSource.packages['pi-web-access@0.24.0'].resolution.integrity =
    'sha512-unreviewed-source';
  assert.throws(
    () => validateLockfile(stringifyYaml(changedSource), versions, baseline),
    /changed the pi-web-access package source or metadata/,
  );

  const changedPolicy = parseYaml(candidate);
  changedPolicy.settings.autoInstallPeers = false;
  assert.throws(
    () => validateLockfile(stringifyYaml(changedPolicy), versions, baseline),
    /changed pnpm policy or metadata/,
  );
});

test('resolver manifest validation allows only the two direct Pi pins', function manifestBoundary() {
  const base = JSON.stringify({
    name: 'fixture',
    dependencies: {
      '@earendil-works/pi-ai': '0.80.10',
      '@earendil-works/pi-coding-agent': '0.80.10',
      other: '^1.0.0',
    },
  });
  const candidate = JSON.stringify({
    name: 'fixture',
    dependencies: {
      '@earendil-works/pi-ai': '1.4.0',
      '@earendil-works/pi-coding-agent': '1.4.0',
      other: '^1.0.0',
    },
  });
  validateManifestChanges(
    base,
    candidate,
    {
      '@earendil-works/pi-ai': '0.80.10',
      '@earendil-works/pi-coding-agent': '0.80.10',
    },
    versions,
  );
  assert.throws(
    () =>
      validateManifestChanges(
        base,
        candidate.replace('^1.0.0', '2.0.0'),
        {
          '@earendil-works/pi-ai': '0.80.10',
          '@earendil-works/pi-coding-agent': '0.80.10',
        },
        versions,
      ),
    /beyond the two exact Pi pins/,
  );
});

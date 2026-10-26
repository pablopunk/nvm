const assert = require('node:assert/strict');
const test = require('node:test');
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

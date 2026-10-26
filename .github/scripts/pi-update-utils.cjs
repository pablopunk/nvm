const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const PI_PACKAGES = [
  '@earendil-works/pi-ai',
  '@earendil-works/pi-coding-agent',
];
const PACKAGE_FILES = ['package.json', 'pnpm-lock.yaml'];
const DEFAULT_MINIMUM_RELEASE_AGE_MINUTES = 1_440;

function parseStableVersion(version) {
  const match = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.exec(version);
  if (!match) return null;
  const components = match.slice(1).map(Number);
  return components.every(Number.isSafeInteger) ? components : null;
}

function compareVersions(left, right) {
  const a = parseStableVersion(left);
  const b = parseStableVersion(right);
  if (!a || !b)
    throw new Error('Only canonical stable Pi versions are accepted.');
  for (let index = 0; index < 3; index += 1) {
    if (a[index] !== b[index]) return a[index] - b[index];
  }
  return 0;
}

function stableVersions(metadata) {
  return Object.keys(metadata.versions || {})
    .filter((version) => parseStableVersion(version))
    .sort(compareVersions);
}

function minimumReleaseAgeMinutes(workspaceText) {
  const match = /^minimumReleaseAge:\s*(\d+)\s*$/m.exec(workspaceText);
  if (!match) return DEFAULT_MINIMUM_RELEASE_AGE_MINUTES;
  const value = Number(match[1]);
  if (!Number.isSafeInteger(value) || value < 0)
    throw new Error('pnpm minimumReleaseAge must be a non-negative integer.');
  return value;
}

function minimumAgeExclusions(workspaceText) {
  const section =
    /^minimumReleaseAgeExclude:\s*\n((?:\s+-\s+[^\n]*\n?)*)/m.exec(
      workspaceText,
    );
  if (!section) return [];
  return [...section[1].matchAll(/^\s+-\s+['"]?([^'"\s]+)['"]?\s*$/gm)].map(
    (match) => match[1],
  );
}

function isExcludedFromMinimumAge(packageName, version, exclusions) {
  return exclusions.some(
    (entry) =>
      entry === packageName ||
      entry === `${packageName}@${version}` ||
      entry === `${packageName}@*`,
  );
}

function isOldEnough(publishedAt, minimumAgeMinutes, now, isExcluded) {
  if (isExcluded || minimumAgeMinutes === 0) return true;
  const timestamp = Date.parse(publishedAt || '');
  return (
    Number.isFinite(timestamp) && timestamp <= now - minimumAgeMinutes * 60_000
  );
}

function packageFingerprint(manifestText, lockfileText) {
  return crypto
    .createHash('sha256')
    .update(manifestText)
    .update('\0')
    .update(lockfileText)
    .digest('hex');
}

function packageVersionDelta(beforeLock, afterLock) {
  const before = lockPackageVersions(beforeLock);
  const after = lockPackageVersions(afterLock);
  const names = [...new Set([...before.keys(), ...after.keys()])].sort();
  const changes = [];
  for (const name of names) {
    const oldVersions = before.get(name) || [];
    const newVersions = after.get(name) || [];
    if (JSON.stringify(oldVersions) !== JSON.stringify(newVersions)) {
      changes.push({
        name,
        from: oldVersions,
        to: newVersions,
      });
    }
  }
  return changes;
}

function lockPackageVersions(lockText) {
  const packagesBlock = /^packages:\s*\n([\s\S]*?)^snapshots:\s*$/m.exec(
    lockText,
  );
  if (!packagesBlock) throw new Error('pnpm lockfile has no packages section.');
  const versions = new Map();
  for (const line of packagesBlock[1].split('\n')) {
    const match = /^  (.+):$/.exec(line);
    if (!match) continue;
    const key = unquoteYamlKey(match[1]);
    const separator = key.indexOf(
      '@',
      key.startsWith('@') ? key.indexOf('/') + 1 : 0,
    );
    if (separator <= 0) continue;
    const name = key.slice(0, separator);
    const versionAndPeers = key.slice(separator + 1);
    const version = versionAndPeers.split('(')[0];
    const entries = versions.get(name) || [];
    entries.push(version);
    versions.set(name, entries);
  }
  for (const entries of versions.values())
    entries.sort(function sortVersions(left, right) {
      return compareVersionsIfStable(left, right);
    });
  return versions;
}

function compareVersionsIfStable(left, right) {
  const a = parseStableVersion(left);
  const b = parseStableVersion(right);
  if (a && b) return compareVersions(a.join('.'), b.join('.'));
  return left.localeCompare(right);
}

function unquoteYamlKey(value) {
  if (value.startsWith("'") && value.endsWith("'"))
    return value.slice(1, -1).replaceAll("''", "'");
  if (value.startsWith('"') && value.endsWith('"')) {
    try {
      return JSON.parse(value);
    } catch {}
  }
  return value;
}

function writeCandidateManifest(candidateVersion, root = process.cwd()) {
  const packagePath = path.join(root, 'package.json');
  const manifest = JSON.parse(fs.readFileSync(packagePath, 'utf8'));
  for (const packageName of PI_PACKAGES) {
    if (!manifest.dependencies || !manifest.dependencies[packageName])
      throw new Error(
        `Expected direct dependency ${packageName} in package.json.`,
      );
    manifest.dependencies[packageName] = candidateVersion;
  }
  fs.writeFileSync(packagePath, `${JSON.stringify(manifest, null, 2)}\n`);
}

function validateManifestChanges(
  baseManifestText,
  candidateManifestText,
  baseVersions,
  candidateVersions,
) {
  const baseManifest = JSON.parse(baseManifestText);
  const candidateManifest = JSON.parse(candidateManifestText);
  for (const packageName of PI_PACKAGES) {
    if (baseManifest.dependencies?.[packageName] !== baseVersions[packageName])
      throw new Error(
        `Base manifest does not pin ${packageName} to its recorded version.`,
      );
  }
  const expected = structuredClone(baseManifest);
  for (const packageName of PI_PACKAGES)
    expected.dependencies[packageName] = candidateVersions[packageName];
  if (JSON.stringify(candidateManifest) !== JSON.stringify(expected))
    throw new Error(
      'Candidate manifest changed fields beyond the two exact Pi pins.',
    );
  for (const packageName of PI_PACKAGES) {
    if (
      !parseStableVersion(candidateManifest.dependencies[packageName]) ||
      candidateManifest.dependencies[packageName] !==
        candidateVersions[packageName]
    )
      throw new Error(
        `Candidate pin for ${packageName} is not a stable exact version.`,
      );
  }
}

function validateLockfile(lockText, versions, baselineLockText = '') {
  const importer = /^  \.?:\s*\n([\s\S]*?)(?=^  [^ ]|^packages:)/m.exec(
    lockText,
  );
  if (!importer) throw new Error('pnpm lockfile root importer is missing.');
  for (const packageName of PI_PACKAGES) {
    const escaped = packageName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const entry = new RegExp(
      `^      ['"]?${escaped}['"]?:\\n        specifier: ['"]?${versions[packageName]}['"]?\\n        version: ['"]?${versions[packageName]}(?:\\([^\\n]*\\))?['"]?\\s*$`,
      'm',
    );
    if (!entry.test(importer[1]))
      throw new Error(
        `Lockfile root importer does not resolve ${packageName} to the candidate pin.`,
      );
  }
  const resolutions = packageResolutions(lockText);
  const baselineResolutions = packageResolutions(baselineLockText);
  for (const [packageKey, resolution] of resolutions) {
    const hasGitOrLocalSource = /\b(?:file|link|git|github):|git\+/i.test(
      resolution,
    );
    const urls = [...resolution.matchAll(/https?:\/\/[^,\s}]+/gi)].map(
      (match) => match[0],
    );
    const hasNonRegistryUrl = urls.some(
      (url) => !url.startsWith('https://registry.npmjs.org/'),
    );
    if (
      (hasGitOrLocalSource || hasNonRegistryUrl) &&
      baselineResolutions.get(packageKey) !== resolution
    )
      throw new Error(
        `Lockfile contains a new non-registry package resolution: ${resolution.trim()}`,
      );
  }
}

function packageResolutions(lockText) {
  const packagesBlock = /^packages:\s*\n([\s\S]*?)^snapshots:\s*$/m.exec(
    lockText,
  );
  if (!packagesBlock) return new Map();
  const resolutions = new Map();
  let packageKey;
  for (const line of packagesBlock[1].split('\n')) {
    const packageHeader = /^  (.+):$/.exec(line);
    if (packageHeader) {
      packageKey = unquoteYamlKey(packageHeader[1]);
      continue;
    }
    const resolution = /^    resolution:\s*(.*)$/.exec(line);
    if (packageKey && resolution) resolutions.set(packageKey, resolution[1]);
  }
  return resolutions;
}

module.exports = {
  DEFAULT_MINIMUM_RELEASE_AGE_MINUTES,
  PACKAGE_FILES,
  PI_PACKAGES,
  compareVersions,
  isExcludedFromMinimumAge,
  isOldEnough,
  lockPackageVersions,
  minimumAgeExclusions,
  minimumReleaseAgeMinutes,
  packageFingerprint,
  packageVersionDelta,
  packageResolutions,
  parseStableVersion,
  stableVersions,
  validateLockfile,
  validateManifestChanges,
  writeCandidateManifest,
};

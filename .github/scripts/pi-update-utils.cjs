const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { parse: parseYaml } = require('yaml');

const PI_PACKAGES = [
  '@earendil-works/pi-ai',
  '@earendil-works/pi-coding-agent',
];
const PI_WEB_ACCESS_PACKAGE = 'pi-web-access';
const PI_WEB_ACCESS_PEERS = [...PI_PACKAGES, '@earendil-works/pi-tui'];
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
  const lockfile = parsePnpmLockfile(lockText);
  const versions = new Map();
  for (const key of Object.keys(lockfile.packages)) {
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
  const lockfile = parsePnpmLockfile(lockText);
  const rootImporter = lockfile.importers['.'];
  if (!isRecord(rootImporter))
    throw new Error('pnpm lockfile root importer is missing.');
  for (const packageName of PI_PACKAGES) {
    const pin = rootImporter.dependencies?.[packageName];
    if (
      !isRecord(pin) ||
      pin.specifier !== versions[packageName] ||
      !lockVersionMatches(pin.version, versions[packageName])
    )
      throw new Error(
        `Lockfile root importer does not resolve ${packageName} to the candidate pin.`,
      );
  }

  let baseline;
  if (baselineLockText) {
    baseline = parsePnpmLockfile(baselineLockText);
    validateUnchangedLockfilePolicy(lockfile, baseline);
  }

  const baselinePackages = baseline?.packages || {};
  for (const [packageKey, packageRecord] of Object.entries(lockfile.packages)) {
    if (isNonRegistryPackage(packageKey, packageRecord)) {
      const baselineRecord = baselinePackages[packageKey];
      if (
        !baselineRecord ||
        stableJson(baselineRecord) !== stableJson(packageRecord)
      )
        throw new Error(
          `Lockfile contains a new non-registry package resolution for ${packageKey}.`,
        );
    }
  }
}

function packageResolutions(lockText) {
  return new Map(
    Object.entries(parsePnpmLockfile(lockText).packages).map(
      ([packageKey, record]) => [packageKey, record?.resolution],
    ),
  );
}

function parsePnpmLockfile(lockText) {
  let lockfile;
  try {
    lockfile = parseYaml(lockText, { uniqueKeys: true });
  } catch (error) {
    throw new Error('pnpm lockfile is invalid YAML.', { cause: error });
  }
  if (
    !isRecord(lockfile) ||
    !isRecord(lockfile.importers) ||
    !isRecord(lockfile.packages) ||
    !isRecord(lockfile.snapshots)
  )
    throw new Error('pnpm lockfile is missing required data sections.');
  return lockfile;
}

function isRecord(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function stableJson(value) {
  return JSON.stringify(canonicalize(value));
}

function canonicalize(value) {
  if (Array.isArray(value)) return value.map(canonicalize);
  if (!isRecord(value)) return value;
  return Object.fromEntries(
    Object.keys(value)
      .sort()
      .map((key) => [key, canonicalize(value[key])]),
  );
}

function validateUnchangedLockfilePolicy(candidate, baseline) {
  const candidatePolicy = { ...candidate };
  const baselinePolicy = { ...baseline };
  for (const document of [candidatePolicy, baselinePolicy]) {
    delete document.importers;
    delete document.packages;
    delete document.snapshots;
  }
  if (stableJson(candidatePolicy) !== stableJson(baselinePolicy))
    throw new Error('Candidate lockfile changed pnpm policy or metadata.');

  validatePiWebAccessPeerUpdate(candidate, baseline);

  const candidateImporters = importerPolicyView(candidate.importers);
  const baselineImporters = importerPolicyView(baseline.importers);
  for (const importers of [candidateImporters, baselineImporters]) {
    const dependencies = importers['.']?.dependencies;
    if (!isRecord(dependencies)) continue;
    for (const packageName of PI_PACKAGES) {
      const pin = dependencies[packageName];
      if (!isRecord(pin)) continue;
      delete pin.specifier;
      delete pin.version;
      if (Object.keys(pin).length === 0) delete dependencies[packageName];
    }
  }
  const candidateWeb =
    candidateImporters['.']?.dependencies?.[PI_WEB_ACCESS_PACKAGE];
  const baselineWeb =
    baselineImporters['.']?.dependencies?.[PI_WEB_ACCESS_PACKAGE];
  if (isRecord(candidateWeb) && isRecord(baselineWeb))
    candidateWeb.version = baselineWeb.version;
  if (stableJson(candidateImporters) !== stableJson(baselineImporters))
    throw new Error(
      'Candidate lockfile changed importers beyond the two direct Pi pins.',
    );
}

function importerPolicyView(importers) {
  return structuredClone(importers);
}

function validatePiWebAccessPeerUpdate(candidate, baseline) {
  const candidateImporter = candidate.importers['.'];
  const baselineImporter = baseline.importers['.'];
  const candidateWeb = candidateImporter?.dependencies?.[PI_WEB_ACCESS_PACKAGE];
  const baselineWeb = baselineImporter?.dependencies?.[PI_WEB_ACCESS_PACKAGE];
  if (candidateWeb === undefined && baselineWeb === undefined) return;
  if (!isRecord(candidateWeb) || !isRecord(baselineWeb))
    throw new Error('Candidate lockfile changed the pi-web-access importer.');
  if (candidateWeb.specifier !== baselineWeb.specifier)
    throw new Error('Candidate lockfile changed the pi-web-access specifier.');

  const candidateResolution = splitPnpmPeerResolution(candidateWeb.version);
  const baselineResolution = splitPnpmPeerResolution(baselineWeb.version);
  if (candidateResolution.baseVersion !== baselineResolution.baseVersion)
    throw new Error('Candidate lockfile changed the pi-web-access version.');

  const candidatePackageKey = `${PI_WEB_ACCESS_PACKAGE}@${candidateResolution.baseVersion}`;
  const baselinePackageKey = `${PI_WEB_ACCESS_PACKAGE}@${baselineResolution.baseVersion}`;
  const candidatePackage = candidate.packages[candidatePackageKey];
  const baselinePackage = baseline.packages[baselinePackageKey];
  if (
    !isRecord(candidatePackage) ||
    !isRecord(baselinePackage) ||
    stableJson(candidatePackage) !== stableJson(baselinePackage)
  )
    throw new Error(
      'Candidate lockfile changed the pi-web-access package source or metadata.',
    );

  const expectedCandidatePeers = piWebAccessPeerVersions(candidate);
  const expectedBaselinePeers = piWebAccessPeerVersions(baseline);
  validatePiPeerContexts(
    candidateResolution,
    baselineResolution,
    candidatePackage,
    expectedCandidatePeers,
    expectedBaselinePeers,
  );

  const candidateSnapshot =
    candidate.snapshots[`${PI_WEB_ACCESS_PACKAGE}@${candidateWeb.version}`];
  const baselineSnapshot =
    baseline.snapshots[`${PI_WEB_ACCESS_PACKAGE}@${baselineWeb.version}`];
  if (!isRecord(candidateSnapshot) || !isRecord(baselineSnapshot))
    throw new Error(
      'Candidate lockfile is missing a pi-web-access peer snapshot.',
    );
  if (
    stableJson(
      snapshotWithoutPiWebAccessPeerResolutions(
        candidateSnapshot,
        expectedCandidatePeers,
      ),
    ) !==
    stableJson(
      snapshotWithoutPiWebAccessPeerResolutions(
        baselineSnapshot,
        expectedBaselinePeers,
      ),
    )
  )
    throw new Error(
      'Candidate lockfile changed unrelated pi-web-access snapshot dependencies.',
    );
}

function splitPnpmPeerResolution(value) {
  if (typeof value !== 'string' || value.length === 0)
    throw new Error(
      'Candidate lockfile has an invalid pi-web-access resolution.',
    );
  const firstPeer = value.indexOf('(');
  const baseVersion = firstPeer < 0 ? value : value.slice(0, firstPeer);
  const peers = new Map();
  let cursor = firstPeer < 0 ? value.length : firstPeer;
  while (cursor < value.length) {
    if (value[cursor] !== '(')
      throw new Error(
        'Candidate lockfile has a malformed pnpm peer resolution.',
      );
    let depth = 0;
    let end = cursor;
    for (; end < value.length; end += 1) {
      if (value[end] === '(') depth += 1;
      else if (value[end] === ')') {
        depth -= 1;
        if (depth === 0) break;
        if (depth < 0) break;
      }
    }
    if (depth !== 0 || end === cursor + 1)
      throw new Error(
        'Candidate lockfile has an unbalanced pnpm peer resolution.',
      );
    const peerPath = value.slice(cursor + 1, end);
    const packageName = packageNameInPeerPath(peerPath);
    if (peers.has(packageName))
      throw new Error(
        `Candidate lockfile repeats the ${packageName} peer context.`,
      );
    peers.set(packageName, peerPath);
    cursor = end + 1;
  }
  return { baseVersion, peers };
}

function packageNameInPeerPath(peerPath) {
  const packageVersionSeparator = peerPath.startsWith('@')
    ? peerPath.indexOf('@', peerPath.indexOf('/') + 1)
    : peerPath.indexOf('@');
  if (packageVersionSeparator <= 0)
    throw new Error(
      'Candidate lockfile has an invalid pnpm peer package path.',
    );
  return peerPath.slice(0, packageVersionSeparator);
}

function piWebAccessPeerVersions(lockfile) {
  const rootImporter = lockfile.importers['.'];
  const dependencies = rootImporter?.dependencies;
  if (!isRecord(dependencies))
    throw new Error('Candidate lockfile has no root importer dependencies.');
  const peerVersions = new Map();
  for (const packageName of PI_PACKAGES) {
    const pin = dependencies[packageName];
    if (!isRecord(pin) || typeof pin.version !== 'string')
      throw new Error(
        `Candidate lockfile is missing the ${packageName} Pi peer pin.`,
      );
    peerVersions.set(packageName, pin.version);
  }
  const codingAgentPin = dependencies[PI_PACKAGES[1]];
  const codingAgentSnapshot =
    lockfile.snapshots[`${PI_PACKAGES[1]}@${codingAgentPin.version}`];
  const tuiVersion =
    codingAgentSnapshot?.dependencies?.['@earendil-works/pi-tui'] ??
    codingAgentSnapshot?.optionalDependencies?.['@earendil-works/pi-tui'];
  if (typeof tuiVersion !== 'string')
    throw new Error(
      'Candidate lockfile is missing the Pi TUI dependency path.',
    );
  peerVersions.set('@earendil-works/pi-tui', tuiVersion);
  return peerVersions;
}

function validatePiPeerContexts(
  candidateResolution,
  baselineResolution,
  webPackage,
  candidatePeerVersions,
  baselinePeerVersions,
) {
  const expectedPeers = new Set(Object.keys(webPackage.peerDependencies || {}));
  if (!sameStringSet(expectedPeers, new Set(candidateResolution.peers.keys())))
    throw new Error('Candidate lockfile changed the pi-web-access peer set.');
  if (!sameStringSet(expectedPeers, new Set(baselineResolution.peers.keys())))
    throw new Error('Baseline lockfile has an invalid pi-web-access peer set.');
  for (const peerName of expectedPeers) {
    const candidatePath = candidateResolution.peers.get(peerName);
    const baselinePath = baselineResolution.peers.get(peerName);
    if (PI_WEB_ACCESS_PEERS.includes(peerName)) {
      if (
        candidatePath !==
          `${peerName}@${candidatePeerVersions.get(peerName)}` ||
        baselinePath !== `${peerName}@${baselinePeerVersions.get(peerName)}`
      )
        throw new Error(
          `Candidate lockfile changed the ${peerName} peer context incoherently.`,
        );
    } else if (candidatePath !== baselinePath) {
      throw new Error(
        `Candidate lockfile changed an unrelated ${peerName} peer context.`,
      );
    }
  }
}

function sameStringSet(left, right) {
  return (
    left.size === right.size && [...left].every((value) => right.has(value))
  );
}

function snapshotWithoutPiWebAccessPeerResolutions(
  snapshot,
  expectedPeerVersions,
) {
  const normalized = structuredClone(snapshot);
  for (const peerName of PI_WEB_ACCESS_PEERS) {
    let occurrences = 0;
    for (const sectionName of ['dependencies', 'optionalDependencies']) {
      const section = normalized[sectionName];
      if (!isRecord(section) || !Object.hasOwn(section, peerName)) continue;
      if (section[peerName] !== expectedPeerVersions.get(peerName))
        throw new Error(
          `Candidate lockfile has an incoherent ${peerName} peer snapshot.`,
        );
      delete section[peerName];
      occurrences += 1;
    }
    if (occurrences !== 1)
      throw new Error(
        `Candidate lockfile must contain one ${peerName} peer snapshot.`,
      );
  }
  return normalized;
}

function lockVersionMatches(actual, expected) {
  return (
    actual === expected ||
    (typeof actual === 'string' &&
      actual.startsWith(`${expected}(`) &&
      actual.endsWith(')'))
  );
}

function isNonRegistryPackage(packageKey, packageRecord) {
  if (packageKeyUsesNonRegistrySource(packageKey)) return true;
  return containsNonRegistrySource(packageRecord?.resolution);
}

function packageKeyUsesNonRegistrySource(packageKey) {
  const separator = packageKey.indexOf(
    '@',
    packageKey.startsWith('@') ? packageKey.indexOf('/') + 1 : 0,
  );
  if (separator < 0) return false;
  const specifier = packageKey.slice(separator + 1);
  if (specifier.startsWith('npm:')) {
    const aliasedPackage = specifier.slice(4).split('(')[0];
    const aliasVersion = aliasedPackage.slice(
      aliasedPackage.lastIndexOf('@') + 1,
    );
    return !isRegistryPackageVersion(aliasVersion);
  }
  const version = specifier.split('(')[0];
  if (isRegistryPackageVersion(version)) return false;
  if (/^https?:\/\//i.test(specifier)) return !isNpmRegistryTarball(specifier);
  return true;
}

function isRegistryPackageVersion(version) {
  return /^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(
    version,
  );
}

function containsNonRegistrySource(value) {
  if (typeof value === 'string') {
    if (/^[A-Za-z][A-Za-z\d+.-]*:/i.test(value))
      return !isNpmRegistryTarball(value);
    return false;
  }
  if (Array.isArray(value))
    return value.some((item) => containsNonRegistrySource(item));
  if (!isRecord(value)) return false;
  if (
    ['directory', 'repo', 'commit', 'git'].some((key) => key in value) ||
    ['tarball', 'url'].some(
      (key) => key in value && !isNpmRegistryTarball(value[key]),
    )
  )
    return true;
  if (
    typeof value.type === 'string' &&
    !['registry', 'tarball'].includes(value.type)
  )
    return true;
  for (const [key, nestedValue] of Object.entries(value)) {
    if (containsNonRegistrySource(nestedValue)) return true;
  }
  return false;
}

function isNpmRegistryTarball(value) {
  if (typeof value !== 'string') return false;
  try {
    const url = new URL(value);
    return (
      url.protocol === 'https:' &&
      url.hostname === 'registry.npmjs.org' &&
      !url.username &&
      !url.password
    );
  } catch {
    return false;
  }
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

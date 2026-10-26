const { execFileSync } = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');
const {
  PACKAGE_FILES,
  PI_PACKAGES,
  compareVersions,
  isExcludedFromMinimumAge,
  isOldEnough,
  minimumAgeExclusions,
  minimumReleaseAgeMinutes,
  packageFingerprint,
  packageVersionDelta,
  parseStableVersion,
  stableVersions,
  validateLockfile,
  validateManifestChanges,
  writeCandidateManifest,
} = require('./pi-update-utils.cjs');

async function readRegistryPackage(packageName) {
  const registryUrl = `https://registry.npmjs.org/${encodeURIComponent(packageName)}`;
  const response = await fetch(registryUrl, {
    headers: { accept: 'application/json' },
  });
  if (!response.ok)
    throw new Error(
      `Registry metadata for ${packageName} returned ${response.status}.`,
    );
  const metadata = await response.json();
  if (
    !metadata ||
    typeof metadata.versions !== 'object' ||
    typeof metadata.time !== 'object'
  )
    throw new Error(`Registry metadata for ${packageName} is incomplete.`);
  return metadata;
}

function currentPiPins(manifest) {
  const current = {};
  for (const packageName of PI_PACKAGES) {
    const version = manifest.dependencies?.[packageName];
    if (!parseStableVersion(version))
      throw new Error(
        `Installed Pi pin ${packageName} must be an exact stable version.`,
      );
    current[packageName] = version;
  }
  if (current[PI_PACKAGES[0]] !== current[PI_PACKAGES[1]])
    throw new Error(
      'Installed Pi packages must use one coherent exact version tuple.',
    );
  return current;
}

async function coherentFamilyMetadata(candidateVersion, packageMetadataByName) {
  const familyPackages = new Set();
  for (const packageName of PI_PACKAGES) {
    const manifest =
      packageMetadataByName.get(packageName)?.versions?.[candidateVersion];
    if (!manifest)
      throw new Error(`${packageName}@${candidateVersion} is not published.`);
    for (const dependencyName of Object.keys({
      ...manifest.dependencies,
      ...manifest.optionalDependencies,
    }))
      if (dependencyName.startsWith('@earendil-works/pi-'))
        familyPackages.add(dependencyName);
  }

  for (const packageName of familyPackages) {
    let metadata = packageMetadataByName.get(packageName);
    if (!metadata) {
      metadata = await readRegistryPackage(packageName);
      packageMetadataByName.set(packageName, metadata);
    }
    const manifest = metadata.versions[candidateVersion];
    if (!manifest || manifest.version !== candidateVersion)
      return { eligible: false, packageName };
    for (const dependencyName of Object.keys({
      ...manifest.dependencies,
      ...manifest.optionalDependencies,
    })) {
      if (
        dependencyName.startsWith('@earendil-works/pi-') &&
        manifest.dependencies?.[dependencyName] !== candidateVersion &&
        manifest.optionalDependencies?.[dependencyName] !== candidateVersion
      )
        return { eligible: false, packageName: dependencyName };
    }
  }
  return { eligible: true, packages: [...familyPackages].sort() };
}

async function resolveCandidate(root = process.cwd(), now = Date.now()) {
  const manifest = JSON.parse(
    await fs.readFile(path.join(root, 'package.json'), 'utf8'),
  );
  const current = currentPiPins(manifest);
  const workspaceText = await fs.readFile(
    path.join(root, 'pnpm-workspace.yaml'),
    'utf8',
  );
  const minimumAge = minimumReleaseAgeMinutes(workspaceText);
  const exclusions = minimumAgeExclusions(workspaceText);
  const packageMetadataByName = new Map();
  for (const packageName of PI_PACKAGES)
    packageMetadataByName.set(
      packageName,
      await readRegistryPackage(packageName),
    );

  const commonVersions = stableVersions(
    packageMetadataByName.get(PI_PACKAGES[0]),
  ).filter(
    (version) => packageMetadataByName.get(PI_PACKAGES[1]).versions[version],
  );
  for (let index = commonVersions.length - 1; index >= 0; index -= 1) {
    const version = commonVersions[index];
    if (compareVersions(version, current[PI_PACKAGES[0]]) <= 0) return null;
    const family = await coherentFamilyMetadata(version, packageMetadataByName);
    if (!family.eligible) continue;
    const ageEligible = PI_PACKAGES.every((packageName) =>
      isOldEnough(
        packageMetadataByName.get(packageName).time[version],
        minimumAge,
        now,
        isExcludedFromMinimumAge(packageName, version, exclusions),
      ),
    );
    const familyAgeEligible = family.packages.every((packageName) =>
      isOldEnough(
        packageMetadataByName.get(packageName).time[version],
        minimumAge,
        now,
        isExcludedFromMinimumAge(packageName, version, exclusions),
      ),
    );
    if (!ageEligible || !familyAgeEligible) continue;
    const tarballs = [...PI_PACKAGES, ...family.packages].map(
      (packageName) =>
        packageMetadataByName.get(packageName).versions[version]?.dist?.tarball,
    );
    if (
      tarballs.some(
        (tarball) =>
          typeof tarball !== 'string' ||
          !tarball.startsWith('https://registry.npmjs.org/'),
      )
    )
      throw new Error('A coherent Pi release includes a non-registry tarball.');
    return {
      baseVersions: current,
      candidateVersions: Object.fromEntries(
        PI_PACKAGES.map((packageName) => [packageName, version]),
      ),
      familyPackages: family.packages,
      minimumReleaseAgeMinutes: minimumAge,
    };
  }
  return null;
}

function gitFileAtCommit(baseSha, filePath) {
  return execFileSync('git', ['show', `${baseSha}:${filePath}`], {
    encoding: 'utf8',
  });
}

async function resolveAndWrite(candidateFile, root = process.cwd()) {
  const candidate = await resolveCandidate(root);
  if (!candidate) {
    await fs.writeFile(candidateFile, JSON.stringify({ hasUpdate: false }));
    if (process.env.GITHUB_OUTPUT)
      await fs.appendFile(process.env.GITHUB_OUTPUT, 'has_update=false\n');
    return null;
  }
  const version = candidate.candidateVersions[PI_PACKAGES[0]];
  writeCandidateManifest(version, root);
  await fs.writeFile(
    candidateFile,
    `${JSON.stringify({ hasUpdate: true, ...candidate }, null, 2)}\n`,
  );
  if (process.env.GITHUB_OUTPUT)
    await fs.appendFile(process.env.GITHUB_OUTPUT, 'has_update=true\n');
  return version;
}

async function snapshotCandidate(
  candidateFile,
  outputDirectory,
  root = process.cwd(),
) {
  const candidate = JSON.parse(await fs.readFile(candidateFile, 'utf8'));
  if (candidate.hasUpdate !== true) return false;
  const baseSha = process.env.PI_UPDATE_BASE_SHA || process.env.GITHUB_SHA;
  if (
    !baseSha ||
    !parseStableVersion(candidate.candidateVersions?.[PI_PACKAGES[0]])
  )
    throw new Error(
      'Candidate snapshot is missing its base SHA or stable version tuple.',
    );
  for (const packageName of PI_PACKAGES) {
    if (
      candidate.candidateVersions[packageName] !==
      candidate.candidateVersions[PI_PACKAGES[0]]
    )
      throw new Error(
        'Candidate Pi packages are not pinned to one coherent version.',
      );
  }
  const baseManifestText = gitFileAtCommit(baseSha, 'package.json');
  const baseLockText = gitFileAtCommit(baseSha, 'pnpm-lock.yaml');
  const manifestText = await fs.readFile(
    path.join(root, 'package.json'),
    'utf8',
  );
  const lockText = await fs.readFile(path.join(root, 'pnpm-lock.yaml'), 'utf8');
  validateManifestChanges(
    baseManifestText,
    manifestText,
    candidate.baseVersions,
    candidate.candidateVersions,
  );
  validateLockfile(lockText, candidate.candidateVersions, baseLockText);
  const changedPaths = execFileSync('git', ['diff', '--name-only', baseSha], {
    encoding: 'utf8',
  })
    .split('\n')
    .filter(Boolean)
    .sort();
  const untrackedPaths = execFileSync(
    'git',
    ['ls-files', '--others', '--exclude-standard'],
    { encoding: 'utf8' },
  )
    .split('\n')
    .filter(Boolean)
    .sort();
  const actualChangedPaths = [...changedPaths, ...untrackedPaths].sort();
  if (
    JSON.stringify(actualChangedPaths) !==
    JSON.stringify([...PACKAGE_FILES].sort())
  )
    throw new Error(
      'Candidate install must change only package.json and pnpm-lock.yaml.',
    );

  await fs.mkdir(outputDirectory, { recursive: true });
  await fs.writeFile(path.join(outputDirectory, 'package.json'), manifestText);
  await fs.writeFile(path.join(outputDirectory, 'pnpm-lock.yaml'), lockText);
  const metadata = {
    schemaVersion: 1,
    repository: process.env.GITHUB_REPOSITORY,
    runId: process.env.GITHUB_RUN_ID,
    runAttempt: process.env.GITHUB_RUN_ATTEMPT,
    baseSha,
    workflowSha: baseSha,
    baseVersions: candidate.baseVersions,
    candidateVersions: candidate.candidateVersions,
    familyPackages: candidate.familyPackages,
    minimumReleaseAgeMinutes: candidate.minimumReleaseAgeMinutes,
    fingerprint: packageFingerprint(manifestText, lockText),
    transitiveDelta: packageVersionDelta(baseLockText, lockText),
  };
  await fs.writeFile(
    path.join(outputDirectory, 'metadata.json'),
    `${JSON.stringify(metadata, null, 2)}\n`,
  );
  return true;
}

async function main(args) {
  if (args[0] === '--snapshot') {
    const copied = await snapshotCandidate(args[1], args[2]);
    console.log(`snapshot=${copied}`);
    return;
  }
  const candidateFile = args[0];
  if (!candidateFile)
    throw new Error('Pass a task-local candidate metadata file path.');
  const version = await resolveAndWrite(candidateFile);
  console.log(`has_update=${version ? 'true' : 'false'}`);
  if (version) console.log(`candidate_version=${version}`);
}

if (require.main === module) {
  main(process.argv.slice(2)).catch(function reportResolveFailure(error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}

module.exports = {
  coherentFamilyMetadata,
  currentPiPins,
  resolveCandidate,
  snapshotCandidate,
};

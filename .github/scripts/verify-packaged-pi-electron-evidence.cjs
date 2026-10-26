const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { fileURLToPath } = require('node:url');
const asar = require('@electron/asar');
const {
  lockedPiPackageVersion,
  lockfileImporterSpecifier,
} = require('./verify-pi-integration-evidence.cjs');

const REQUIRED_TOOLS = [
  'fetch_content',
  'find',
  'grep',
  'ls',
  'read',
  'web_search',
];
const PACKAGED_PI_PACKAGES = [
  '@earendil-works/pi-coding-agent',
  '@earendil-works/pi-ai',
  'pi-web-access',
];

function readArchiveManifest(archive, directory) {
  const entry = directory ? `${directory}/package.json` : 'package.json';
  return JSON.parse(asar.extractFile(archive, entry).toString());
}

function archivePackageVersions(archive) {
  return Object.fromEntries(
    PACKAGED_PI_PACKAGES.map(function packageVersion(name) {
      return [
        name,
        readArchiveManifest(archive, `node_modules/${name}`).version,
      ];
    }),
  );
}

function runtimePathWithinArchive(runtimePath, archive) {
  if (typeof runtimePath !== 'string' || !runtimePath.startsWith('file:'))
    return false;
  const resolvedPath = fileURLToPath(runtimePath);
  const relative = path.relative(path.resolve(archive), resolvedPath);
  return (
    relative !== '' &&
    relative !== '..' &&
    !relative.startsWith(`..${path.sep}`) &&
    !path.isAbsolute(relative) &&
    relative.split(path.sep)[0] === 'node_modules'
  );
}

function samePath(left, right) {
  const resolvedLeft = path.resolve(left);
  const resolvedRight = path.resolve(right);
  return process.platform === 'win32'
    ? resolvedLeft.toLowerCase() === resolvedRight.toLowerCase()
    : resolvedLeft === resolvedRight;
}

function verifyPackagedPiElectronEvidence(
  directory,
  expectedSha,
  expectedArchitecture,
) {
  const evidencePath = path.join(directory, 'pi-electron-evidence.json');
  if (!fs.existsSync(evidencePath))
    throw new Error(
      `Packaged Pi Electron evidence is missing: ${evidencePath}`,
    );
  const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
  const repository = path.resolve(__dirname, '../..');
  const manifest = JSON.parse(
    fs.readFileSync(path.join(repository, 'package.json'), 'utf8'),
  );
  const lockfile = fs.readFileSync(
    path.join(repository, 'pnpm-lock.yaml'),
    'utf8',
  );
  assert.equal(typeof evidence.appPath, 'string');
  assert.ok(path.isAbsolute(evidence.appPath));
  const archive = path.resolve(String(evidence.appPath || ''));
  assert.ok(fs.existsSync(archive), 'running Electron app.asar must exist');
  assert.equal(path.basename(archive), 'app.asar');
  assert.equal(evidence.commitSha, expectedSha);
  assert.equal(typeof evidence.runId, 'string');
  assert.equal(evidence.architecture, expectedArchitecture);
  assert.equal(evidence.runtimeBoundary, 'packaged-electron-nevermind-adapter');
  assert.equal(evidence.runtime, 'electron');
  assert.equal(evidence.electronRuntime, true);
  assert.equal(evidence.adapterIntegration, true);
  assert.equal(evidence.appIsPackaged, true);
  assert.ok(samePath(evidence.appPath, archive));
  assert.equal(evidence.finalOutcome, 'done');
  assert.deepEqual(evidence.terminalEvents, ['done']);
  assert.equal(evidence.responseMatches, true);
  assert.equal(evidence.sessionDisposed, true);
  assert.equal(evidence.provider, 'loopback-openai-completions');
  assert.equal(evidence.providerApiFormat, 'openai-completions');
  assert.equal(evidence.providerCalls, 1);
  assert.deepEqual(evidence.unexpectedRoutes, []);
  assert.equal(evidence.providerPaths.length, 1);
  assert.deepEqual(evidence.providerPaths, ['/v1/chat/completions']);
  assert.deepEqual(evidence.toolNames, REQUIRED_TOOLS);
  assert.equal(evidence.managedHeadersPreserved, true);

  const actualPackageVersions = archivePackageVersions(archive);
  assert.deepEqual(evidence.packageVersions, actualPackageVersions);
  for (const packageName of PACKAGED_PI_PACKAGES) {
    const manifestVersion = manifest.dependencies[packageName];
    assert.equal(actualPackageVersions[packageName], manifestVersion);
    assert.equal(
      lockedPiPackageVersion(lockfile, packageName),
      manifestVersion,
      `${packageName} in app.asar must match the locked package version`,
    );
    assert.equal(
      lockfileImporterSpecifier(lockfile, packageName),
      manifestVersion,
      `${packageName} must have an exact lockfile importer pin`,
    );
  }

  for (const moduleName of ['piCodingAgent', 'piAi', 'piWebAccess'])
    assert.ok(
      runtimePathWithinArchive(evidence.resolvedModules?.[moduleName], archive),
      `${moduleName} must resolve from the running app.asar`,
    );
  return evidence;
}

if (require.main === module) {
  try {
    const evidence = verifyPackagedPiElectronEvidence(
      path.resolve(process.argv[2]),
      process.argv[3],
      process.argv[4],
    );
    console.log(JSON.stringify(evidence));
  } catch (error) {
    console.error(error);
    process.exitCode = 1;
  }
}

module.exports = { verifyPackagedPiElectronEvidence };

#!/usr/bin/env node
'use strict';

const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const {
  evaluateCiGate,
  evaluateDeploymentGate,
  evaluateProductionIdentity,
  evaluateScheduleGate,
} = require('./nightly-release-gates.cjs');
const {
  CHECKSUMS,
  NIGHTLY_UPDATER_METADATA,
  checksumsFor,
  isReleaseAsset,
  retargetUpdaterMetadata,
  validateNightlyAssetSet,
  verifyChecksums,
  verifyUpdaterMetadata,
} = require('./nightly-assets.cjs');

function parseArguments(argv) {
  const [command, ...rest] = argv;
  const options = {};
  for (let index = 0; index < rest.length; index += 2) {
    const key = rest[index];
    if (!key?.startsWith('--') || rest[index + 1] === undefined)
      throw new Error(`Malformed arguments near ${key ?? '(end)'}`);
    options[key.slice(2)] = rest[index + 1];
  }
  return { command, options };
}

function requireOption(options, name) {
  if (!options[name]) throw new Error(`Missing --${name}`);
  return options[name];
}

function readJsonFile(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'));
}

function readJsonFileIfPresent(filePath) {
  if (!fs.existsSync(filePath)) return null;
  try {
    return readJsonFile(filePath);
  } catch {
    return null;
  }
}

function gh(args) {
  return execFileSync('gh', args, { encoding: 'utf8' }).trim();
}

function ghStatusOrNull(args) {
  try {
    return gh(args);
  } catch {
    return null;
  }
}

function repositoryName() {
  return requireEnvironment('GITHUB_REPOSITORY');
}

function requireEnvironment(name) {
  const value = process.env[name];
  if (!value) throw new Error(`Missing environment variable ${name}`);
  return value;
}

function commandScheduleGate(options) {
  return evaluateScheduleGate({
    event: requireOption(options, 'event'),
    now: new Date(requireOption(options, 'now')),
    scheduledCron: options.cron,
  });
}

function commandCiGate(options) {
  return evaluateCiGate({
    sha: requireOption(options, 'sha'),
    runs: readJsonFile(requireOption(options, 'runs')),
  });
}

function commandIdentity(options) {
  const sha = requireOption(options, 'sha');
  const health = readJsonFileIfPresent(requireOption(options, 'health'));
  const verdict = evaluateProductionIdentity({ sha, health });
  if (verdict.status === 'rejected') return verdict;
  const repo = repositoryName();
  const mainSha =
    verdict.status === 'ancestry-required' &&
    ghStatusOrNull([
      'api',
      `repos/${repo}/git/ref/heads/main`,
      '--jq',
      '.object.sha',
    ]);
  const descendsStatus =
    verdict.status === 'ancestry-required' &&
    ghStatusOrNull([
      'api',
      `repos/${repo}/compare/${sha}...${verdict.commit}`,
      '--jq',
      '.status',
    ]);
  const onMainStatus = mainSha
    ? ghStatusOrNull([
        'api',
        `repos/${repo}/compare/${verdict.commit}...${mainSha}`,
        '--jq',
        '.status',
      ])
    : null;
  const identity =
    verdict.status === 'ancestry-required'
      ? evaluateProductionIdentity({
          sha,
          health,
          descendsStatus: descendsStatus ?? null,
          onMainStatus: onMainStatus ?? null,
        })
      : verdict;
  if (identity.status !== 'verified') return identity;
  const rawRuns = ghStatusOrNull([
    'api',
    `repos/${repo}/actions/workflows/database-migration.yml/runs?head_sha=${identity.commit}&event=workflow_run&per_page=100`,
    '--jq',
    '.workflow_runs',
  ]);
  if (!rawRuns)
    return {
      status: 'rejected',
      commit: null,
      reason: 'production deployment runs unavailable',
    };
  const runs = JSON.parse(rawRuns);
  const latest = runs
    .filter(
      (run) =>
        run.head_sha === identity.commit &&
        run.event === 'workflow_run' &&
        run.head_branch === 'main',
    )
    .sort((left, right) => right.id - left.id)[0];
  const rawJobs = latest
    ? ghStatusOrNull([
        'api',
        `repos/${repo}/actions/runs/${latest.id}/jobs?per_page=100`,
        '--jq',
        '.jobs',
      ])
    : null;
  const deployment = evaluateDeploymentGate({
    commit: identity.commit,
    runs,
    jobs: rawJobs ? JSON.parse(rawJobs) : null,
  });
  return deployment.status === 'verified' ? identity : deployment;
}

function fileNames(directory) {
  return fs
    .readdirSync(directory)
    .filter((name) => fs.statSync(path.join(directory, name)).isFile());
}

function sha256(filePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest();
}

// Checks a staged release: the exact Go asset set, each manifest's version, files, sizes and signatures at the
// release's tag, and SHA256SUMS.
function commandValidateAssets(options) {
  const directory = requireOption(options, 'dir');
  const version = requireOption(options, 'version');
  const tag = requireOption(options, 'tag');
  const names = fileNames(directory);
  validateNightlyAssetSet(names, version);
  for (const metadataName of NIGHTLY_UPDATER_METADATA) {
    verifyUpdaterMetadata({
      name: metadataName,
      text: fs.readFileSync(path.join(directory, metadataName), 'utf8'),
      version,
      repository: repositoryName(),
      tag,
      assetNames: names,
      sha256Of: (assetName) => sha256(path.join(directory, assetName)),
      sizeOf: (assetName) => fs.statSync(path.join(directory, assetName)).size,
      publicKey: options['public-key'],
    });
  }
  verifyChecksums({
    text: fs.readFileSync(path.join(directory, CHECKSUMS), 'utf8'),
    names,
    sha256Hex: (assetName) => sha256(path.join(directory, assetName)).toString('hex'),
  });
  return { valid: true, assetCount: names.length, version, tag };
}

// Gathers the release files of every packaging job's artifact (one directory each under --from) into --dir, refuses
// two different files of one name, and writes the release's SHA256SUMS.
function commandAssembleAssets(options) {
  const from = requireOption(options, 'from');
  const directory = requireOption(options, 'dir');
  fs.mkdirSync(directory, { recursive: true });
  for (const artifact of fs.readdirSync(from).sort()) {
    const artifactDirectory = path.join(from, artifact);
    if (!fs.statSync(artifactDirectory).isDirectory()) continue;
    for (const name of fileNames(artifactDirectory).filter(isReleaseAsset)) {
      const source = path.join(artifactDirectory, name);
      const destination = path.join(directory, name);
      if (fs.existsSync(destination)) {
        if (!fs.readFileSync(source).equals(fs.readFileSync(destination)))
          throw new Error(`${artifact} has another ${name}`);
        continue;
      }
      fs.copyFileSync(source, destination);
    }
  }
  const names = fileNames(directory);
  fs.writeFileSync(
    path.join(directory, CHECKSUMS),
    checksumsFor(names, (name) => sha256(path.join(directory, name)).toString('hex')),
  );
  return { assetCount: names.length + 1 };
}

// Points the manifests in --dir at the release --tag, the dev, nightly or stable release that will hold them.
function commandRetargetMetadata(options) {
  const directory = requireOption(options, 'dir');
  const version = requireOption(options, 'version');
  const tag = requireOption(options, 'tag');
  for (const metadataName of NIGHTLY_UPDATER_METADATA) {
    const file = path.join(directory, metadataName);
    fs.writeFileSync(
      file,
      retargetUpdaterMetadata({
        text: fs.readFileSync(file, 'utf8'),
        repository: repositoryName(),
        version,
        tag,
      }),
    );
  }
  return { tag };
}

const commands = {
  'schedule-gate': commandScheduleGate,
  'ci-gate': commandCiGate,
  identity: commandIdentity,
  'validate-assets': commandValidateAssets,
  'assemble-assets': commandAssembleAssets,
  'retarget-metadata': commandRetargetMetadata,
};

function main() {
  const { command, options } = parseArguments(process.argv.slice(2));
  const handler = commands[command];
  if (!handler)
    throw new Error(
      `Unknown command: ${command}. Expected one of ${Object.keys(commands).join(', ')}`,
    );
  process.stdout.write(`${JSON.stringify(handler(options))}\n`);
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    process.stderr.write(
      `${error instanceof Error ? error.message : String(error)}\n`,
    );
    process.exitCode = 1;
  }
}

module.exports = { parseArguments };

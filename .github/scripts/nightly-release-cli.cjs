#!/usr/bin/env node
'use strict';

const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const {
  classifyNightlyAppChanges,
  isVersionOnlyPackageJsonChange,
} = require('./nightly-app-paths.cjs');
const {
  evaluateCiGate,
  evaluateDeploymentGate,
  evaluateProductionIdentity,
  evaluatePublicationRace,
  evaluateScheduleGate,
} = require('./nightly-release-gates.cjs');
const {
  NIGHTLY_UPDATER_METADATA,
  validateNightlyAssetSet,
  verifyUpdaterMetadata,
} = require('./nightly-assets.cjs');
const { selectNightlyRetention } = require('./nightly-retention.cjs');
const {
  formatNightlyVersion,
  madridDateStamp,
  newestStableRelease,
  nightlyBaseline,
  nightlyTagName,
  parseVersion,
  publishedNightlies,
} = require('./nightly-versioning.cjs');

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

function git(args) {
  return execFileSync('git', args, { encoding: 'utf8' }).trim();
}

function gitShowIfPresent(revision, filePath) {
  try {
    return execFileSync('git', ['show', `${revision}:${filePath}`], {
      encoding: 'utf8',
    });
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

function releasesWithResolvedTags(filePath) {
  const releases = readJsonFile(filePath);
  for (const { release } of publishedNightlies(releases)) {
    const sha = gh([
      'api',
      `repos/${repositoryName()}/commits/${encodeURIComponent(release.tag_name)}`,
      '--jq',
      '.sha',
    ]);
    if (!/^[0-9a-f]{40}$/.test(sha))
      throw new Error(`Cannot resolve nightly tag ${release.tag_name}`);
    release.target_commitish = sha;
  }
  return releases;
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

function commandPlan(options) {
  const sha = requireOption(options, 'sha');
  const now = new Date(requireOption(options, 'now'));
  const releases = releasesWithResolvedTags(requireOption(options, 'releases'));
  const runNumber = Number(requireOption(options, 'run-number'));
  const stableTags = releases
    .filter((release) => !release.draft && !release.prerelease)
    .map((release) => release.tag_name);
  const stable = newestStableRelease(stableTags);
  if (!stable)
    throw new Error('No stable v* tag exists to derive the nightly base');
  const bootstrapCommit = git(['rev-parse', `${stable.tag}^{commit}`]);
  const baseline = nightlyBaseline({ releases, bootstrapCommit });
  const changedPaths = git([
    'diff',
    '--name-only',
    '--no-renames',
    baseline.commit,
    sha,
  ])
    .split('\n')
    .filter(Boolean);
  const classification = classifyNightlyAppChanges(changedPaths, {
    isPackageJsonVersionOnly: (filePath) =>
      isVersionOnlyPackageJsonChange(
        gitShowIfPresent(baseline.commit, filePath),
        gitShowIfPresent(sha, filePath),
      ),
  });
  const base = {
    sourceSha: sha,
    baselineCommit: baseline.commit,
    baselineSource: baseline.source,
    matchedPaths: classification.matched,
  };
  if (!classification.appAffecting)
    return {
      ...base,
      build: false,
      reason: `no app-affecting changes since ${baseline.commit}`,
    };
  const version = formatNightlyVersion({
    baseVersion: stable.version,
    dateStamp: madridDateStamp(now),
    runNumber,
  });
  const race = evaluatePublicationRace({ releases, sha, version });
  if (race.action === 'reject') throw new Error(race.reason);
  if (race.action === 'noop')
    return { ...base, build: false, reason: race.reason };
  return {
    ...base,
    build: true,
    version,
    tag: nightlyTagName(version),
    reason: race.reason,
  };
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
  const deployment = evaluateDeploymentGate({
    commit: identity.commit,
    runs: JSON.parse(rawRuns),
  });
  return deployment.status === 'verified' ? identity : deployment;
}

function commandPublishGuard(options) {
  return evaluatePublicationRace({
    releases: releasesWithResolvedTags(requireOption(options, 'releases')),
    sha: requireOption(options, 'sha'),
    version: requireOption(options, 'version'),
  });
}

function sha512Base64(filePath) {
  return crypto
    .createHash('sha512')
    .update(fs.readFileSync(filePath))
    .digest('base64');
}

function commandValidateAssets(options) {
  const directory = requireOption(options, 'dir');
  const version = requireOption(options, 'version');
  const names = fs
    .readdirSync(directory)
    .filter((name) => fs.statSync(path.join(directory, name)).isFile());
  validateNightlyAssetSet(names, version);
  for (const metadataName of NIGHTLY_UPDATER_METADATA) {
    verifyUpdaterMetadata({
      name: metadataName,
      text: fs.readFileSync(path.join(directory, metadataName), 'utf8'),
      version,
      assetNames: names,
      sha512Of: (assetName) => sha512Base64(path.join(directory, assetName)),
    });
  }
  return { valid: true, assetCount: names.length, version };
}

function commandSetPackageVersion(options) {
  const version = requireOption(options, 'version');
  if (!parseVersion(version)?.nightly)
    throw new Error(`Refusing non-nightly package version: ${version}`);
  const manifestPath = path.join(process.cwd(), 'package.json');
  const manifest = readJsonFile(manifestPath);
  manifest.version = version;
  fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
  return { version };
}

function commandRetention(options) {
  return selectNightlyRetention({
    releases: readJsonFile(requireOption(options, 'releases')),
    now: new Date(requireOption(options, 'now')),
  });
}

const commands = {
  'schedule-gate': commandScheduleGate,
  'ci-gate': commandCiGate,
  plan: commandPlan,
  identity: commandIdentity,
  'publish-guard': commandPublishGuard,
  'validate-assets': commandValidateAssets,
  'set-package-version': commandSetPackageVersion,
  retention: commandRetention,
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

'use strict';

const { compareVersions, parseVersion } = require('./nightly-versioning.cjs');
const { selectNewestDev } = require('./promotion-selection.cjs');
const { evaluateCiGate } = require('./nightly-release-gates.cjs');
const { execFileSync } = require('node:child_process');
const path = require('node:path');

function planNightlyPromotion(releases, isEligible = () => true) {
  if (!Array.isArray(releases)) throw new Error('Release list is unavailable');
  let current = null;
  let currentTag = null;
  for (const release of releases) {
    if (release?.draft !== false || release.prerelease !== true) continue;
    const tag = release.tag_name ?? '';
    const promoted = /^v(\d+\.\d+\.\d+)-nightly$/.exec(tag);
    const legacy = tag.startsWith('v') ? tag.slice(1) : '';
    const version =
      promoted?.[1] ?? (parseVersion(legacy)?.nightly ? legacy : null);
    if (version && (!current || compareVersions(version, current) > 0)) {
      current = version;
      currentTag = tag;
    }
  }
  let remaining = releases;
  let selected = selectNewestDev(remaining, current);
  while (selected && !isEligible(selected)) {
    remaining = remaining.filter(
      (release) => release?.tag_name !== selected.tag,
    );
    selected = selectNewestDev(remaining, current);
  }
  return { current, currentTag, selected };
}

function createEligibilityProbe({ repository, healthFile, gh, identity }) {
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository ?? ''))
    throw new Error('Invalid repository');
  return function isEligible(candidate) {
    const sha = gh(`repos/${repository}/commits/${candidate.tag}`).sha;
    if (!/^[0-9a-f]{40}$/.test(sha ?? ''))
      throw new Error('Invalid dev tag commit');
    const runs = gh(
      `repos/${repository}/actions/workflows/ci.yml/runs?head_sha=${sha}&event=push&per_page=50`,
    ).workflow_runs;
    if (!Array.isArray(runs)) throw new Error('CI runs unavailable');
    if (!evaluateCiGate({ sha, runs }).ok) return false;
    const verdict = identity(sha, healthFile);
    if (verdict?.status === 'verified') return true;
    if (
      verdict?.status === 'rejected' &&
      /compare status (behind|diverged)\)/.test(verdict.reason ?? '')
    )
      return false;
    throw new Error(
      `Cannot verify production identity: ${verdict?.reason ?? 'invalid response'}`,
    );
  };
}

if (require.main === module) {
  try {
    const releases = JSON.parse(
      require('node:fs').readFileSync(process.argv[2], 'utf8'),
    );
    if (process.argv[3] === '--candidates') {
      console.log(JSON.stringify(planNightlyPromotion(releases)));
      process.exit(0);
    }
    if (process.argv[3] !== '--verify' || !process.argv[4])
      throw new Error('Expected --verify health-file');
    const gh = (endpoint) =>
      JSON.parse(execFileSync('gh', ['api', endpoint], { encoding: 'utf8' }));
    const identity = (sha, healthFile) =>
      JSON.parse(
        execFileSync(
          process.execPath,
          [
            path.join(__dirname, 'nightly-release-cli.cjs'),
            'identity',
            '--sha',
            sha,
            '--health',
            healthFile,
          ],
          { encoding: 'utf8' },
        ),
      );
    const probe = createEligibilityProbe({
      repository: process.env.GITHUB_REPOSITORY,
      healthFile: process.argv[4],
      gh,
      identity,
    });
    console.log(JSON.stringify(planNightlyPromotion(releases, probe)));
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}
module.exports = { planNightlyPromotion, createEligibilityProbe };

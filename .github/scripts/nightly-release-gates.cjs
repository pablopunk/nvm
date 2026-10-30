'use strict';

const {
  compareVersions,
  publishedNightlies,
} = require('./nightly-versioning.cjs');

const MADRID_TIME_ZONE = 'Europe/Madrid';
const SCHEDULED_MADRID_HOUR = 4;
const FULL_COMMIT_SHA_PATTERN = /^[0-9a-f]{40}$/;
const ANCESTRY_OK_STATUSES = new Set(['identical', 'ahead']);

function madridHour(date) {
  const hourText = new Intl.DateTimeFormat('en-GB', {
    timeZone: MADRID_TIME_ZONE,
    hour: '2-digit',
    hourCycle: 'h23',
  })
    .formatToParts(date)
    .find((part) => part.type === 'hour').value;
  return Number(hourText);
}

// GitHub cron is UTC-only, so the workflow registers both UTC hours that can map
// to 04:00 Madrid and this gate selects the matching schedule and allows a delayed start.
function evaluateScheduleGate({ event, now, scheduledCron }) {
  if (event !== 'schedule')
    return { run: true, reason: `non-scheduled trigger (${event})` };
  const hour = madridHour(now);
  const utcHour =
    scheduledCron === '0 2 * * *'
      ? 2
      : scheduledCron === '0 3 * * *'
        ? 3
        : null;
  const scheduled = new Date(now);
  if (utcHour !== null) scheduled.setUTCHours(utcHour, 0, 0, 0);
  if (
    (utcHour === null && hour !== SCHEDULED_MADRID_HOUR) ||
    (utcHour !== null &&
      (madridHour(scheduled) !== SCHEDULED_MADRID_HOUR || hour < 4))
  )
    return {
      run: false,
      reason:
        'scheduled trigger is outside the selected Madrid 04:00 run',
    };
  return { run: true, reason: 'scheduled Europe/Madrid 04:00 run' };
}

function evaluateCiGate({ sha, runs }) {
  const pushRuns = runs
    .filter((run) => run.head_sha === sha && run.event === 'push')
    .sort((left, right) => right.run_number - left.run_number);
  if (pushRuns.length === 0)
    return {
      ok: false,
      pending: false,
      reason: `no CI push run found for ${sha}`,
    };
  const latest = pushRuns[0];
  if (latest.status !== 'completed')
    return {
      ok: false,
      pending: true,
      reason: `CI run ${latest.id} for ${sha} is still ${latest.status}`,
    };
  if (latest.conclusion !== 'success')
    return {
      ok: false,
      pending: false,
      reason: `CI run ${latest.id} for ${sha} concluded ${latest.conclusion}`,
    };
  return {
    ok: true,
    pending: false,
    reason: `CI run ${latest.id} passed for ${sha}`,
  };
}

function reject(reason) {
  return { status: 'rejected', commit: null, reason };
}

function evaluateDeploymentGate({ commit, runs, jobs }) {
  const matching = runs
    .filter(
      (run) =>
        run.head_sha === commit &&
        run.event === 'workflow_run' &&
        run.head_branch === 'main',
    )
    .sort((left, right) => right.id - left.id);
  const latest = matching[0];
  if (
    !latest ||
    latest.status !== 'completed' ||
    latest.conclusion !== 'success'
  )
    return reject(
      `no successful completed production deployment run for ${commit}`,
    );
  const migration = Array.isArray(jobs)
    ? jobs.find((job) => job.name === 'migrate')
    : null;
  const deploy = migration?.steps?.find(
    (step) => step.name === 'Deploy production',
  );
  if (
    migration?.status !== 'completed' ||
    migration.conclusion !== 'success' ||
    deploy?.conclusion !== 'success'
  )
    return reject(`production deploy step did not succeed in run ${latest.id}`);
  return {
    status: 'verified',
    commit,
    reason: `production deployment run ${latest.id} succeeded`,
  };
}

// Fail closed: production must be healthy, its upstream must return a successful
// model catalog (the additive `upstreamReady` field of /api/health), and
// it must report a full 40-hex commit (the additive `commit` field). Only an exact
// match or a verified descendant of the selected commit that is on main may publish.
function evaluateProductionIdentity({
  sha,
  health,
  descendsStatus,
  onMainStatus,
}) {
  if (!health || typeof health !== 'object')
    return reject('production health is unavailable');
  if (health.ok !== true || health.db !== true || health.upstream !== true)
    return reject('production health is not fully healthy');
  if (typeof health.upstreamReady !== 'boolean')
    return reject(
      'production does not report upstream readiness (deployment predates the upstreamReady contract)',
    );
  if (health.upstreamReady !== true)
    return reject(
      'production upstream did not return a successful model catalog',
    );
  if (!FULL_COMMIT_SHA_PATTERN.test(health.commit ?? ''))
    return reject(
      `production does not report a full commit identity (version=${health.version ?? 'missing'})`,
    );
  if (health.commit === sha)
    return {
      status: 'verified',
      commit: health.commit,
      reason: 'production serves the selected commit',
    };
  if (descendsStatus === undefined || onMainStatus === undefined)
    return {
      status: 'ancestry-required',
      commit: health.commit,
      reason: 'ancestry check pending',
    };
  if (!ANCESTRY_OK_STATUSES.has(descendsStatus))
    return reject(
      `production commit ${health.commit} does not descend from ${sha} (compare status ${descendsStatus}); it is older, rolled back, or unrelated`,
    );
  if (!ANCESTRY_OK_STATUSES.has(onMainStatus))
    return reject(
      `production commit ${health.commit} is not on the current main history`,
    );
  return {
    status: 'verified',
    commit: health.commit,
    reason: `production serves verified main descendant ${health.commit}`,
  };
}

// Publication is the only mutation of public release state, so it re-checks the
// published set immediately before going public. A build for an older version, or
// for a commit that is already published, must not publish.
function evaluatePublicationRace({ releases, sha, version }) {
  const published = publishedNightlies(releases);
  if (published.some((entry) => entry.release.target_commitish === sha))
    return {
      action: 'noop',
      reason: `commit ${sha} already has a published nightly`,
    };
  const superseding = published.find(
    (entry) => compareVersions(entry.version, version) >= 0,
  );
  if (superseding)
    return {
      action: 'reject',
      reason: `published nightly ${superseding.version} is not older than ${version}`,
    };
  return {
    action: 'publish',
    reason: `${version} is newer than every published nightly`,
  };
}

module.exports = {
  evaluateCiGate,
  evaluateDeploymentGate,
  evaluateProductionIdentity,
  evaluatePublicationRace,
  evaluateScheduleGate,
  madridHour,
};

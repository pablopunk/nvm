'use strict';

const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const {
  evaluateCiGate,
  evaluateDeploymentGate,
  evaluateProductionIdentity,
  evaluatePublicationRace,
  evaluateScheduleGate,
  madridHour,
} = require('./nightly-release-gates.cjs');

const CLI = path.join(__dirname, 'nightly-release-cli.cjs');
const SOURCE = 'a'.repeat(40);
const PRODUCTION = 'b'.repeat(40);

test('accepts a delayed selected schedule without admitting the other cron', () => {
  const delayed = new Date('2026-06-15T03:20:00Z');
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: delayed,
      scheduledCron: '0 2 * * *',
    }).run,
    true,
  );
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: delayed,
      scheduledCron: '0 3 * * *',
    }).run,
    false,
  );
});

test('requires the latest matching production deployment to succeed', () => {
  const passed = {
    id: 10,
    head_sha: PRODUCTION,
    head_branch: 'main',
    event: 'workflow_run',
    status: 'completed',
    conclusion: 'success',
  };
  const jobs = [
    {
      name: 'migrate',
      status: 'completed',
      conclusion: 'success',
      steps: [{ name: 'Deploy production', conclusion: 'success' }],
    },
  ];
  assert.equal(
    evaluateDeploymentGate({ commit: PRODUCTION, runs: [passed], jobs }).status,
    'verified',
  );
  assert.equal(
    evaluateDeploymentGate({ commit: PRODUCTION, runs: [passed], jobs: [] })
      .status,
    'rejected',
  );
  assert.equal(
    evaluateDeploymentGate({
      commit: PRODUCTION,
      runs: [passed],
      jobs: [
        {
          ...jobs[0],
          steps: [{ name: 'Deploy production', conclusion: 'skipped' }],
        },
      ],
    }).status,
    'rejected',
  );
  assert.equal(
    evaluateDeploymentGate({ commit: SOURCE, runs: [passed] }).status,
    'rejected',
  );
  assert.equal(
    evaluateDeploymentGate({
      commit: PRODUCTION,
      runs: [passed, { ...passed, id: 11, conclusion: 'failure' }],
    }).status,
    'rejected',
  );
  assert.equal(
    evaluateDeploymentGate({
      commit: PRODUCTION,
      runs: [{ ...passed, status: 'in_progress', conclusion: null }],
    }).status,
    'rejected',
  );
});

function healthy(overrides = {}) {
  return {
    ok: true,
    db: true,
    upstream: true,
    upstreamReady: true,
    version: 'bbbbbbb',
    commit: PRODUCTION,
    ...overrides,
  };
}

// Bodies /api/health reports for an upstream /models probe that returns a non-2xx
// status or a non-catalog 200. The released `upstream` flag stays true for these
// (status < 500), so only `upstreamReady` may keep the gate closed.
const UPSTREAM_NOT_READY_HEALTH = [
  ['401 unauthorized', healthy({ upstreamReady: false })],
  ['403 forbidden', healthy({ upstreamReady: false })],
  ['404 missing catalog path', healthy({ upstreamReady: false })],
  ['429 rate limited', healthy({ upstreamReady: false })],
  ['200 without a model catalog', healthy({ upstreamReady: false })],
];

function withoutUpstreamReady(health) {
  const legacy = { ...health };
  delete legacy.upstreamReady;
  return legacy;
}

function runIdentityCli(healthPath) {
  return JSON.parse(
    execFileSync(
      process.execPath,
      [CLI, 'identity', '--sha', SOURCE, '--health', healthPath],
      { encoding: 'utf8' },
    ),
  );
}

function ciRun(overrides = {}) {
  return {
    id: 1,
    head_sha: SOURCE,
    event: 'push',
    status: 'completed',
    conclusion: 'success',
    run_number: 10,
    ...overrides,
  };
}

test('schedule admits only the 04:xx Europe/Madrid window across both DST transitions', () => {
  // Summer (CEST, UTC+2): 02:00Z is 04:00 local.
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: new Date('2026-06-15T02:00:00Z'),
    }).run,
    true,
  );
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: new Date('2026-06-15T03:00:00Z'),
    }).run,
    false,
  );
  // Winter (CET, UTC+1): 03:00Z is 04:00 local.
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: new Date('2026-10-28T03:00:00Z'),
    }).run,
    true,
  );
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: new Date('2026-10-28T02:00:00Z'),
    }).run,
    false,
  );
  // Spring-forward day 2026-03-29 (01:00Z): 02:00Z is 04:00 CEST.
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: new Date('2026-03-29T02:00:00Z'),
    }).run,
    true,
  );
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: new Date('2026-03-29T03:00:00Z'),
    }).run,
    false,
  );
  // Fall-back day 2026-10-25 (01:00Z): 03:00Z is 04:00 CET.
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: new Date('2026-10-25T03:00:00Z'),
    }).run,
    true,
  );
  assert.equal(
    evaluateScheduleGate({
      event: 'schedule',
      now: new Date('2026-10-25T02:00:00Z'),
    }).run,
    false,
  );
});

test('manual dispatch is never blocked by the schedule window', () => {
  assert.equal(
    evaluateScheduleGate({
      event: 'workflow_dispatch',
      now: new Date('2026-06-15T11:00:00Z'),
    }).run,
    true,
  );
  assert.equal(madridHour(new Date('2026-10-28T03:00:00Z')), 4);
});

test('CI gate requires a completed, successful push run for the exact commit', () => {
  assert.equal(evaluateCiGate({ sha: SOURCE, runs: [ciRun()] }).ok, true);
  assert.equal(evaluateCiGate({ sha: SOURCE, runs: [] }).ok, false);
  assert.equal(
    evaluateCiGate({ sha: SOURCE, runs: [ciRun({ head_sha: PRODUCTION })] }).ok,
    false,
    'runs for another commit do not count',
  );
  assert.equal(
    evaluateCiGate({ sha: SOURCE, runs: [ciRun({ event: 'pull_request' })] })
      .ok,
    false,
  );
});

test('CI gate reports pending and failing latest runs, and the newest run decides', () => {
  const pending = evaluateCiGate({
    sha: SOURCE,
    runs: [ciRun({ status: 'in_progress', conclusion: null })],
  });
  assert.equal(pending.ok, false);
  assert.equal(pending.pending, true);
  const failing = evaluateCiGate({
    sha: SOURCE,
    runs: [ciRun({ conclusion: 'failure' })],
  });
  assert.equal(failing.ok, false);
  assert.equal(failing.pending, false);
  const rerunFailed = evaluateCiGate({
    sha: SOURCE,
    runs: [
      ciRun({ run_number: 10 }),
      ciRun({ id: 2, run_number: 11, conclusion: 'failure' }),
    ],
  });
  assert.equal(rerunFailed.ok, false);
});

test('production identity fails closed when health, commit identity, or health contract is missing', () => {
  assert.equal(
    evaluateProductionIdentity({ sha: SOURCE, health: null }).status,
    'rejected',
  );
  assert.equal(
    evaluateProductionIdentity({ sha: SOURCE, health: healthy({ ok: false }) })
      .status,
    'rejected',
  );
  assert.equal(
    evaluateProductionIdentity({ sha: SOURCE, health: healthy({ db: false }) })
      .status,
    'rejected',
  );
  // Today's abbreviated or fallback identity must never satisfy the gate.
  const abbreviated = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy({ commit: undefined, version: 'bbbbbbb' }),
  });
  assert.equal(abbreviated.status, 'rejected');
  assert.match(abbreviated.reason, /full commit identity/);
  assert.equal(
    evaluateProductionIdentity({
      sha: SOURCE,
      health: healthy({ commit: 'dev' }),
    }).status,
    'rejected',
  );
  assert.equal(
    evaluateProductionIdentity({
      sha: SOURCE,
      health: healthy({ commit: PRODUCTION.slice(0, 7) }),
    }).status,
    'rejected',
  );
});

for (const [label, health] of UPSTREAM_NOT_READY_HEALTH) {
  test(`production identity rejects an upstream ${label} even though health is up`, () => {
    const verdict = evaluateProductionIdentity({ sha: SOURCE, health });
    assert.equal(verdict.status, 'rejected');
    assert.match(verdict.reason, /did not return a successful model catalog/);
  });
}

test('production identity rejects a 5xx or timed-out upstream whether health answered 503 or not at all', () => {
  // The workflow's curl -f drops the body on a 503, so the gate sees no health file.
  assert.equal(
    evaluateProductionIdentity({ sha: SOURCE, health: null }).status,
    'rejected',
  );
  const unreachable = healthy({
    ok: false,
    upstream: false,
    upstreamReady: false,
  });
  assert.equal(
    evaluateProductionIdentity({ sha: SOURCE, health: unreachable }).status,
    'rejected',
  );
});

test('production identity rejects a deployment that does not report upstreamReady', () => {
  // A backend deployed before this contract only reports the legacy flags.
  const legacy = evaluateProductionIdentity({
    sha: SOURCE,
    health: withoutUpstreamReady(healthy()),
  });
  assert.equal(legacy.status, 'rejected');
  assert.match(legacy.reason, /does not report upstream readiness/);
  const malformed = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy({ upstreamReady: 'true' }),
  });
  assert.equal(malformed.status, 'rejected');
  assert.match(malformed.reason, /does not report upstream readiness/);
});

test('identity CLI rejects unready, legacy, and missing production health from real files', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'nightly-identity-'));
  try {
    for (const [label, health] of [
      ...UPSTREAM_NOT_READY_HEALTH,
      ['legacy backend', withoutUpstreamReady(healthy())],
    ]) {
      const healthPath = path.join(
        directory,
        `${label.replace(/\W+/g, '-')}.json`,
      );
      fs.writeFileSync(healthPath, JSON.stringify(health));
      assert.equal(runIdentityCli(healthPath).status, 'rejected', label);
    }
    const missing = runIdentityCli(path.join(directory, 'missing.json'));
    assert.equal(missing.status, 'rejected');
    assert.match(missing.reason, /health is unavailable/);
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});

test('production identity verifies an exact match without ancestry', () => {
  const verdict = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy({ commit: SOURCE }),
  });
  assert.equal(verdict.status, 'verified');
  assert.equal(verdict.commit, SOURCE);
});

test('a different production commit requires an ancestry check first', () => {
  const verdict = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy(),
  });
  assert.equal(verdict.status, 'ancestry-required');
  assert.equal(verdict.commit, PRODUCTION);
});

test('a rolled-back, older, or unrelated production commit is rejected', () => {
  const rolledBack = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy(),
    descendsStatus: 'behind',
    onMainStatus: 'ahead',
  });
  assert.equal(rolledBack.status, 'rejected');
  const unrelated = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy(),
    descendsStatus: 'diverged',
    onMainStatus: 'ahead',
  });
  assert.equal(unrelated.status, 'rejected');
  const failedCompare = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy(),
    descendsStatus: null,
    onMainStatus: 'ahead',
  });
  assert.equal(failedCompare.status, 'rejected');
});

test('a verified descendant must also be on current main history', () => {
  const onMain = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy(),
    descendsStatus: 'ahead',
    onMainStatus: 'ahead',
  });
  assert.equal(onMain.status, 'verified');
  const offMain = evaluateProductionIdentity({
    sha: SOURCE,
    health: healthy(),
    descendsStatus: 'ahead',
    onMainStatus: 'diverged',
  });
  assert.equal(offMain.status, 'rejected');
});

test('publication race guard noops on an already-published commit and rejects older builds', () => {
  const published = {
    tag_name: 'v0.24.1-nightly.20261028.20',
    draft: false,
    prerelease: true,
    target_commitish: PRODUCTION,
  };
  assert.equal(
    evaluatePublicationRace({
      releases: [published],
      sha: PRODUCTION,
      version: '0.24.1-nightly.20261029.30',
    }).action,
    'noop',
  );
  assert.equal(
    evaluatePublicationRace({
      releases: [published],
      sha: SOURCE,
      version: '0.24.1-nightly.20261028.19',
    }).action,
    'reject',
  );
  assert.equal(
    evaluatePublicationRace({
      releases: [published],
      sha: SOURCE,
      version: '0.24.1-nightly.20261029.21',
    }).action,
    'publish',
  );
});

test('publication race guard ignores drafts when comparing published work', () => {
  const draft = {
    tag_name: 'v0.24.1-nightly.20261029.40',
    draft: true,
    prerelease: true,
    target_commitish: PRODUCTION,
  };
  assert.equal(
    evaluatePublicationRace({
      releases: [draft],
      sha: SOURCE,
      version: '0.24.1-nightly.20261029.21',
    }).action,
    'publish',
  );
});

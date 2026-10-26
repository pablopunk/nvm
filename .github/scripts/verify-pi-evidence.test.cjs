const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const {
  lockedPiPackageVersion,
  supportsBetweenToolCompaction,
  verifyPiIntegrationEvidence,
} = require('./verify-pi-integration-evidence.cjs');
const {
  verifyPackagedPiEvidence,
} = require('./verify-packaged-pi-evidence.cjs');

const repository = path.resolve(__dirname, '../..');
const packageManifest = require(path.join(repository, 'package.json'));
const expectedSha = 'a'.repeat(40);
const candidateVersion =
  packageManifest.dependencies['@earendil-works/pi-coding-agent'];

test('adapter evidence distinguishes installed lock versions from the legacy manifest range', function lockedSdkBoundary() {
  const lockfile = fs.readFileSync(
    path.join(repository, 'pnpm-lock.yaml'),
    'utf8',
  );
  assert.equal(
    lockedPiPackageVersion(lockfile, '@earendil-works/pi-coding-agent'),
    candidateVersion,
  );
  assert.equal(supportsBetweenToolCompaction('0.80.10'), false);
  assert.equal(supportsBetweenToolCompaction('0.84.3'), false);
  assert.equal(supportsBetweenToolCompaction('0.84.4'), true);
  assert.equal(supportsBetweenToolCompaction('1.4.0'), true);
});

function validIntegrationEvidence() {
  const builder = {
    scenario: 'builder-proactive-compaction-overflow-continuation-follow-up',
    finalOutcome: 'done',
    proactiveCompaction: true,
    raw413Recovered: true,
    managedHeadersPreserved: true,
    pageCount: 2,
    tokenEvidence: [{ backendEstimate: 10, requestBytes: 20 }],
    toolAllowlist: ['read_extension_api'],
    registeredToolAllowlist: ['read_extension_api'],
  };
  const outcomes = [
    builder,
    {
      scenario: 'invalid-generated-extension-preserves-live-source',
      finalOutcome: 'done',
      validationFailurePreservedLiveSource: true,
    },
    {
      scenario: 'byte-size-413-keeps-request-too-large-notice',
      finalOutcome: 'error',
      noticeKind: 'request_too_large',
    },
    {
      scenario: 'backend-admission-below-pi-proactive-threshold',
      finalOutcome: 'done',
      raw413Recovered: true,
      backendEstimate: 100_001,
      backendLimit: 100_000,
      piProactiveThreshold: 110_000,
    },
    ...[
      'byo-extension-session-overflow-recovery',
      'tool-free-one-shot-direct-stream',
      'prepared-fallback-session-reused',
      'general-session-prompts-serialize-outcomes',
    ].map((scenario) => ({ scenario, finalOutcome: 'done' })),
    ...[
      'provider-413-openai-completions',
      'provider-413-anthropic-messages',
      'provider-413-google-generative-ai',
    ].map((scenario) => ({
      scenario,
      finalOutcome: 'done',
      raw413Recovered: true,
    })),
    ...[
      'provider-raw-413-openai-completions',
      'provider-raw-413-anthropic-messages',
      'provider-raw-413-google-generative-ai',
    ].map((scenario) => ({
      scenario,
      finalOutcome: 'done',
      raw413Recovered: true,
    })),
    {
      scenario: 'conversation-explicit-read-only-tool-allowlist',
      finalOutcome: 'done',
    },
    ...[
      'summary-failure-summary-unauthorized',
      'summary-failure-summary-credits',
      'overflow-recovery-exhausted',
    ].map((scenario) => ({
      scenario,
      finalOutcome: 'error',
      noFalseSuccess: true,
    })),
    {
      scenario: 'host-cancellation-during-compaction',
      finalOutcome: 'aborted',
    },
    {
      scenario: 'host-cancellation-during-transient-retry',
      finalOutcome: 'aborted',
    },
    {
      scenario: 'builder-transient-retry-budget-two',
      providerCalls: 3,
    },
    {
      scenario: 'general-transient-retries-disabled',
      providerCalls: 1,
    },
  ];
  return { sdkVersion: candidateVersion, commitSha: expectedSha, outcomes };
}

test('Pi integration evidence rejects a broken overflow recovery result', async function brokenOverflowEvidence() {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-integration-evidence-'),
  );
  try {
    const evidence = validIntegrationEvidence();
    evidence.outcomes[0].raw413Recovered = false;
    await fs.writeFile(
      path.join(directory, 'integration-evidence.json'),
      JSON.stringify(evidence),
    );
    assert.throws(() => verifyPiIntegrationEvidence(directory, expectedSha));
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

test('Pi integration evidence rejects forbidden tool activation', async function forbiddenToolEvidence() {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-tool-evidence-'),
  );
  try {
    const evidence = validIntegrationEvidence();
    evidence.outcomes[0].registeredToolAllowlist.push('tool_search');
    await fs.writeFile(
      path.join(directory, 'integration-evidence.json'),
      JSON.stringify(evidence),
    );
    assert.throws(() => verifyPiIntegrationEvidence(directory, expectedSha));
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

test('packaged Pi evidence fails when a required import is missing', async function missingPackagedImport() {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-packaged-evidence-'),
  );
  try {
    const evidence = {
      commitSha: expectedSha,
      fixtureSession: 'success',
      provider: 'loopback-openai-completions',
      providerCalls: 1,
      architecture: 'linux-x64',
      packagedImports: {
        'pi-coding-agent': candidateVersion,
        'pi-ai': packageManifest.dependencies['@earendil-works/pi-ai'],
        'pi-web-access': undefined,
      },
    };
    await fs.writeFile(
      path.join(directory, 'pi-session-evidence.json'),
      JSON.stringify(evidence),
    );
    assert.throws(() =>
      verifyPackagedPiEvidence(directory, expectedSha, 'linux-x64'),
    );
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

test('packaged Pi evidence fails when the runtime session artifact is missing', async function missingPackagedEvidence() {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-packaged-no-evidence-'),
  );
  try {
    assert.throws(() =>
      verifyPackagedPiEvidence(directory, expectedSha, 'linux-x64'),
    );
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const REQUIRED_SCENARIOS = [
  'builder-proactive-compaction-overflow-continuation-follow-up',
  'byte-size-413-keeps-request-too-large-notice',
  'invalid-generated-extension-preserves-live-source',
  'backend-admission-below-pi-proactive-threshold',
  'byo-extension-session-overflow-recovery',
  'provider-413-openai-completions',
  'provider-413-anthropic-messages',
  'provider-413-google-generative-ai',
  'provider-raw-413-openai-completions',
  'provider-raw-413-anthropic-messages',
  'provider-raw-413-google-generative-ai',
  'tool-free-one-shot-direct-stream',
  'prepared-fallback-session-reused',
  'general-session-prompts-serialize-outcomes',
  'conversation-explicit-read-only-tool-allowlist',
  'summary-failure-summary-unauthorized',
  'summary-failure-summary-credits',
  'overflow-recovery-exhausted',
  'host-cancellation-during-compaction',
  'host-cancellation-during-transient-retry',
  'builder-transient-retry-budget-two',
  'general-transient-retries-disabled',
];

function verifyPiIntegrationEvidence(directory, expectedSha) {
  const evidencePath = path.join(directory, 'integration-evidence.json');
  if (!fs.existsSync(evidencePath))
    throw new Error(`Pi adapter evidence is missing: ${evidencePath}`);
  const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
  const manifest = JSON.parse(fs.readFileSync('package.json', 'utf8'));
  const lockfile = fs.readFileSync('pnpm-lock.yaml', 'utf8');
  const candidateVersion = lockedPiPackageVersion(
    lockfile,
    '@earendil-works/pi-coding-agent',
  );
  assert.equal(evidence.commitSha, expectedSha);
  assert.equal(evidence.sdkVersion, candidateVersion);
  for (const packageName of [
    '@earendil-works/pi-ai',
    '@earendil-works/pi-coding-agent',
  ]) {
    assert.equal(
      lockedPiPackageVersion(lockfile, packageName),
      candidateVersion,
    );
    assert.equal(
      lockfileImporterSpecifier(lockfile, packageName),
      manifest.dependencies[packageName],
      `the lockfile specifier for ${packageName} must match the manifest`,
    );
  }
  assert.ok(Array.isArray(evidence.outcomes));
  const byScenario = new Map(
    evidence.outcomes.map((outcome) => [outcome.scenario, outcome]),
  );
  for (const scenario of REQUIRED_SCENARIOS) {
    assert.ok(
      byScenario.has(scenario),
      `missing Pi integration evidence: ${scenario}`,
    );
  }

  const builder = byScenario.get(
    'builder-proactive-compaction-overflow-continuation-follow-up',
  );
  assert.equal(builder.finalOutcome, 'done');
  if (supportsBetweenToolCompaction(candidateVersion))
    assert.equal(builder.proactiveCompaction, true);
  else assert.equal(builder.overflowFallback, true);
  assert.equal(builder.raw413Recovered, true);
  assert.equal(builder.managedHeadersPreserved, true);
  assert.ok(builder.pageCount >= 2);
  assert.ok(
    Array.isArray(builder.tokenEvidence) && builder.tokenEvidence.length >= 3,
  );
  assert.ok(
    builder.tokenEvidence.every(
      (request) => request.backendEstimate > 0 && request.requestBytes > 0,
    ),
  );
  const estimatorBoundary = byScenario.get(
    'backend-admission-below-pi-proactive-threshold',
  );
  assert.equal(estimatorBoundary.finalOutcome, 'done');
  assert.equal(estimatorBoundary.raw413Recovered, true);
  assert.ok(
    estimatorBoundary.backendEstimate >= estimatorBoundary.backendLimit,
  );
  assert.ok(
    estimatorBoundary.backendEstimate < estimatorBoundary.piProactiveThreshold,
  );
  assert.ok(Array.isArray(builder.registeredToolAllowlist));
  assert.ok(
    builder.registeredToolAllowlist.every((name) =>
      builder.toolAllowlist.includes(name),
    ),
    'registered or deferred tools must stay within the active allowlist',
  );
  assert.ok(
    !builder.registeredToolAllowlist.some((name) =>
      ['tool_search', 'codemode', 'mcp'].some((forbidden) =>
        String(name).toLowerCase().includes(forbidden),
      ),
    ),
    'tool discovery and native extension tools must not be loaded',
  );
  const validationFailure = byScenario.get(
    'invalid-generated-extension-preserves-live-source',
  );
  assert.equal(validationFailure.finalOutcome, 'done');
  assert.equal(validationFailure.validationFailurePreservedLiveSource, true);
  const byteSizeFailure = byScenario.get(
    'byte-size-413-keeps-request-too-large-notice',
  );
  assert.equal(byteSizeFailure.finalOutcome, 'error');
  assert.equal(byteSizeFailure.noticeKind, 'request_too_large');

  for (const scenario of [
    'byo-extension-session-overflow-recovery',
    'provider-413-openai-completions',
    'provider-413-anthropic-messages',
    'provider-413-google-generative-ai',
    'tool-free-one-shot-direct-stream',
    'prepared-fallback-session-reused',
  ])
    assert.equal(byScenario.get(scenario).finalOutcome, 'done', scenario);
  for (const scenario of [
    'provider-413-openai-completions',
    'provider-413-anthropic-messages',
    'provider-413-google-generative-ai',
  ])
    assert.equal(byScenario.get(scenario).raw413Recovered, true, scenario);
  for (const scenario of [
    'provider-raw-413-openai-completions',
    'provider-raw-413-anthropic-messages',
    'provider-raw-413-google-generative-ai',
  ]) {
    const rawOutcome = byScenario.get(scenario);
    assert.equal(
      rawOutcome.finalOutcome,
      supportsBetweenToolCompaction(candidateVersion) ? 'done' : 'error',
      scenario,
    );
    assert.equal(
      rawOutcome.raw413Recovered,
      supportsBetweenToolCompaction(candidateVersion),
      scenario,
    );
  }
  assert.equal(
    byScenario.get('conversation-explicit-read-only-tool-allowlist')
      .finalOutcome,
    'done',
  );
  for (const scenario of [
    'summary-failure-summary-unauthorized',
    'summary-failure-summary-credits',
    'overflow-recovery-exhausted',
  ]) {
    assert.equal(byScenario.get(scenario).finalOutcome, 'error', scenario);
    assert.equal(byScenario.get(scenario).noFalseSuccess, true, scenario);
  }
  assert.equal(
    byScenario.get('host-cancellation-during-compaction').finalOutcome,
    'aborted',
  );
  assert.equal(
    byScenario.get('host-cancellation-during-transient-retry').finalOutcome,
    'aborted',
  );
  assert.equal(
    byScenario.get('builder-transient-retry-budget-two').providerCalls,
    3,
  );
  assert.equal(
    byScenario.get('general-transient-retries-disabled').providerCalls,
    1,
  );
  return evidence;
}

function lockfileImporterEntry(lockfile, packageName) {
  const escapedPackageName = packageName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = new RegExp(
    `^      ['\"]?${escapedPackageName}['\"]?:\\n        specifier: ['\"]?([^\\n'\"]+)['\"]?\\n        version: ['\"]?([^\\n'\"]+)['\"]?`,
    'm',
  ).exec(lockfile);
  if (!match)
    throw new Error(`Pi lockfile root importer is missing ${packageName}.`);
  return { specifier: match[1], version: match[2] };
}

function lockfileImporterSpecifier(lockfile, packageName) {
  return lockfileImporterEntry(lockfile, packageName).specifier;
}

function lockedPiPackageVersion(lockfile, packageName) {
  return lockfileImporterEntry(lockfile, packageName).version.split('(')[0];
}

function supportsBetweenToolCompaction(version) {
  const [major = 0, minor = 0, patch = 0] = version.split('.').map(Number);
  return major > 0 || minor > 84 || (minor === 84 && patch >= 4);
}

if (require.main === module) {
  try {
    console.log(
      JSON.stringify(
        verifyPiIntegrationEvidence(
          path.resolve(process.argv[2]),
          process.argv[3],
        ),
      ),
    );
  } catch (error) {
    console.error(error);
    process.exitCode = 1;
  }
}

module.exports = {
  REQUIRED_SCENARIOS,
  lockedPiPackageVersion,
  supportsBetweenToolCompaction,
  verifyPiIntegrationEvidence,
};

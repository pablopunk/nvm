const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { finished } = require('node:stream/promises');
const { pathToFileURL } = require('node:url');
const test = require('node:test');
const asar = require('@electron/asar');
const {
  lockedPiPackageVersion,
  supportsBetweenToolCompaction,
  verifyPiIntegrationEvidence,
} = require('./verify-pi-integration-evidence.cjs');
const {
  verifyPackagedPiElectronEvidence,
} = require('./verify-packaged-pi-electron-evidence.cjs');

const repository = path.resolve(__dirname, '../..');
const packageManifest = require(path.join(repository, 'package.json'));
const expectedSha = 'a'.repeat(40);
const candidateVersion =
  packageManifest.dependencies['@earendil-works/pi-coding-agent'];

test('adapter evidence distinguishes installed lock versions from the legacy manifest range', async function lockedSdkBoundary() {
  const lockfile = await fs.readFile(
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
    managedProviderIdentity: 'nevermind',
    providerApiFormat: 'openai-completions',
    pageCount: 2,
    tokenEvidence: [
      { backendEstimate: 10, requestBytes: 20 },
      { backendEstimate: 10, requestBytes: 20 },
      { backendEstimate: 10, requestBytes: 20 },
    ],
    toolAllowlist: ['read_extension_api'],
    activeToolNames: ['read_extension_api'],
    callableToolNames: ['read_extension_api'],
    registeredTools: [{ name: 'read_extension_api', exposure: 'direct' }],
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
      toolAllowlist: [
        'fetch_content',
        'find',
        'grep',
        'ls',
        'read',
        'web_search',
      ],
      activeToolNames: [
        'fetch_content',
        'find',
        'grep',
        'ls',
        'read',
        'web_search',
      ],
      callableToolNames: [
        'fetch_content',
        'find',
        'grep',
        'ls',
        'read',
        'web_search',
      ],
      registeredTools: [
        ...[
          'fetch_content',
          'find',
          'get_search_content',
          'grep',
          'ls',
          'read',
          'source_check',
          'web_search',
        ].map((name) => ({ name, exposure: 'direct' })),
      ],
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
      scenario: 'general-session-cancellation-during-model-prepare',
      finalOutcome: 'aborted',
      providerCalls: 1,
      promptSuppressed: true,
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

test('adapter evidence accepts registered direct-only tools absent from the allowlist', async function inactiveDirectToolEvidence() {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-inactive-direct-tool-evidence-'),
  );
  try {
    const evidence = validIntegrationEvidence();
    await fs.writeFile(
      path.join(directory, 'integration-evidence.json'),
      JSON.stringify(evidence),
    );
    assert.equal(
      verifyPiIntegrationEvidence(directory, expectedSha).sdkVersion,
      candidateVersion,
    );
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

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
    evidence.outcomes[0].callableToolNames.push('tool_search');
    evidence.outcomes[0].registeredTools.push({
      name: 'tool_search',
      exposure: 'deferred',
    });
    await fs.writeFile(
      path.join(directory, 'integration-evidence.json'),
      JSON.stringify(evidence),
    );
    assert.throws(() => verifyPiIntegrationEvidence(directory, expectedSha));
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

test('Pi integration evidence rejects an allowlisted deferred tool', async function deferredToolEvidence() {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-deferred-tool-evidence-'),
  );
  try {
    const evidence = validIntegrationEvidence();
    evidence.outcomes[0].registeredTools[0].exposure = 'deferred';
    await fs.writeFile(
      path.join(directory, 'integration-evidence.json'),
      JSON.stringify(evidence),
    );
    assert.throws(
      () => verifyPiIntegrationEvidence(directory, expectedSha),
      /allowlist may only declare direct tools/,
    );
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

async function packagedPiArchive(t, { omitPackage } = {}) {
  const temporary = await fs.mkdtemp(
    path.join(os.tmpdir(), 'pi-packaged-electron-evidence-'),
  );
  t.after(function removeFixture() {
    return fs.rm(temporary, { recursive: true, force: true });
  });
  const source = path.join(temporary, 'source');
  const packageVersions = {
    '@earendil-works/pi-coding-agent': candidateVersion,
    '@earendil-works/pi-ai':
      packageManifest.dependencies['@earendil-works/pi-ai'],
    'pi-web-access': packageManifest.dependencies['pi-web-access'],
  };
  await fs.mkdir(source, { recursive: true });
  await fs.writeFile(
    path.join(source, 'package.json'),
    JSON.stringify({ name: 'nvm', dependencies: packageVersions }),
  );
  for (const [name, version] of Object.entries(packageVersions)) {
    if (name === omitPackage) continue;
    const packageDirectory = path.join(source, 'node_modules', name);
    await fs.mkdir(packageDirectory, { recursive: true });
    await fs.writeFile(
      path.join(packageDirectory, 'package.json'),
      JSON.stringify({ name, version }),
    );
  }
  const archive = path.join(temporary, 'app.asar');
  await finished(await asar.createPackage(source, archive));
  return { archive, packageVersions };
}

function validPiElectronEvidence(archive, packageVersions) {
  function moduleEntry(name, entry) {
    return pathToFileURL(path.join(archive, 'node_modules', name, entry)).href;
  }
  return {
    commitSha: expectedSha,
    runId: '12345',
    architecture: 'linux-x64',
    runtimeBoundary: 'packaged-electron-nevermind-adapter',
    runtime: 'electron',
    electronRuntime: true,
    adapterIntegration: true,
    appIsPackaged: true,
    appPath: archive,
    finalOutcome: 'done',
    terminalEvents: ['done'],
    responseMatches: true,
    sessionDisposed: true,
    provider: 'loopback-openai-completions',
    providerApiFormat: 'openai-completions',
    providerCalls: 1,
    providerPaths: ['/v1/chat/completions'],
    unexpectedRoutes: [],
    toolNames: ['fetch_content', 'find', 'grep', 'ls', 'read', 'web_search'],
    managedHeadersPreserved: true,
    resolvedModules: {
      piCodingAgent: moduleEntry(
        '@earendil-works/pi-coding-agent',
        'dist/index.js',
      ),
      piAi: moduleEntry('@earendil-works/pi-ai', 'dist/compat.js'),
      piWebAccess: moduleEntry('pi-web-access', 'package.json'),
    },
    packageVersions,
  };
}

test('packaged Pi evidence binds the real Electron session and modules to app.asar', async function packagedElectronBoundary(t) {
  const { archive, packageVersions } = await packagedPiArchive(t);
  const directory = path.dirname(archive);
  const evidence = validPiElectronEvidence(archive, packageVersions);
  await fs.writeFile(
    path.join(directory, 'pi-electron-evidence.json'),
    JSON.stringify(evidence),
  );
  assert.equal(
    verifyPackagedPiElectronEvidence(directory, expectedSha, 'linux-x64')
      .runtimeBoundary,
    'packaged-electron-nevermind-adapter',
  );
});

test('packaged Pi evidence rejects resolution that escapes app.asar', async function packageResolutionEscape(t) {
  const { archive, packageVersions } = await packagedPiArchive(t);
  const directory = path.dirname(archive);
  const evidence = validPiElectronEvidence(archive, packageVersions);
  evidence.resolvedModules.piWebAccess = pathToFileURL(
    path.join(directory, 'node_modules', 'pi-web-access', 'package.json'),
  ).href;
  await fs.writeFile(
    path.join(directory, 'pi-electron-evidence.json'),
    JSON.stringify(evidence),
  );
  assert.throws(
    () => verifyPackagedPiElectronEvidence(directory, expectedSha, 'linux-x64'),
    /piWebAccess must resolve from the running app\.asar/,
  );
});

test('packaged Pi evidence fails when an SDK or web package is missing from app.asar', async function missingPackagedImport(t) {
  const { archive, packageVersions } = await packagedPiArchive(t, {
    omitPackage: 'pi-web-access',
  });
  const directory = path.dirname(archive);
  await fs.writeFile(
    path.join(directory, 'pi-electron-evidence.json'),
    JSON.stringify(validPiElectronEvidence(archive, packageVersions)),
  );
  assert.throws(() =>
    verifyPackagedPiElectronEvidence(directory, expectedSha, 'linux-x64'),
  );
});

test('packaged Pi evidence rejects a missing runtime session result', async function missingPackagedEvidence(t) {
  const { archive } = await packagedPiArchive(t);
  const directory = path.dirname(archive);
  assert.throws(
    () => verifyPackagedPiElectronEvidence(directory, expectedSha, 'linux-x64'),
    /Packaged Pi Electron evidence is missing/,
  );
});

test('packaged Pi evidence cannot claim success after an error terminal', async function noFalseTerminalSuccess(t) {
  const { archive, packageVersions } = await packagedPiArchive(t);
  const directory = path.dirname(archive);
  const evidence = validPiElectronEvidence(archive, packageVersions);
  evidence.terminalEvents = ['error'];
  await fs.writeFile(
    path.join(directory, 'pi-electron-evidence.json'),
    JSON.stringify(evidence),
  );
  assert.throws(
    () => verifyPackagedPiElectronEvidence(directory, expectedSha, 'linux-x64'),
    /strictly deep-equal/,
  );
});

const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const { createServer } = require('node:http');
const os = require('node:os');
const path = require('node:path');
const { fileURLToPath } = require('node:url');
const { _electron: electron } = require('playwright');
const asar = require('@electron/asar');
const {
  createDiagnosticSmokeCollector,
} = require('./diagnostic-smoke-collector.cjs');
const PACKAGED_PI_SESSION_TIMEOUT_MS = 45_000;

async function waitForCapture(collector, predicate) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    assert.deepEqual(collector.failures, []);
    const captured = collector.events.find(predicate);
    if (captured) return captured.event;
    await new Promise(function retry(resolve) {
      setTimeout(resolve, 100);
    });
  }
  throw new Error(
    `Diagnostic capture did not arrive; received ${collector.events.length} envelopes`,
  );
}

function smokeEnvironment(temporary, artifacts, dsn, piFixture) {
  const env = Object.fromEntries(
    Object.entries(process.env).filter(function excludeCredentials([name]) {
      return (
        !/TOKEN|SECRET|PASSWORD|API_KEY|DSN/i.test(name) &&
        !name.startsWith('NVM_')
      );
    }),
  );
  const fixtureEnvironment = piFixture
    ? {
        NEVERMIND_BASE_URL: piFixture.baseUrl,
        NVM_TEST_PI_PACKAGED: '1',
        PI_CODING_AGENT_DIR: path.join(temporary, 'pi-agent-config'),
        HOME: path.join(temporary, 'home'),
        USERPROFILE: path.join(temporary, 'home'),
        XDG_CONFIG_HOME: path.join(temporary, 'xdg-config'),
      }
    : {};
  return {
    ...env,
    NVM_TEST_MODE: '1',
    NVM_TEST_DIAGNOSTICS: '1',
    NVM_TEST_HEADLESS: '0',
    NVM_TEST_USER_DATA_DIR: temporary,
    NVM_TEST_ARTIFACT_DIR: artifacts,
    SENTRY_DSN_DESKTOP: dsn,
    ...fixtureEnvironment,
  };
}

function sendFixtureJson(response, status, body) {
  response.writeHead(status, { 'content-type': 'application/json' });
  response.end(JSON.stringify(body));
}

async function requestBodyText(request) {
  const chunks = [];
  for await (const chunk of request) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks).toString('utf8');
}

function sendPiFixtureCompletion(response) {
  const text = 'PACKAGED_PI_ELECTRON_SESSION_OK';
  const chunks = [
    {
      id: 'packaged-pi-smoke',
      object: 'chat.completion.chunk',
      created: 1,
      model: 'fixture-model',
      choices: [
        { index: 0, delta: { role: 'assistant' }, finish_reason: null },
      ],
    },
    {
      id: 'packaged-pi-smoke',
      object: 'chat.completion.chunk',
      created: 1,
      model: 'fixture-model',
      choices: [{ index: 0, delta: { content: text }, finish_reason: null }],
    },
    {
      id: 'packaged-pi-smoke',
      object: 'chat.completion.chunk',
      created: 1,
      model: 'fixture-model',
      choices: [{ index: 0, delta: {}, finish_reason: 'stop' }],
      usage: { prompt_tokens: 12, completion_tokens: 2, total_tokens: 14 },
    },
  ];
  response.writeHead(200, {
    'cache-control': 'no-cache',
    connection: 'keep-alive',
    'content-type': 'text/event-stream; charset=utf-8',
  });
  response.end(
    `${chunks.map((chunk) => `data: ${JSON.stringify(chunk)}\n\n`).join('')}data: [DONE]\n\n`,
  );
}

async function createPackagedPiFixture() {
  const requests = [];
  const unexpectedRoutes = [];
  let baseUrl = '';
  const server = createServer(
    async function handlePiFixture(request, response) {
      const requestUrl = new URL(request.url || '/', baseUrl);
      const pathname = requestUrl.pathname;
      if (request.method === 'GET' && pathname === '/api/compatibility') {
        sendFixtureJson(response, 200, {
          backend: { environment: 'fixture', version: '1' },
          api: { currentVersion: 1, supportedVersions: [1] },
          desktop: {
            minimumSupportedVersion: '0.0.0',
            latestVersion: null,
            updateUrl: 'https://example.invalid/update',
          },
          client: { compatible: true, unsupportedReason: null },
          features: {},
        });
        return;
      }
      if (request.method === 'GET' && pathname === '/api/v1/active-model') {
        sendFixtureJson(response, 200, {
          id: 'fixture-model',
          name: 'Fixture Model',
          provider: 'nevermind',
          api: 'openai-completions',
          baseUrl: `${baseUrl}/v1`,
          contextWindow: 64_000,
          maxInputTokens: 48_000,
          maxTokens: 1_024,
          reasoning: false,
          thinkingLevel: 'off',
          input: ['text'],
          modelTier: 'pro',
          creditKind: 'paid',
          credits: { paid: 1000, free: 0, total: 1000 },
          notice: 'ok',
        });
        return;
      }
      if (
        request.method === 'POST' &&
        pathname === '/v1/chat/completions' &&
        requestUrl.search.length === 0
      ) {
        let body;
        try {
          body = JSON.parse(await requestBodyText(request));
        } catch {
          sendFixtureJson(response, 400, { error: 'invalid request JSON' });
          return;
        }
        const toolNames = Array.isArray(body.tools)
          ? body.tools
              .map((tool) => tool.function?.name)
              .filter(Boolean)
              .sort()
          : [];
        requests.push({
          path: `${pathname}${requestUrl.search}`,
          model: body.model,
          toolNames,
          authorization: request.headers.authorization,
          modelRole: request.headers['x-nevermind-ai-model'],
          modelTier: request.headers['x-nevermind-ai-model-tier'],
          creditKind: request.headers['x-nevermind-ai-credit-kind'],
          apiVersion: request.headers['x-nevermind-api-version'],
          client: request.headers['x-nevermind-client'],
        });
        sendPiFixtureCompletion(response);
        return;
      }
      unexpectedRoutes.push({ method: request.method, path: pathname });
      sendFixtureJson(response, 404, {
        error: `unknown fixture route: ${request.method} ${pathname}`,
      });
    },
  );

  await new Promise(function listenForPiFixture(resolve, reject) {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  baseUrl = `http://127.0.0.1:${address.port}`;
  return {
    baseUrl,
    requests,
    unexpectedRoutes,
    close: () =>
      new Promise(function closePiFixture(resolve, reject) {
        server.close(function fixtureClosed(error) {
          if (error) reject(error);
          else resolve();
        });
      }),
  };
}

async function preparePackagedPiAuth(userData, baseUrl) {
  await fs.writeFile(
    path.join(userData, 'nevermind-auth-by-origin.json'),
    JSON.stringify({
      [baseUrl]: {
        email: 'packaged-pi-fixture@example.invalid',
        role: 'pro',
        baseUrl,
        environment: 'development',
        connectedAt: '2026-10-26T00:00:00.000Z',
        token: Buffer.from('packaged-pi-fixture-token').toString('base64'),
      },
    }),
    { mode: 0o600 },
  );
}

function packagedPiVersions(archive) {
  const names = [
    '@earendil-works/pi-coding-agent',
    '@earendil-works/pi-ai',
    'pi-web-access',
  ];
  const root = JSON.parse(asar.extractFile(archive, 'package.json').toString());
  return Object.fromEntries(
    names.map(function packageVersion(name) {
      const manifestPath = `node_modules/${name}/package.json`;
      const packageManifest = JSON.parse(
        asar.extractFile(archive, manifestPath).toString(),
      );
      assert.equal(
        root.dependencies?.[name],
        packageManifest.version,
        `${name} in app.asar must match its direct exact dependency pin`,
      );
      return [name, packageManifest.version];
    }),
  );
}

function runtimePathWithinArchive(runtimePath, archive) {
  const resolvedPath = fileURLToPath(runtimePath);
  const relative = path.relative(path.resolve(archive), resolvedPath);
  return (
    relative !== '' &&
    !relative.startsWith(`..${path.sep}`) &&
    relative !== '..' &&
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

async function evaluatePackagedPiSession(page) {
  let timeout;
  try {
    return await Promise.race([
      page.evaluate(function runPackagedPiSession() {
        return window.nvm.testRunPiSession();
      }),
      new Promise((_, reject) => {
        timeout = setTimeout(() => {
          const error = new Error('Packaged Pi Electron session timed out');
          error.name = 'TimeoutError';
          reject(error);
        }, PACKAGED_PI_SESSION_TIMEOUT_MS);
      }),
    ]);
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

async function runPackagedPiSessionSmoke(page, fixture, archive, artifacts) {
  let runtime;
  let invocationErrorName;
  try {
    runtime = await evaluatePackagedPiSession(page);
  } catch (error) {
    invocationErrorName = error instanceof Error ? error.name : 'UnknownError';
  }

  const request = fixture.requests[0];
  const packageVersions = packagedPiVersions(archive);
  const toolNames = request?.toolNames || [];
  const terminalEvents = runtime?.terminalEvents || [];
  const evidence = {
    commitSha: process.env.GITHUB_SHA || 'local',
    runId: process.env.GITHUB_RUN_ID || 'local',
    architecture: `${process.platform}-${process.arch}`,
    runtimeBoundary: 'packaged-electron-nevermind-adapter',
    runtime: 'electron',
    electronRuntime: true,
    adapterIntegration: true,
    appIsPackaged: runtime?.appIsPackaged === true,
    appPath: runtime?.appPath || null,
    finalOutcome: terminalEvents.at(-1) || 'error',
    terminalEvents,
    responseMatches: runtime?.responseMatches === true,
    sessionDisposed: runtime?.sessionDisposed === true,
    ...(runtime?.errorName ? { errorName: runtime.errorName } : {}),
    ...(invocationErrorName ? { invocationErrorName } : {}),
    provider: 'loopback-openai-completions',
    providerApiFormat: 'openai-completions',
    providerCalls: fixture.requests.length,
    providerPaths: fixture.requests.map((entry) => entry.path),
    unexpectedRoutes: fixture.unexpectedRoutes,
    toolNames,
    managedHeadersPreserved: Boolean(
      request &&
        request.authorization === 'Bearer packaged-pi-fixture-token' &&
        request.modelRole === 'fast' &&
        request.modelTier === 'pro' &&
        request.creditKind === 'paid' &&
        request.apiVersion === '1' &&
        request.client === 'desktop',
    ),
    resolvedModules: runtime?.resolvedModules || {},
    packageVersions,
  };
  await fs.mkdir(artifacts, { recursive: true });
  await fs.writeFile(
    path.join(artifacts, 'pi-electron-evidence.json'),
    JSON.stringify(evidence, null, 2),
  );

  assert.equal(evidence.appIsPackaged, true);
  assert.ok(evidence.appPath && samePath(evidence.appPath, archive));
  assert.deepEqual(evidence.terminalEvents, ['done']);
  assert.equal(evidence.finalOutcome, 'done');
  assert.equal(evidence.responseMatches, true);
  assert.equal(evidence.sessionDisposed, true);
  assert.equal(evidence.providerCalls, 1);
  assert.deepEqual(evidence.unexpectedRoutes, []);
  assert.deepEqual(evidence.providerPaths, ['/v1/chat/completions']);
  assert.deepEqual(toolNames, [
    'fetch_content',
    'find',
    'grep',
    'ls',
    'read',
    'web_search',
  ]);
  assert.equal(evidence.managedHeadersPreserved, true);
  for (const moduleName of ['piCodingAgent', 'piAi', 'piWebAccess'])
    assert.ok(
      runtimePathWithinArchive(evidence.resolvedModules[moduleName], archive),
      `${moduleName} must resolve from the packaged app archive`,
    );
  return evidence;
}

async function prepareDiagnosticFixture(temporary) {
  const extensions = path.join(temporary, 'extensions');
  await fs.mkdir(extensions, { recursive: true });
  await fs.writeFile(
    path.join(extensions, 'diagnostic-smoke.ts'),
    `export default {
    id: 'qa.diagnostic-smoke', title: 'QA Diagnostics', capabilities: [],
    commands: [{ id: 'error', title: 'QA Diagnostics Error', run: function controlledFailure() { throw new TypeError('PRIVATE_MAIN_FIXTURE'); } }],
  };`,
  );
  await fs.writeFile(
    path.join(temporary, 'state.json'),
    JSON.stringify({
      hasCompletedOnboarding: true,
      settings: { errorReporting: true },
      extensionManager: { schemaVersion: 0, files: {}, proposals: {} },
    }),
  );
}

async function runPaletteCommand(page, title) {
  const input = page.getByRole('combobox', { name: 'Nevermind', exact: true });
  await input.fill(title);
  await page.getByRole('option').filter({ hasText: title }).first().click();
}

async function packagedDiagnosticsSmoke(
  executable,
  artifacts,
  { archive, piArtifacts } = {},
) {
  const temporary = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nevermind-diagnostic-smoke-'),
  );
  let piFixture;
  let collector;
  let app;
  let page;
  try {
    await fs.mkdir(artifacts, { recursive: true });
    piFixture = piArtifacts ? await createPackagedPiFixture() : null;
    if (piArtifacts) await fs.mkdir(piArtifacts, { recursive: true });
    if (piFixture) {
      await Promise.all(
        ['home', 'xdg-config', 'pi-agent-config'].map((directory) =>
          fs.mkdir(path.join(temporary, directory), { recursive: true }),
        ),
      );
      await preparePackagedPiAuth(temporary, piFixture.baseUrl);
    }
    collector = await createDiagnosticSmokeCollector();
    await prepareDiagnosticFixture(temporary);
    app = await electron.launch({
      executablePath: executable,
      args: process.platform === 'linux' ? ['--no-sandbox'] : [],
      env: smokeEnvironment(temporary, artifacts, collector.dsn, piFixture),
      timeout: 30_000,
    });
    app.process().stdout?.on('data', function recordStdout(chunk) {
      void fs.appendFile(path.join(artifacts, 'app.stdout.log'), chunk);
    });
    app.process().stderr?.on('data', function recordStderr(chunk) {
      void fs.appendFile(path.join(artifacts, 'app.stderr.log'), chunk);
    });
    const packaged = await app.evaluate(function inspectPackage({ app }) {
      return app.isPackaged;
    });
    assert.equal(
      packaged,
      true,
      'Diagnostics smoke must use the packaged application',
    );
    page = await app.firstWindow();
    await runPaletteCommand(page, 'Report a Problem');
    const report = await waitForCapture(
      collector,
      function supportReport(item) {
        return (
          item.type === 'event' &&
          item.event.tags?.operation === 'support.report'
        );
      },
    );
    assert.match(report.tags.support_ref, /^[a-f0-9]{32}$/);
    await page.screenshot({ path: path.join(artifacts, 'problem-report.png') });
    const journey = await waitForCapture(
      collector,
      function successfulJourney(item) {
        return (
          item.type === 'transaction' &&
          item.event.tags?.outcome === 'success' &&
          item.event.tags?.journey_id === report.tags.journey_id
        );
      },
    );
    assert.match(journey.tags.action_id, /^[a-f0-9]{32}$/);
    await page.keyboard.press('Escape');
    await runPaletteCommand(page, 'QA Diagnostics Error');
    const mainError = await waitForCapture(
      collector,
      function controlledMainError(item) {
        return (
          item.type === 'event' &&
          item.event.tags?.component === 'user-extension' &&
          item.event.exception?.values?.some(function typeError(value) {
            return value.type === 'TypeError';
          })
        );
      },
    );
    const existingEventIds = new Set(
      collector.events.map(function eventId(item) {
        return item.event.event_id;
      }),
    );
    await page.evaluate(function failRenderer() {
      setTimeout(function controlledRendererFailure() {
        throw new Error('PRIVATE_RENDERER_FIXTURE');
      }, 0);
    });
    const rendererError = await waitForCapture(
      collector,
      function controlledRendererError(item) {
        return (
          item.type === 'event' &&
          item.event.tags?.process === 'renderer' &&
          !existingEventIds.has(item.event.event_id)
        );
      },
    );
    assert.notEqual(mainError.tags.support_ref, rendererError.tags.support_ref);
    if (piFixture && piArtifacts) {
      page.setDefaultTimeout(45_000);
      await runPackagedPiSessionSmoke(page, piFixture, archive, piArtifacts);
    }
    const evidence = {
      packaged,
      release: report.release,
      supportEventId: report.event_id,
      journeyEventId: journey.event_id,
      mainErrorEventId: mainError.event_id,
      rendererErrorEventId: rendererError.event_id,
      privateContentExcluded: true,
      collector: 'loopback',
    };
    await fs.writeFile(
      path.join(artifacts, 'evidence.json'),
      JSON.stringify(evidence, null, 2),
    );
    console.log(JSON.stringify(evidence));
  } finally {
    if (page && !page.isClosed()) {
      await page.screenshot({ path: path.join(artifacts, 'final-state.png') });
      await fs.writeFile(
        path.join(artifacts, 'final-state.txt'),
        await page.locator('body').innerText(),
      );
    }
    if (collector)
      await fs.writeFile(
        path.join(artifacts, 'received-events.json'),
        JSON.stringify(
          { events: collector.events, failures: collector.failures },
          null,
          2,
        ),
      );
    await app?.close();
    await collector?.close();
    await piFixture?.close();
    await fs.rm(temporary, { recursive: true, force: true });
  }
}

module.exports = { packagedDiagnosticsSmoke };

if (require.main === module) {
  packagedDiagnosticsSmoke(
    path.resolve(process.argv[2]),
    path.resolve(process.argv[3]),
  ).catch(function smokeFailed(error) {
    console.error(error);
    process.exitCode = 1;
  });
}

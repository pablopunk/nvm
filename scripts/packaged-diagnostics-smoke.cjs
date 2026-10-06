const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { _electron: electron } = require('playwright');
const {
  createDiagnosticSmokeCollector,
} = require('./diagnostic-smoke-collector.cjs');

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

function smokeEnvironment(temporary, artifacts, dsn) {
  const env = Object.fromEntries(
    Object.entries(process.env).filter(function excludeCredentials([name]) {
      return (
        !/TOKEN|SECRET|PASSWORD|API_KEY|DSN/i.test(name) &&
        !name.startsWith('NVM_')
      );
    }),
  );
  return {
    ...env,
    NVM_TEST_MODE: '1',
    NVM_TEST_DIAGNOSTICS: '1',
    NVM_TEST_HEADLESS: '0',
    NVM_TEST_USER_DATA_DIR: temporary,
    NVM_TEST_ARTIFACT_DIR: artifacts,
    SENTRY_DSN_DESKTOP: dsn,
  };
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
  await page.getByRole('option').filter({ hasText: title }).first().waitFor();
  await input.press('Enter');
}

async function packagedDiagnosticsSmoke(executable, artifacts) {
  const temporary = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nevermind-diagnostic-smoke-'),
  );
  await fs.mkdir(artifacts, { recursive: true });
  const collector = await createDiagnosticSmokeCollector();
  let app;
  let page;
  try {
    await prepareDiagnosticFixture(temporary);
    app = await electron.launch({
      executablePath: executable,
      args: process.platform === 'linux' ? ['--no-sandbox'] : [],
      env: smokeEnvironment(temporary, artifacts, collector.dsn),
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
    await page.evaluate(function failRenderer() {
      setTimeout(function controlledRendererFailure() {
        throw new Error('PRIVATE_RENDERER_FIXTURE');
      }, 0);
    });
    const rendererError = await waitForCapture(
      collector,
      function controlledRendererError(item) {
        return item.type === 'event' && item.event.tags?.process === 'renderer';
      },
    );
    assert.notEqual(mainError.tags.support_ref, rendererError.tags.support_ref);
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
    await fs.writeFile(
      path.join(artifacts, 'received-events.json'),
      JSON.stringify(
        { events: collector.events, failures: collector.failures },
        null,
        2,
      ),
    );
    await app?.close();
    await collector.close();
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

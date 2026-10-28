import assert from 'node:assert/strict';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import test from 'node:test';
import { runDeployedSmoke } from './deployed-smoke';

const DEPLOYED_COMMIT = '0123456789abcdef0123456789abcdef01234567';
const OTHER_COMMIT = 'fedcba9876543210fedcba9876543210fedcba98';

function compatibilityManifest(headers: Headers) {
  const desktopVersion = headers.get('x-nevermind-client-version');
  const apiVersion = Number(headers.get('x-nevermind-api-version') || 1);
  const desktopSupported = !desktopVersion || desktopVersion !== '1.1.999';
  const apiSupported = apiVersion === 1;
  const compatible = desktopSupported && apiSupported;
  return {
    backend: { environment: 'test', version: 'abcdef0' },
    api: { currentVersion: 1, supportedVersions: [1] },
    desktop: {
      minimumSupportedVersion: '1.2.0',
      latestVersion: '1.3.0',
      updateUrl: 'https://example.com/update',
    },
    client: {
      compatible,
      unsupportedReason: compatible
        ? null
        : desktopSupported
          ? 'unsupported_api_version'
          : 'unsupported_desktop_version',
    },
    features: { proxy_streaming: true },
  };
}

function smokeContractHandler(health: Record<string, unknown>) {
  return (request: IncomingMessage, response: ServerResponse) => {
    response.setHeader('content-type', 'application/json');
    if (request.url === '/api/health') {
      response.end(JSON.stringify(health));
      return;
    }
    if (request.url === '/api/compatibility') {
      response.end(
        JSON.stringify(
          compatibilityManifest(new Headers(request.headers as HeadersInit)),
        ),
      );
      return;
    }
    response.statusCode = 404;
    response.end('{}');
  };
}

async function fixtureServer(
  handler: (request: IncomingMessage, response: ServerResponse) => void,
  action: (baseUrl: string) => Promise<void>,
) {
  const server = createServer(handler);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const address = server.address();
    assert.ok(address && typeof address === 'object');
    await action(`http://127.0.0.1:${address.port}`);
  } finally {
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
}

test('smokes health plus compatible and incompatible desktop contracts', async () => {
  await fixtureServer(
    smokeContractHandler({ ok: true, db: true, upstream: true, version: 'abcdef0', commit: DEPLOYED_COMMIT }),
    async (baseUrl) => {
      assert.deepEqual(await runDeployedSmoke(baseUrl), {
        target: baseUrl,
        health: { version: 'abcdef0', commit: DEPLOYED_COMMIT, db: true, upstream: true },
        compatibility: {
          backendEnvironment: 'test',
          backendVersion: 'abcdef0',
          minimumDesktopVersion: '1.2.0',
          supportedDesktopVersion: '1.2.0',
          unsupportedDesktopVersion: '1.1.999',
          unsupportedReason: 'unsupported_desktop_version',
        },
      });
    },
  );
});

test('reports the target and redacts secrets in failure bodies', async () => {
  await fixtureServer(
    (_request, response) => {
      response.statusCode = 503;
      response.setHeader('content-type', 'application/json');
      response.end(
        JSON.stringify({ error: 'unhealthy', token: 'super-secret-value' }),
      );
    },
    async (baseUrl) => {
      await assert.rejects(
        () => runDeployedSmoke(baseUrl),
        (error: unknown) => {
          assert.ok(error instanceof Error);
          assert.match(error.message, new RegExp(baseUrl));
          assert.match(error.message, /\[redacted\]/);
          assert.doesNotMatch(error.message, /super-secret-value/);
          return true;
        },
      );
    },
  );
});

test('requires an explicit safe target', async () => {
  await assert.rejects(() => runDeployedSmoke(undefined), /is required/);
  await assert.rejects(
    () => runDeployedSmoke('https://user:secret@example.com'),
    /must not include credentials/,
  );
  await assert.rejects(
    () => runDeployedSmoke('http://example.com'),
    /must use HTTPS/,
  );
  await assert.rejects(
    () => runDeployedSmoke('https://api.nvm.fyi/api'),
    /must not include a path/,
  );
});

test('fails instead of following a redirect', async () => {
  await fixtureServer(
    (_request, response) => {
      response.statusCode = 308;
      response.setHeader('location', 'https://api.nvm.fyi/api/health');
      response.end();
    },
    async (baseUrl) => {
      await assert.rejects(() => runDeployedSmoke(baseUrl), /unexpected redirect/);
    },
  );
});

test('accepts health whose full commit matches the expected deployment', async () => {
  await fixtureServer(
    smokeContractHandler({ ok: true, db: true, upstream: true, version: '0123456', commit: DEPLOYED_COMMIT }),
    async (baseUrl) => {
      const result = await runDeployedSmoke(baseUrl, fetch, { expectedCommit: DEPLOYED_COMMIT });
      assert.equal(result.health.commit, DEPLOYED_COMMIT);
    },
  );
});

test('fails when health reports a different commit than the expected deployment', async () => {
  await fixtureServer(
    smokeContractHandler({ ok: true, db: true, upstream: true, version: 'fedcba9', commit: OTHER_COMMIT }),
    async (baseUrl) => {
      await assert.rejects(
        () => runDeployedSmoke(baseUrl, fetch, { expectedCommit: DEPLOYED_COMMIT }),
        new RegExp(`reported commit ${OTHER_COMMIT}, expected ${DEPLOYED_COMMIT}`),
      );
    },
  );
});

test('fails closed when health lacks a full commit identity', async () => {
  for (const commit of [null, 'abcdef0', 'dev']) {
    await fixtureServer(
      smokeContractHandler({ ok: true, db: true, upstream: true, version: 'dev', commit }),
      async (baseUrl) => {
        await assert.rejects(() => runDeployedSmoke(baseUrl, fetch, { expectedCommit: DEPLOYED_COMMIT }), /reported commit/);
      },
    );
  }
});

test('can smoke an older deployment without a commit when none is required', async () => {
  await fixtureServer(
    smokeContractHandler({ ok: true, db: true, upstream: true, version: 'abcdef0' }),
    async (baseUrl) => {
      const result = await runDeployedSmoke(baseUrl);
      assert.equal(result.health.commit, null);
    },
  );
});

test('rejects malformed expected commits before requesting health', async () => {
  let requested = false;
  const failingFetch = (async () => {
    requested = true;
    throw new Error('unexpected request');
  }) as typeof fetch;

  await assert.rejects(
    () => runDeployedSmoke('https://api.nvm.fyi', failingFetch, { expectedCommit: 'abcdef0' }),
    /full lowercase 40-character commit SHA/,
  );
  await assert.rejects(
    () => runDeployedSmoke('https://api.nvm.fyi', failingFetch, { expectedCommit: DEPLOYED_COMMIT.toUpperCase() }),
    /full lowercase 40-character commit SHA/,
  );
  assert.equal(requested, false);
});

import assert from 'node:assert/strict';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import test from 'node:test';
import { runDeployedSmoke } from './deployed-smoke';

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
    (request, response) => {
      response.setHeader('content-type', 'application/json');
      if (request.url === '/api/health') {
        response.end(
          JSON.stringify({ ok: true, db: true, upstream: true, version: 'abcdef0' }),
        );
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
    },
    async (baseUrl) => {
      assert.deepEqual(await runDeployedSmoke(baseUrl), {
        target: baseUrl,
        health: { version: 'abcdef0', db: true, upstream: true },
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
});

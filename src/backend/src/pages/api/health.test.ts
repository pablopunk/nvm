import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import type { Database } from '../../db/client';

const UPSTREAM_BASE_URL = 'https://upstream.test/zen/v1';
const CATALOG_BODY = JSON.stringify({ object: 'list', data: [{ id: 'model-a' }] });

process.env.DATABASE_URL ??= 'postgres://health-test.invalid/nvm';
process.env.OPENCODE_BASE_URL = UPSTREAM_BASE_URL;
process.env.OPENCODE_API_KEY = 'test-opencode-key';

type UpstreamReply = { status: number; contentType?: string; body?: string } | 'timeout';

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function stubUpstream(reply: UpstreamReply): string[] {
  const requested: string[] = [];
  globalThis.fetch = (async function upstreamStub(input: RequestInfo | URL, init?: RequestInit) {
    requested.push(String(input));
    if (reply === 'timeout')
      return new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () =>
          reject(new DOMException('The operation was aborted', 'AbortError')),
        );
      });
    return new Response(reply.body ?? '', {
      status: reply.status,
      headers: { 'content-type': reply.contentType ?? 'application/json' },
    });
  }) as typeof fetch;
  return requested;
}

function emptySettingsDatabase(): Database {
  const settingsQuery = {
    from: () => settingsQuery,
    where: () => settingsQuery,
    limit: async () => [],
  };
  return {
    execute: async () => [],
    select: () => settingsQuery,
  } as unknown as Database;
}

async function healthFor(reply: UpstreamReply) {
  const [{ GET }, { withTestDb }] = await Promise.all([
    import('./health'),
    import('../../db/client'),
  ]);
  const requested = stubUpstream(reply);
  const response = await withTestDb(emptySettingsDatabase(), () =>
    GET({} as Parameters<typeof GET>[0]),
  );
  return {
    status: response.status,
    body: (await response.json()) as Record<string, unknown>,
    requested,
  };
}

function readinessOf(body: Record<string, unknown>) {
  return {
    ok: body.ok,
    db: body.db,
    upstream: body.upstream,
    upstreamReady: body.upstreamReady,
  };
}

test('health reports upstream ready only for a 2xx model catalog from the active provider', async () => {
  const response = await healthFor({ status: 200, body: CATALOG_BODY });
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), {
    ok: true,
    db: true,
    upstream: true,
    upstreamReady: true,
  });
  assert.deepEqual(response.requested, [`${UPSTREAM_BASE_URL}/models`]);
});

for (const status of [401, 403, 404, 429]) {
  test(`health keeps released liveness for upstream ${status} but never reports it ready`, async () => {
    const response = await healthFor({
      status,
      body: JSON.stringify({ error: 'rejected' }),
    });
    assert.equal(response.status, 200);
    assert.deepEqual(readinessOf(response.body), {
      ok: true,
      db: true,
      upstream: true,
      upstreamReady: false,
    });
  });
}

test('health fails closed when the upstream returns a 5xx', async () => {
  const response = await healthFor({ status: 503, body: '{}' });
  assert.equal(response.status, 503);
  assert.deepEqual(readinessOf(response.body), {
    ok: false,
    db: true,
    upstream: false,
    upstreamReady: false,
  });
});

test('health does not treat a 200 without a model catalog as ready', async () => {
  const html = await healthFor({
    status: 200,
    contentType: 'text/html',
    body: '<!doctype html><html></html>',
  });
  assert.equal(html.body.upstreamReady, false);
  assert.equal(html.body.upstream, true);

  const wrongShape = await healthFor({ status: 200, body: '{"models":[]}' });
  assert.equal(wrongShape.body.upstreamReady, false);
  assert.equal(wrongShape.body.upstream, true);
});

test('health reports an unresponsive upstream as unreachable and not ready', async () => {
  const response = await healthFor('timeout');
  assert.equal(response.status, 503);
  assert.deepEqual(readinessOf(response.body), {
    ok: false,
    db: true,
    upstream: false,
    upstreamReady: false,
  });
});

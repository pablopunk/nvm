import assert from 'node:assert/strict';
import test from 'node:test';
import type { APIRoute } from 'astro';

process.env.NVM_MAGIC_AUTH_PROBE_SECRET = 'staging-probe-secret';
const { GET } = await import('./magic-auth-e2e');

function context(authorization?: string) {
  const request = new Request('https://auth-staging.example.test/api/health/magic-auth-e2e', {
    headers: authorization ? { authorization } : undefined,
  });
  return { request, url: new URL(request.url) } as Parameters<APIRoute>[0];
}

test('Magic Auth deployment probe hides itself without the exact staging secret', async () => {
  assert.equal((await GET(context())).status, 404);
  assert.equal((await GET(context('Bearer wrong-secret'))).status, 404);
});

test('Magic Auth deployment probe fails closed before datastore access when staging config is incomplete', async () => {
  const response = await GET(context('Bearer staging-probe-secret'));
  assert.equal(response.status, 503);
  assert.deepEqual(await response.json(), { ok: false, reason: 'deployment_configuration' });
});

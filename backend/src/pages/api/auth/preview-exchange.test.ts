import assert from 'node:assert/strict';
import test from 'node:test';

process.env.VERCEL_ENV = 'preview';
process.env.PREVIEW_GATEWAY_ORIGIN = 'https://nvm.fyi';
process.env.PREVIEW_START_KEY = 'preview-start-test-key';
process.env.GATEWAY_STATE_KEY = 'gateway-state-test-key';
process.env.GATEWAY_STATE_REDIS_URL = 'https://gateway-state';
process.env.GATEWAY_STATE_REDIS_TOKEN = 'gateway-state-acl';
process.env.UPSTASH_REDIS_REST_URL = 'https://production-redis';
process.env.UPSTASH_REDIS_REST_TOKEN = 'production-acl';
process.env.PREVIEW_SESSION_KEY = 'preview-session-test-key';
process.env.DATABASE_URL = 'postgres://production-preview-test';
process.env.WORKOS_API_KEY = 'sk_test_preview_exchange';
process.env.WORKOS_CLIENT_ID = 'client_preview_exchange';
process.env.WORKOS_COOKIE_PASSWORD = 'workos-cookie-password';

const { GET, previewExchangeSecurityHeaders } = await import('./preview-exchange');
const { redactAuthUrl } = await import('../../../lib/log');

function context(request: Request) {
  return { request, url: new URL(request.url) } as any;
}

test('preview exchange security headers prevent referrer, caching, indexing, and body rendering', async () => {
  const headers = previewExchangeSecurityHeaders();
  assert.equal(headers.get('Referrer-Policy'), 'no-referrer');
  assert.equal(headers.get('Cache-Control'), 'no-store, private');
  assert.equal(headers.get('X-Robots-Tag'), 'noindex');

  const response = await GET(context(new Request('https://nvm-git-branch-team-pablo-varelas-projects-4f86af8b.vercel.app/api/auth/preview-exchange?grant=v2.invalid')));
  assert.equal(response.status, 400);
  assert.equal(await response.text(), 'Invalid or expired preview session grant');
  assert.equal(response.headers.get('Referrer-Policy'), 'no-referrer');
  assert.equal(response.headers.get('Cache-Control'), 'no-store, private');
  assert.equal(response.headers.get('X-Robots-Tag'), 'noindex');
});

test('preview exchange rejects grant-bearing legacy query material with redaction-safe headers', async () => {
  assert.equal(redactAuthUrl('https://nvm.fyi/api/auth/preview-exchange?grant=secret&code=secret&safe=1'), '/api/auth/preview-exchange?safe=1');
  const response = await GET(context(new Request('https://nvm-git-branch-team-pablo-varelas-projects-4f86af8b.vercel.app/api/auth/preview-exchange?grant=v2.invalid&sealedSession=secret')));
  assert.equal(response.status, 400);
  assert.match(await response.text(), /Invalid preview session grant/);
  assert.equal(response.headers.get('Referrer-Policy'), 'no-referrer');
});

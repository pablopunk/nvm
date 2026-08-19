import assert from 'node:assert/strict';
import test from 'node:test';
process.env.WORKOS_API_KEY = 'sk_test_preview';
process.env.WORKOS_CLIENT_ID = 'client_test_preview';
process.env.WORKOS_COOKIE_PASSWORD = 'production-session-key';
process.env.PREVIEW_SESSION_KEY = 'preview-session-key';
process.env.VERCEL_ENV = 'preview';
const { createPreviewSessionToken, getSessionFromCookies, PREVIEW_SESSION_COOKIE, SESSION_COOKIE } = await import('./workos');

test('Preview sessions use a separate cookie and reject production cookies', async () => {
  const token = await createPreviewSessionToken({ id: 'preview-user', email: 'preview@example.test' });
  assert.ok(token);
  const session = await getSessionFromCookies(`${PREVIEW_SESSION_COOKIE}=${token}`);
  assert.deepEqual(session?.user, { id: 'preview-user', email: 'preview@example.test' });
  assert.equal(await getSessionFromCookies(`${SESSION_COOKIE}=production-ciphertext`), null);
});

test('production runtime rejects Preview session material', async () => {
  const token = await createPreviewSessionToken({ id: 'preview-user', email: 'preview@example.test' });
  process.env.VERCEL_ENV = 'production';
  assert.equal(await getSessionFromCookies(`${PREVIEW_SESSION_COOKIE}=${token}`), null);
  process.env.VERCEL_ENV = 'preview';
});

import assert from 'node:assert/strict';
import test from 'node:test';

const keys = [
  'VERCEL_ENV',
  'WORKOS_API_KEY',
  'WORKOS_CLIENT_ID',
  'WORKOS_REDIRECT_URI',
  'WORKOS_COOKIE_PASSWORD',
  'GATEWAY_STATE_KEY',
];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));

Object.assign(process.env, {
  VERCEL_ENV: 'development',
  WORKOS_API_KEY: 'sk_test_signin',
  WORKOS_CLIENT_ID: 'client_test_signin',
  WORKOS_REDIRECT_URI: 'https://nvm.fyi/api/auth/callback',
  WORKOS_COOKIE_PASSWORD: 'signin-cookie-password-with-32-characters',
  GATEWAY_STATE_KEY: 'signin-state-key',
});

const { GET } = await import('./signin');
const { setPreviewAuthStoreForTests } = await import('../../../lib/preview-auth');

function context(url: string) {
  const parsed = new URL(url);
  return {
    request: new Request(parsed),
    url: parsed,
    redirect(location: string, status = 302) {
      return new Response(null, { status, headers: { Location: location } });
    },
  } as any;
}

test('production sign-in hands a one-use state and canonical callback to WorkOS', async function createsWorkosHandoff() {
  setPreviewAuthStoreForTests(new Map());
  const response = await GET(
    context('https://nvm.fyi/api/auth/signin?return_to=/profile'),
  );

  assert.equal(response.status, 302);
  const location = new URL(response.headers.get('Location') ?? '');
  assert.equal(location.origin, 'https://api.workos.com');
  assert.equal(location.searchParams.get('client_id'), 'client_test_signin');
  assert.equal(
    location.searchParams.get('redirect_uri'),
    'https://nvm.fyi/api/auth/callback',
  );
  assert.ok(location.searchParams.get('state'));
});

test('production sign-in fails closed before creating state when WorkOS config is missing', async function rejectsMissingWorkosConfig() {
  const stateStore = new Map<string, unknown>();
  setPreviewAuthStoreForTests(stateStore);
  delete process.env.WORKOS_API_KEY;

  const response = await GET(context('https://nvm.fyi/api/auth/signin'));

  assert.equal(response.status, 503);
  assert.equal(await response.text(), 'Authentication is temporarily unavailable');
  assert.equal(stateStore.size, 0);
});

test.after(function restoreEnvironment() {
  setPreviewAuthStoreForTests(null);
  for (const key of keys) {
    if (original[key] === undefined) delete process.env[key];
    else process.env[key] = original[key];
  }
});

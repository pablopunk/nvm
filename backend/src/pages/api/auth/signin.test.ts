import assert from 'node:assert/strict';
import test from 'node:test';

const keys = [
  'VERCEL_ENV',
  'VERCEL_URL',
  'PRODUCTION_ORIGIN',
  'PUBLIC_DASHBOARD_URL',
  'PREVIEW_GATEWAY_ORIGIN',
  'WORKOS_API_KEY',
  'WORKOS_CLIENT_ID',
  'WORKOS_REDIRECT_URI',
  'WORKOS_COOKIE_PASSWORD',
  'GATEWAY_STATE_KEY',
];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));

function validProduction() {
  Object.assign(process.env, {
    VERCEL_ENV: 'development',
    PRODUCTION_ORIGIN: 'https://www.nvm.fyi',
    PUBLIC_DASHBOARD_URL: 'https://www.nvm.fyi',
    PREVIEW_GATEWAY_ORIGIN: 'https://www.nvm.fyi',
    WORKOS_API_KEY: 'sk_test_signin',
    WORKOS_CLIENT_ID: 'client_test_signin',
    WORKOS_REDIRECT_URI: 'https://www.nvm.fyi/api/auth/callback',
    WORKOS_COOKIE_PASSWORD: 'signin-cookie-password-with-32-characters',
    GATEWAY_STATE_KEY: 'signin-state-key',
  });
}

validProduction();

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

test('production sign-in hands a one-use state and canonical callback to WorkOS', async () => {
  validProduction();
  setPreviewAuthStoreForTests(new Map());
  const response = await GET(
    context('https://www.nvm.fyi/api/auth/signin?return_to=/profile'),
  );

  assert.equal(response.status, 302);
  const location = new URL(response.headers.get('Location') ?? '');
  assert.equal(location.origin, 'https://api.workos.com');
  assert.equal(location.searchParams.get('client_id'), 'client_test_signin');
  assert.equal(
    location.searchParams.get('redirect_uri'),
    'https://www.nvm.fyi/api/auth/callback',
  );
  assert.ok(location.searchParams.get('state'));
});

test('production sign-in fails closed before creating state when WorkOS config is missing', async () => {
  validProduction();
  const stateStore = new Map<string, unknown>();
  setPreviewAuthStoreForTests(stateStore);
  delete process.env.WORKOS_API_KEY;

  const response = await GET(context('https://www.nvm.fyi/api/auth/signin'));

  assert.equal(response.status, 503);
  assert.equal(await response.text(), 'Authentication is temporarily unavailable');
  assert.equal(stateStore.size, 0);
});

test('Preview sign-in rejects a mismatched request origin before signing or redirecting', async () => {
  validProduction();
  process.env.VERCEL_ENV = 'preview';
  process.env.VERCEL_URL =
    'nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app';
  const response = await GET(context('https://attacker.invalid/api/auth/signin'));
  assert.equal(response.status, 503);
  assert.equal(response.headers.has('location'), false);
  assert.equal(response.headers.has('set-cookie'), false);
});

test.after(() => {
  setPreviewAuthStoreForTests(null);
  for (const key of keys) {
    if (original[key] === undefined) delete process.env[key];
    else process.env[key] = original[key];
  }
});

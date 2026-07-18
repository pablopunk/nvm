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
  'GATEWAY_STATE_REDIS_URL',
  'GATEWAY_STATE_REDIS_TOKEN',
  'UPSTASH_REDIS_REST_URL',
  'UPSTASH_REDIS_REST_TOKEN',
  'PREVIEW_START_KEY',
  'PREVIEW_SESSION_KEY',
  'DATABASE_URL',
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
  const [cookie] = response.headers.getSetCookie();
  assert.ok(cookie);
  assert.match(cookie, /nvm_auth_state=/);
  assert.match(cookie, /Path=\/api\/auth\/callback/);
  assert.match(cookie, /Secure/);
  assert.equal(
    decodeURIComponent(cookie.split(';', 1)[0]!.split('=', 2)[1]!),
    location.searchParams.get('state'),
  );
});

test('local HTTP sign-in uses the callback host without a Secure attribute', async () => {
  validProduction();
  delete process.env.VERCEL_ENV;
  process.env.WORKOS_REDIRECT_URI = 'http://localhost:4321/api/auth/callback';
  setPreviewAuthStoreForTests(new Map());
  const response = await GET(
    context('http://localhost:4321/api/auth/signin?return_to=/profile'),
  );

  assert.equal(response.status, 302);
  const [cookie] = response.headers.getSetCookie();
  assert.ok(cookie);
  assert.doesNotMatch(cookie, /Secure/);
  assert.doesNotMatch(cookie, /Domain=/i);
});

test('production sign-in fails closed and logs the safe configuration stage', async (t) => {
  validProduction();
  const stateStore = new Map<string, unknown>();
  setPreviewAuthStoreForTests(stateStore);
  delete process.env.WORKOS_API_KEY;
  const errors: string[] = [];
  t.mock.method(console, 'error', (line: unknown) => errors.push(String(line)));

  const response = await GET(context('https://www.nvm.fyi/api/auth/signin'));

  assert.equal(response.status, 503);
  assert.equal(await response.text(), 'Authentication is temporarily unavailable');
  assert.equal(stateStore.size, 0);
  assert.equal(errors.length, 1);
  const entry = JSON.parse(errors[0]!);
  assert.equal(entry.msg, 'auth_signin_failed');
  assert.equal(entry.stage, 'configuration');
  assert.equal(entry.error_name, 'AuthConfigurationError');
  assert.equal(
    entry.reason,
    'Production auth configuration is incomplete: WORKOS_API_KEY',
  );
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

test('Preview sign-in redirects to the gateway without setting a Preview-host cookie', async () => {
  validProduction();
  process.env.VERCEL_ENV = 'preview';
  process.env.VERCEL_URL =
    'nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app';
  process.env.PREVIEW_START_KEY = 'preview-start-test-key';
  process.env.GATEWAY_STATE_REDIS_URL = 'https://gateway-state';
  process.env.GATEWAY_STATE_REDIS_TOKEN = 'gateway-state-token';
  process.env.UPSTASH_REDIS_REST_URL = 'https://production-state';
  process.env.UPSTASH_REDIS_REST_TOKEN = 'production-state-token';
  process.env.PREVIEW_SESSION_KEY = 'preview-session-test-key';
  process.env.DATABASE_URL = 'postgres://preview-test';
  const response = await GET(
    context(
      'https://nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app/api/auth/signin',
    ),
  );

  assert.equal(response.status, 302);
  assert.match(
    response.headers.get('location') ?? '',
    /^https:\/\/www\.nvm\.fyi\/api\/auth\/preview-start\?intent=/,
  );
  assert.deepEqual(response.headers.getSetCookie(), []);
});

test.after(() => {
  setPreviewAuthStoreForTests(null);
  for (const key of keys) {
    if (original[key] === undefined) delete process.env[key];
    else process.env[key] = original[key];
  }
});

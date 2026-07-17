import assert from 'node:assert/strict';
import test from 'node:test';
import type { APIRoute } from 'astro';

const keys = [
  'DATABASE_URL',
  'GATEWAY_STATE_KEY',
  'GATEWAY_STATE_REDIS_TOKEN',
  'GATEWAY_STATE_REDIS_URL',
  'PREVIEW_GATEWAY_ORIGIN',
  'PREVIEW_SESSION_KEY',
  'PREVIEW_START_KEY',
  'PRODUCTION_ORIGIN',
  'PUBLIC_DASHBOARD_URL',
  'UPSTASH_REDIS_REST_TOKEN',
  'UPSTASH_REDIS_REST_URL',
  'VERCEL_ENV',
  'VERCEL_URL',
  'WORKOS_API_KEY',
  'WORKOS_CLIENT_ID',
  'WORKOS_COOKIE_PASSWORD',
  'WORKOS_REDIRECT_URI',
];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));

function configureProduction() {
  Object.assign(process.env, {
    DATABASE_URL: 'postgres://callback-test',
    GATEWAY_STATE_KEY: 'callback-state-test-key',
    GATEWAY_STATE_REDIS_TOKEN: 'gateway-state-token',
    GATEWAY_STATE_REDIS_URL: 'https://gateway-state',
    PREVIEW_GATEWAY_ORIGIN: 'https://www.nvm.fyi',
    PREVIEW_SESSION_KEY: 'preview-session-test-key',
    PREVIEW_START_KEY: 'preview-start-test-key',
    PRODUCTION_ORIGIN: 'https://www.nvm.fyi',
    PUBLIC_DASHBOARD_URL: 'https://www.nvm.fyi',
    UPSTASH_REDIS_REST_TOKEN: 'production-state-token',
    UPSTASH_REDIS_REST_URL: 'https://production-state',
    VERCEL_ENV: 'production',
    WORKOS_API_KEY: 'sk_test_callback',
    WORKOS_CLIENT_ID: 'client_test_callback',
    WORKOS_COOKIE_PASSWORD: 'callback-cookie-password-with-32-characters',
    WORKOS_REDIRECT_URI: 'https://www.nvm.fyi/api/auth/callback',
  });
}

configureProduction();

const [
  { createAuthCallbackRoute },
  {
    createProductionState,
    setPreviewAuthStoreForTests,
  },
  { authCorrelationCookie },
  { GET: signIn },
  { GET: previewStart },
  { resetRateLimitOverridesForTests, setRateLimitOverridesForTests },
  { DisposableEmailError },
  { SignupsPolicyError },
] = await Promise.all([
  import('./callback'),
  import('../../../lib/preview-auth'),
  import('../../../lib/auth-correlation'),
  import('./signin'),
  import('./preview-start'),
  import('../../../lib/ratelimit'),
  import('../../../lib/users'),
  import('../../../lib/settings'),
]);

function context(request: Request) {
  return {
    request,
    url: new URL(request.url),
    redirect(location: string, status = 302) {
      return new Response(null, { status, headers: { Location: location } });
    },
  } as any;
}

function cookiePair(setCookie: string) {
  return setCookie.split(';', 1)[0]!;
}

function hasCorrelationClear(response: Response) {
  return response.headers
    .getSetCookie()
    .some((cookie) =>
      /^nvm_auth_state=;/.test(cookie) && /Max-Age=0/.test(cookie),
    );
}

async function productionState() {
  const state = await createProductionState('/profile');
  assert.ok(state);
  return state;
}

function callbackRequest(state: string, cookie = authCorrelationCookie(state, true)) {
  return new Request(
    `https://www.nvm.fyi/api/auth/callback?code=workos_code&state=${encodeURIComponent(state)}`,
    { headers: { cookie: cookiePair(cookie) } },
  );
}

function authenticated(sealedSession = 'sealed-session') {
  return {
    user: { id: 'workos_user', email: 'user@example.test' },
    sealedSession,
  } as any;
}

function authenticatedWithoutSession() {
  return {
    user: { id: 'workos_user', email: 'user@example.test' },
  } as any;
}

function successfulProductionRoute(overrides: Record<string, unknown> = {}) {
  return createAuthCallbackRoute({
    authenticateWithCode: async () => authenticated(),
    findUser: async () => ({ id: 'user_1' }) as any,
    ...overrides,
  } as any);
}

async function call(route: APIRoute, request: Request) {
  return route(context(request));
}

test.beforeEach(() => {
  configureProduction();
  setPreviewAuthStoreForTests(new Map());
  resetRateLimitOverridesForTests();
});

test('clears correlation state on rate limiting', async () => {
  setRateLimitOverridesForTests({
    ip: async () => ({
      ok: false,
      scope: 'auth:test',
      retryAfterSec: 3,
    }),
  });
  const response = await call(
    successfulProductionRoute(),
    new Request('https://www.nvm.fyi/api/auth/callback?code=workos_code'),
  );
  assert.equal(response.status, 429);
  assert.equal(hasCorrelationClear(response), true);
});

test('clears correlation state on missing-code and missing-config failures', async () => {
  const route = successfulProductionRoute();
  const missingCode = await call(
    route,
    new Request('https://www.nvm.fyi/api/auth/callback'),
  );
  assert.equal(missingCode.status, 400);
  assert.equal(hasCorrelationClear(missingCode), true);

  delete process.env.WORKOS_API_KEY;
  const missingConfig = await call(
    route,
    new Request('https://www.nvm.fyi/api/auth/callback?code=workos_code'),
  );
  assert.equal(missingConfig.status, 503);
  assert.equal(hasCorrelationClear(missingConfig), true);
});

test('rejects missing, mismatched, malformed, and replayed state without burning a valid match', async () => {
  const route = successfulProductionRoute();
  const state = await productionState();

  const missing = await call(
    route,
    new Request(
      `https://www.nvm.fyi/api/auth/callback?code=workos_code&state=${encodeURIComponent(state)}`,
    ),
  );
  assert.equal(missing.status, 400);

  const mismatched = await call(
    route,
    callbackRequest(state, authCorrelationCookie('different', true)),
  );
  assert.equal(mismatched.status, 400);

  const correct = await call(route, callbackRequest(state));
  assert.equal(correct.status, 302, 'mismatch must not consume Redis state');
  assert.equal(correct.headers.get('location'), '/profile');

  const replay = await call(route, callbackRequest(state));
  assert.equal(replay.status, 400);

  const malformed = await call(
    route,
    callbackRequest('v2.malformed', authCorrelationCookie('v2.malformed', true)),
  );
  assert.equal(malformed.status, 400);
  for (const response of [missing, mismatched, correct, replay, malformed]) {
    assert.equal(hasCorrelationClear(response), true);
  }
});

test('clears state on provider, missing-session, and unexpected provisioning failures', async () => {
  const providerState = await productionState();
  const providerFailure = await call(
    createAuthCallbackRoute({
      authenticateWithCode: async () => {
        throw new Error('provider unavailable');
      },
    }),
    callbackRequest(providerState),
  );
  assert.equal(providerFailure.status, 502);

  const missingSessionState = await productionState();
  const missingSession = await call(
    createAuthCallbackRoute({
      authenticateWithCode: async () => authenticatedWithoutSession(),
    }),
    callbackRequest(missingSessionState),
  );
  assert.equal(missingSession.status, 502);

  const provisioningState = await productionState();
  const provisioningFailure = await call(
    successfulProductionRoute({
      findUser: async () => {
        throw new Error('database unavailable');
      },
    }),
    callbackRequest(provisioningState),
  );
  assert.equal(provisioningFailure.status, 500);
  assert.equal(
    await provisioningFailure.text(),
    'Authentication failed; please restart.',
  );
  for (const response of [providerFailure, missingSession, provisioningFailure]) {
    assert.equal(hasCorrelationClear(response), true);
  }
});

test('preserves invite and session cookies while appending correlation cleanup', async () => {
  const inviteState = await productionState();
  const inviteRequired = await call(
    successfulProductionRoute({
      findUser: async () => null,
      signupsEnabled: async () => false,
    }),
    callbackRequest(inviteState),
  );
  assert.equal(inviteRequired.status, 303);
  assert.equal(inviteRequired.headers.get('location'), '/?invite=required');
  assert.equal(inviteRequired.headers.getSetCookie().length, 2);
  assert.equal(hasCorrelationClear(inviteRequired), true);

  const successState = await productionState();
  const success = await call(
    successfulProductionRoute(),
    callbackRequest(successState),
  );
  assert.equal(success.status, 302);
  assert.equal(success.headers.get('location'), '/profile');
  assert.equal(success.headers.getSetCookie().length, 2);
  assert.match(success.headers.getSetCookie()[0]!, /^nvm_session=/);
  assert.equal(hasCorrelationClear(success), true);
});

test('clears state on policy and disposable-email rejections', async () => {
  const policyState = await productionState();
  const policyFailure = await call(
    successfulProductionRoute({
      findUser: async () => null,
      signupsEnabled: async () => {
        throw new SignupsPolicyError('policy unavailable');
      },
    }),
    callbackRequest(policyState),
  );
  assert.equal(policyFailure.status, 503);

  const disposableState = await productionState();
  const disposableFailure = await call(
    successfulProductionRoute({
      findUser: async () => null,
      signupsEnabled: async () => true,
      upsertUser: async () => {
        throw new DisposableEmailError('user@example.test');
      },
    }),
    callbackRequest(disposableState),
  );
  assert.equal(disposableFailure.status, 403);
  assert.equal(hasCorrelationClear(policyFailure), true);
  assert.equal(hasCorrelationClear(disposableFailure), true);
});

test('Preview browser journey sets correlation only on the gateway and clears it after callback', async () => {
  process.env.VERCEL_ENV = 'preview';
  process.env.VERCEL_URL =
    'nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app';
  const previewSignIn = await signIn(
    context(
      new Request(
        'https://nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app/api/auth/signin',
      ),
    ),
  );
  assert.equal(previewSignIn.status, 302);
  assert.deepEqual(previewSignIn.headers.getSetCookie(), []);

  process.env.VERCEL_ENV = 'production';
  const gatewayStart = await previewStart(
    context(new Request(previewSignIn.headers.get('location')!)),
  );
  assert.equal(gatewayStart.status, 302);
  const gatewayCookie = gatewayStart.headers.getSetCookie()[0];
  assert.ok(gatewayCookie);
  const providerLocation = new URL(gatewayStart.headers.get('location')!);
  const gatewayState = providerLocation.searchParams.get('state');
  assert.ok(gatewayState);
  assert.equal(
    decodeURIComponent(cookiePair(gatewayCookie).split('=', 2)[1]!),
    gatewayState,
  );

  const callback = await call(
    createAuthCallbackRoute({
      authenticateWithCode: async () => authenticatedWithoutSession(),
      createPreviewGrant: async () => 'v2.preview-grant',
    }),
    callbackRequest(gatewayState, gatewayCookie),
  );
  assert.equal(callback.status, 302);
  assert.equal(
    callback.headers.get('location'),
    'https://nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app/api/auth/preview-exchange?grant=v2.preview-grant',
  );
  assert.equal(hasCorrelationClear(callback), true);
});

test('Preview grant failures clear gateway correlation state', async () => {
  process.env.VERCEL_ENV = 'preview';
  process.env.VERCEL_URL =
    'nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app';
  const previewSignIn = await signIn(
    context(
      new Request(
        'https://nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app/api/auth/signin',
      ),
    ),
  );
  process.env.VERCEL_ENV = 'production';
  const gatewayStart = await previewStart(
    context(new Request(previewSignIn.headers.get('location')!)),
  );
  const gatewayState = new URL(
    gatewayStart.headers.get('location')!,
  ).searchParams.get('state')!;
  const response = await call(
    createAuthCallbackRoute({
      authenticateWithCode: async () => authenticatedWithoutSession(),
      createPreviewGrant: async () => null,
    }),
    callbackRequest(gatewayState, gatewayStart.headers.getSetCookie()[0]!),
  );
  assert.equal(response.status, 503);
  assert.equal(hasCorrelationClear(response), true);
});

test.after(() => {
  setPreviewAuthStoreForTests(null);
  resetRateLimitOverridesForTests();
  for (const key of keys) {
    if (original[key] === undefined) delete process.env[key];
    else process.env[key] = original[key];
  }
});

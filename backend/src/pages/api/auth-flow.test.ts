import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import type { APIRoute } from 'astro';

Object.assign(process.env, {
  VERCEL_ENV: 'production',
  WORKOS_API_KEY: 'sk_test_magic_auth',
  WORKOS_CLIENT_ID: 'client_test_magic_auth',
  WORKOS_COOKIE_PASSWORD: 'test-cookie-password-at-least-32-characters',
  WORKOS_REDIRECT_URI: 'https://auth-staging.example.test/api/auth/callback',
  GATEWAY_STATE_KEY: 'test-state-signing-key-at-least-32-characters',
});

const { createSignInHandler } = await import('./auth/signin');
const { createCallbackHandler } = await import('./auth/callback');
const { createMeHandler } = await import('./me');
const {
  createProductionState,
  consumeGatewayState,
  setPreviewAuthStoreForTests,
} = await import('../../lib/preview-auth');
const { getSessionFromCookies, SESSION_COOKIE } = await import('../../lib/workos');

const stateStore = new Map<string, unknown>();

function context(request: Request, redirect?: (location: string, status?: number) => Response) {
  return {
    request,
    url: new URL(request.url),
    redirect: redirect ?? ((location: string, status = 302) => new Response(null, { status, headers: { Location: location } })),
  } as Parameters<APIRoute>[0];
}

function quietLogger(entries: unknown[] = []) {
  const capture = (message: string, fields?: unknown) => entries.push({ message, fields });
  return { info: capture, warn: capture, error: capture };
}

function callbackHandler(overrides: Record<string, unknown> = {}) {
  return createCallbackHandler({
    rateLimit: async () => ({ ok: true }),
    authenticate: async () => ({ user: { id: 'user_workos_1', email: 'magic@example.test' }, sealedSession: 'sealed-session-value' }),
    getExistingUser: async () => null,
    getSignupsEnabled: async () => true,
    upsertUser: async () => ({ id: 'local-user-1' }),
    logger: quietLogger(),
    ...overrides,
  } as any);
}

async function invoke(handler: APIRoute, url: string, cookie?: string) {
  const request = new Request(url, cookie ? { headers: { cookie } } : undefined);
  return handler(context(request));
}

beforeEach(() => {
  stateStore.clear();
  setPreviewAuthStoreForTests(stateStore);
  process.env.VERCEL_ENV = 'production';
});

afterEach(() => setPreviewAuthStoreForTests(null));

test('signin creates a fresh one-use state and sends the expected AuthKit authorization inputs', async () => {
  const authorizationInputs: Record<string, string>[] = [];
  const handler = createSignInHandler({
    rateLimit: async () => ({ ok: true }),
    authorizationUrl: (input) => {
      authorizationInputs.push(input);
      return `https://authkit.example.test/authorize?state=${encodeURIComponent(input.state)}`;
    },
  });

  const first = await invoke(handler, 'https://auth-staging.example.test/api/auth/signin?return_to=%2Fdashboard');
  const second = await invoke(handler, 'https://auth-staging.example.test/api/auth/signin?return_to=https%3A%2F%2Fevil.example');

  assert.equal(first.status, 302);
  assert.equal(second.status, 302);
  assert.equal(authorizationInputs.length, 2);
  assert.deepEqual(
    authorizationInputs.map(({ provider, clientId, redirectUri }) => ({ provider, clientId, redirectUri })),
    [
      { provider: 'authkit', clientId: 'client_test_magic_auth', redirectUri: process.env.WORKOS_REDIRECT_URI },
      { provider: 'authkit', clientId: 'client_test_magic_auth', redirectUri: process.env.WORKOS_REDIRECT_URI },
    ],
  );
  assert.notEqual(authorizationInputs[0]!.state, authorizationInputs[1]!.state);
  const firstState = await consumeGatewayState(authorizationInputs[0]!.state);
  assert.equal(firstState?.flow, 'production');
  if (firstState?.flow !== 'production') assert.fail('expected production state');
  assert.equal(firstState.safeRelativeReturnPath, '/dashboard');
  assert.equal(await consumeGatewayState(authorizationInputs[0]!.state), null, 'state is single-use');
  const secondState = await consumeGatewayState(authorizationInputs[1]!.state);
  assert.equal(secondState?.flow, 'production');
  if (secondState?.flow !== 'production') assert.fail('expected production state');
  assert.equal(secondState.safeRelativeReturnPath, '/dashboard');
});

test('callback sets the protected session cookie and that exact cookie authenticates /api/me', async () => {
  const state = await createProductionState('/dashboard?tab=usage');
  assert.ok(state);
  const response = await invoke(callbackHandler(), `https://auth-staging.example.test/api/auth/callback?code=authorization-code&state=${encodeURIComponent(state)}`);

  assert.equal(response.status, 302);
  assert.equal(response.headers.get('location'), '/dashboard?tab=usage');
  const setCookie = response.headers.get('set-cookie');
  assert.ok(setCookie);
  assert.ok(setCookie.includes(`${SESSION_COOKIE}=sealed-session-value`));
  assert.match(setCookie, /HttpOnly/i);
  assert.match(setCookie, /Secure/i);
  assert.match(setCookie, /SameSite=Lax/i);
  const cookie = setCookie.split(';', 1)[0]!;

  const sessionLoader = {
    loadSealedSession(input: { sessionData: string; cookiePassword: string }) {
      assert.equal(input.sessionData, 'sealed-session-value');
      assert.equal(input.cookiePassword, process.env.WORKOS_COOKIE_PASSWORD);
      return { authenticate: async () => ({ authenticated: true, user: { id: 'user_workos_1', email: 'magic@example.test' } }) };
    },
  };
  const parsed = await getSessionFromCookies(cookie, sessionLoader as any);
  assert.equal(parsed?.user.id, 'user_workos_1');

  const me = createMeHandler({
    getSession: (header) => getSessionFromCookies(header, sessionLoader as any),
    getUser: async (workosUserId) => ({ id: 'local-user-1', workosUserId, email: 'magic@example.test', plan: 'free' }),
    ensureCredits: async () => {},
    getBalance: async () => 500,
    getRecentUsage: async () => [],
  });
  const meResponse = await invoke(me, 'https://auth-staging.example.test/api/me', cookie);
  assert.equal(meResponse.status, 200);
  assert.deepEqual(await meResponse.json(), { email: 'magic@example.test', plan: 'free', balance: 500, recentUsage: [] });
});

test('missing, malformed, expired, and replayed state fail closed before provisioning', async (t) => {
  let exchanges = 0;
  let provisions = 0;
  const handler = callbackHandler({
    authenticate: async () => {
      exchanges += 1;
      return { user: { id: 'user_workos_1', email: 'magic@example.test' }, sealedSession: 'sealed-session-value' };
    },
    upsertUser: async () => {
      provisions += 1;
      return { id: 'local-user-1' };
    },
  });

  assert.equal((await invoke(handler, 'https://auth-staging.example.test/api/auth/callback?state=bad')).status, 400);
  assert.equal((await invoke(handler, 'https://auth-staging.example.test/api/auth/callback?code=code&state=v2.malformed')).status, 400);

  t.mock.timers.enable({ apis: ['Date'], now: new Date('2026-07-17T12:00:00Z') });
  const expired = await createProductionState('/');
  assert.ok(expired);
  t.mock.timers.tick(11 * 60 * 1000);
  assert.equal((await invoke(handler, `https://auth-staging.example.test/api/auth/callback?code=code&state=${encodeURIComponent(expired)}`)).status, 400);
  t.mock.timers.reset();

  const replayed = await createProductionState('/');
  assert.ok(replayed);
  assert.equal((await invoke(handler, `https://auth-staging.example.test/api/auth/callback?code=code&state=${encodeURIComponent(replayed)}`)).status, 302);
  assert.equal((await invoke(handler, `https://auth-staging.example.test/api/auth/callback?code=code&state=${encodeURIComponent(replayed)}`)).status, 400);
  assert.equal(exchanges, 1);
  assert.equal(provisions, 1);
});

test('provider failures and missing sealed sessions keep secrets out of responses and logs', async () => {
  const entries: unknown[] = [];
  const logger = quietLogger(entries);
  const providerFailureState = await createProductionState('/');
  assert.ok(providerFailureState);
  const providerFailure = await invoke(
    callbackHandler({ authenticate: async () => { throw new Error('authorization-code 654321 inbox-secret'); }, logger }),
    `https://auth-staging.example.test/api/auth/callback?code=authorization-code&state=${encodeURIComponent(providerFailureState)}`,
  );
  assert.equal(providerFailure.status, 502);

  const missingSessionState = await createProductionState('/');
  assert.ok(missingSessionState);
  const missingSession = await invoke(
    callbackHandler({ authenticate: async () => ({ user: { id: 'user_workos_1', email: 'magic@example.test' } }), logger }),
    `https://auth-staging.example.test/api/auth/callback?code=second-code&state=${encodeURIComponent(missingSessionState)}`,
  );
  assert.equal(missingSession.status, 502);

  const visible = `${await providerFailure.text()} ${await missingSession.text()} ${JSON.stringify(entries)}`;
  for (const secret of ['authorization-code', 'second-code', '654321', 'inbox-secret', providerFailureState, missingSessionState]) {
    assert.equal(visible.includes(secret), false, `must redact ${secret}`);
  }
});

test('repeat sign-in and stale WorkOS ID relinking stay idempotent', async () => {
  const localUsers: { id: string; workosUserId: string; email: string }[] = [];
  let initialGrants = 0;
  let nextWorkosId = 'workos-old';
  const handler = callbackHandler({
    authenticate: async () => ({ user: { id: nextWorkosId, email: 'Same@Example.test' }, sealedSession: `sealed-${nextWorkosId}` }),
    getExistingUser: async (workosUserId: string) => localUsers.find((user) => user.workosUserId === workosUserId) ?? null,
    upsertUser: async ({ workosUserId, email }: { workosUserId: string; email: string }) => {
      const existing = localUsers.find((user) => user.email === email.toLowerCase());
      if (existing) {
        existing.workosUserId = workosUserId;
        return existing;
      }
      const created = { id: 'local-user-1', workosUserId, email: email.toLowerCase() };
      localUsers.push(created);
      initialGrants += 1;
      return created;
    },
  });

  for (const workosId of ['workos-old', 'workos-new']) {
    nextWorkosId = workosId;
    const state = await createProductionState('/');
    assert.ok(state);
    assert.equal((await invoke(handler, `https://auth-staging.example.test/api/auth/callback?code=code&state=${encodeURIComponent(state)}`)).status, 302);
  }
  assert.deepEqual(localUsers, [{ id: 'local-user-1', workosUserId: 'workos-new', email: 'same@example.test' }]);
  assert.equal(initialGrants, 1);
});

test('Magic Auth obeys the invite-required gate and accepts a valid invite intent', async () => {
  let openProvisioning = 0;
  const deniedState = await createProductionState('/');
  assert.ok(deniedState);
  const denied = await invoke(
    callbackHandler({ getSignupsEnabled: async () => false, upsertUser: async () => { openProvisioning += 1; return { id: 'no' }; } }),
    `https://auth-staging.example.test/api/auth/callback?code=code&state=${encodeURIComponent(deniedState)}`,
  );
  assert.equal(denied.status, 303);
  assert.equal(denied.headers.get('location'), '/?invite=required');
  assert.equal(openProvisioning, 0);

  let invitedProvisioning = 0;
  const invitedState = await createProductionState('/');
  assert.ok(invitedState);
  const invited = await invoke(
    callbackHandler({
      getSignupsEnabled: async () => false,
      readInviteIntent: () => ({ id: 'intent-1', nonce: 'nonce-1' }),
      createUserFromInvite: async () => { invitedProvisioning += 1; return { id: 'local-user-1' }; },
    }),
    `https://auth-staging.example.test/api/auth/callback?code=code&state=${encodeURIComponent(invitedState)}`,
  );
  assert.equal(invited.status, 302);
  assert.equal(invitedProvisioning, 1);
});

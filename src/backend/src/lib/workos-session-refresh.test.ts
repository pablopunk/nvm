import assert from 'node:assert/strict';
import test from 'node:test';
process.env.WORKOS_API_KEY = 'sk_test_refresh';
process.env.WORKOS_CLIENT_ID = 'client_test_refresh';
process.env.WORKOS_COOKIE_PASSWORD = 'production-session-key';
process.env.VERCEL_ENV = 'production';
const { getSessionFromCookies, SESSION_COOKIE, workosClient } = await import('./workos');
const { sendRefreshedSessionCookie } = await import('./session-refresh');

const user = { id: 'user_admin', email: 'admin@example.test' };

function fakeSealedSessions(validSealedSessions: Set<string>, refreshCalls: string[]) {
  workosClient().userManagement.loadSealedSession = function loadFakeSealedSession({ sessionData }) {
    return {
      async authenticate() {
        return validSealedSessions.has(sessionData)
          ? { authenticated: true, user }
          : { authenticated: false, reason: 'invalid_jwt' };
      },
      async refresh() {
        refreshCalls.push(sessionData);
        if (sessionData !== 'expired-sealed') return { authenticated: false, reason: 'invalid_grant' };
        validSealedSessions.add('fresh-sealed');
        return { authenticated: true, user, sealedSession: 'fresh-sealed' };
      },
    } as never;
  };
}

test('an expired session is refreshed once per request and the new cookie is sent back', async () => {
  const refreshCalls: string[] = [];
  fakeSealedSessions(new Set(), refreshCalls);
  const cookie = `${SESSION_COOKIE}=expired-sealed`;
  const response = await sendRefreshedSessionCookie(true, async function adminRequest() {
    assert.equal((await getSessionFromCookies(cookie))?.user.id, user.id);
    assert.equal((await getSessionFromCookies(cookie))?.user.id, user.id);
    return new Response('ok');
  });
  assert.deepEqual(refreshCalls, ['expired-sealed']);
  const setCookie = response.headers.get('set-cookie') ?? '';
  assert.match(setCookie, new RegExp(`^${SESSION_COOKIE}=fresh-sealed; Path=/; HttpOnly; Secure; SameSite=Lax`));
});

test('a valid session sends no new cookie', async () => {
  fakeSealedSessions(new Set(['valid-sealed']), []);
  const response = await sendRefreshedSessionCookie(true, async function adminRequest() {
    assert.equal((await getSessionFromCookies(`${SESSION_COOKIE}=valid-sealed`))?.user.id, user.id);
    return new Response('ok');
  });
  assert.equal(response.headers.get('set-cookie'), null);
});

test('a session WorkOS refuses to refresh stays signed out', async () => {
  fakeSealedSessions(new Set(), []);
  const response = await sendRefreshedSessionCookie(true, async function adminRequest() {
    assert.equal(await getSessionFromCookies(`${SESSION_COOKIE}=revoked-sealed`), null);
    return new Response('forbidden', { status: 403 });
  });
  assert.equal(response.headers.get('set-cookie'), null);
});

test('a refresh still sets the cookie when the response headers are immutable', async () => {
  fakeSealedSessions(new Set(), []);
  const response = await sendRefreshedSessionCookie(false, async function redirectRequest() {
    await getSessionFromCookies(`${SESSION_COOKIE}=expired-sealed`);
    return Response.redirect('https://api.nvm.fyi/admin', 302);
  });
  assert.equal(response.status, 302);
  assert.match(response.headers.get('set-cookie') ?? '', /^nvm_session=fresh-sealed; Path=\/; HttpOnly; SameSite=Lax/);
});

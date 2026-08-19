import assert from 'node:assert/strict';
import { afterEach, test, type TestContext } from 'node:test';

process.env.WORKOS_API_KEY = 'sk_test_approve';
process.env.WORKOS_CLIENT_ID = 'client_test_approve';
process.env.WORKOS_COOKIE_PASSWORD =
  'approve-cookie-password-with-32-characters';
process.env.VERCEL_ENV = 'production';

const [{ POST }, { setDbForTests, resetDbForTests }, { workos }] =
  await Promise.all([
    import('./approve'),
    import('../../../../db/client'),
    import('../../../../lib/workos'),
  ]);

function chain(result: unknown) {
  const promise = () => Promise.resolve(result);
  const value = {
    from: () => value,
    where: () => value,
    limit: () => promise(),
    set: () => value,
    returning: () => promise(),
    then: (
      resolve: Parameters<Promise<unknown>['then']>[0],
      reject: Parameters<Promise<unknown>['then']>[1],
    ) => promise().then(resolve, reject),
  };
  return value;
}

function fakeDb(input: { user?: unknown; updated?: unknown[] } = {}) {
  let selectCalls = 0;
  let updateCalls = 0;
  return {
    get selectCalls() {
      return selectCalls;
    },
    get updateCalls() {
      return updateCalls;
    },
    select() {
      selectCalls += 1;
      return chain(input.user ? [input.user] : []);
    },
    update() {
      updateCalls += 1;
      return chain(input.updated ?? [{ code: 'device_code' }]);
    },
  };
}

function request(headers: Record<string, string>, body = { code: 'device_code' }) {
  return new Request('https://www.nvm.fyi/api/auth/device/approve', {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
  });
}

function context(value: Request) {
  return { request: value, url: new URL(value.url) } as any;
}

function mockSession(t: TestContext) {
  let calls = 0;
  t.mock.method(workos.userManagement as any, 'loadSealedSession', () => {
    calls += 1;
    return {
      authenticate: async () => ({
        authenticated: true,
        user: { id: 'workos_user' },
      }),
    };
  });
  return () => calls;
}

afterEach(() => resetDbForTests());

test('rejects cross-origin and malformed origins before media type, session, or database work', async (t) => {
  const db = fakeDb();
  setDbForTests(db as any);
  const sessionCalls = mockSession(t);
  const crossOrigin = await POST(
    context(
      request({
        origin: 'https://attacker.invalid',
        'content-type': 'text/plain',
        cookie: 'nvm_session=sealed',
      }),
    ),
  );
  assert.equal(crossOrigin.status, 403);

  const malformedOrigin = await POST(
    context(
      request({
        origin: 'not a url',
        'content-type': 'application/json',
        cookie: 'nvm_session=sealed',
      }),
    ),
  );
  assert.equal(malformedOrigin.status, 403);
  assert.equal(sessionCalls(), 0);
  assert.equal(db.selectCalls, 0);
  assert.equal(db.updateCalls, 0);
});

test('rejects missing and non-JSON media types before session or database work', async (t) => {
  const db = fakeDb();
  setDbForTests(db as any);
  const sessionCalls = mockSession(t);
  const headersToTry: Array<Record<string, string>> = [
    { origin: 'https://www.nvm.fyi', cookie: 'nvm_session=sealed' },
    {
      origin: 'https://www.nvm.fyi',
      'content-type': 'text/plain',
      cookie: 'nvm_session=sealed',
    },
  ];
  for (const headers of headersToTry) {
    const response = await POST(context(request(headers)));
    assert.equal(response.status, 415);
  }
  assert.equal(sessionCalls(), 0);
  assert.equal(db.selectCalls, 0);
  assert.equal(db.updateCalls, 0);
});

test('accepts case-insensitive JSON with parameters and preserves approval contracts', async (t) => {
  const db = fakeDb({
    user: { id: 'user_1', workosUserId: 'workos_user' },
  });
  setDbForTests(db as any);
  mockSession(t);
  const response = await POST(
    context(
      request({
        origin: 'https://www.nvm.fyi',
        'content-type': 'Application/JSON; Charset=UTF-8',
        cookie: 'nvm_session=sealed',
      }),
    ),
  );
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { ok: true });
  assert.equal(db.selectCalls, 1);
  assert.equal(db.updateCalls, 1);
});

test('preserves unauthorized, missing-code, and invalid-code responses', async (t) => {
  const unauthorizedDb = fakeDb();
  setDbForTests(unauthorizedDb as any);
  const unauthorized = await POST(
    context(
      request({
        origin: 'https://www.nvm.fyi',
        'content-type': 'application/json',
      }),
    ),
  );
  assert.equal(unauthorized.status, 401);

  const missingDb = fakeDb({
    user: { id: 'user_1', workosUserId: 'workos_user' },
  });
  setDbForTests(missingDb as any);
  mockSession(t);
  const missing = await POST(
    context(
      request(
        {
          origin: 'https://www.nvm.fyi',
          'content-type': 'application/json',
          cookie: 'nvm_session=sealed',
        },
        { code: '   ' },
      ),
    ),
  );
  assert.equal(missing.status, 400);
  assert.equal(await missing.text(), 'Missing code');

  const invalidDb = fakeDb({
    user: { id: 'user_1', workosUserId: 'workos_user' },
    updated: [],
  });
  setDbForTests(invalidDb as any);
  const invalid = await POST(
    context(
      request({
        origin: 'https://www.nvm.fyi',
        'content-type': 'application/json',
        cookie: 'nvm_session=sealed',
      }),
    ),
  );
  assert.equal(invalid.status, 400);
  assert.equal(await invalid.text(), 'Invalid or expired code');
});
